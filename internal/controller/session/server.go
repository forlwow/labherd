// Package session 实现 Controller 端的 AgentService：接受 Agent 的会话并处理上行消息。
package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/controller/registry"
)

// Server 实现 agentv1.AgentServiceServer。
//
// 会话与节点状态登记在注册表中；还没有存储，指标只记录最新一次采样。
type Server struct {
	agentv1.UnimplementedAgentServiceServer

	registry *registry.Registry
	log      *slog.Logger
}

// NewServer 创建 Server，会话登记到 reg。
func NewServer(reg *registry.Registry, log *slog.Logger) *Server {
	return &Server{registry: reg, log: log}
}

// OpenSession 处理一条 Agent 会话，直到 Agent 断开、会话被取代或协议出错。
func (s *Server) OpenSession(stream agentv1.AgentService_OpenSessionServer) error {
	hello, err := recvHello(stream)
	if err != nil {
		return err
	}
	// Agent 没有指定 IP 时，用连接的来源地址。注册表从 hello 中读取。
	if hello.GetIp() == "" {
		hello.Ip = peerIP(stream.Context())
	}
	log := s.log.With("node_id", hello.GetNodeId(), "instance_id", hello.GetInstanceId())

	// 先登记再回复 HelloAck：同一节点的旧会话此时被取代。
	sess, ctx := s.registry.Connect(stream.Context(), hello)
	err = s.serve(ctx, stream, sess, log, hello)
	sess.Close(err)
	return err
}

// serve 回复 HelloAck 并处理后续消息。返回 nil 表示 Agent 正常结束会话。
func (s *Server) serve(
	ctx context.Context,
	stream agentv1.AgentService_OpenSessionServer,
	sess *registry.Session,
	log *slog.Logger,
	hello *agentv1.Hello,
) error {
	// 还没有存储，不知道之前确认到哪里，所以 last_acked_seq 恒为 0。
	// 按协议，Agent 会因此重发全部缓冲，重复数据由存储层去重。
	err := stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_HelloAck{HelloAck: &agentv1.HelloAck{
			LastAckedSeq: 0,
			ServerTime:   timestamppb.Now(),
		}},
	})
	if err != nil {
		return err
	}
	log.Info("agent connected",
		"hostname", hello.GetHostname(),
		"agent_version", hello.GetAgentVersion(),
		"gpu_count", len(hello.GetGpus()),
		"ip", hello.GetIp(),
	)

	// stream.Recv 不受 ctx 控制，放到单独的协程中，主循环才能在会话被取代时及时退出。
	// 主循环返回后 gRPC 会取消 stream.Context()，接收协程随之结束。
	reqs := make(chan recvResult)
	go recvLoop(stream, reqs)

	for {
		var r recvResult
		select {
		case <-ctx.Done():
			if errors.Is(context.Cause(ctx), registry.ErrSuperseded) {
				log.Info("session superseded by a newer one")
				return status.Error(codes.Aborted, "session superseded by a newer one")
			}
			// 连接被取消：Agent 进程被杀、网络中断，或 Controller 正在退出。
			log.Info("agent disconnected", "err", ctx.Err())
			return ctx.Err()
		case r = <-reqs:
		}

		if errors.Is(r.err, io.EOF) {
			log.Info("agent closed session")
			return nil
		}
		if r.err != nil {
			log.Info("agent disconnected", "err", r.err)
			return r.err
		}

		sess.Touch()
		switch p := r.req.GetPayload().(type) {
		case *agentv1.OpenSessionRequest_Heartbeat:
			log.Debug("heartbeat received", "pending_batches", p.Heartbeat.GetPendingBatches())
		case *agentv1.OpenSessionRequest_Metrics:
			if err := handleMetrics(stream, sess, log, p.Metrics); err != nil {
				return err
			}
		case *agentv1.OpenSessionRequest_Processes:
			log.Debug("processes received", "count", len(p.Processes.GetProcesses()))
		case *agentv1.OpenSessionRequest_Hello:
			return status.Error(codes.FailedPrecondition, "hello must be sent only once per session")
		default:
			// 新版 Agent 可能发送本版本不认识的消息类型，忽略而不断开，保持向前兼容。
			log.Warn("unknown payload ignored")
		}
	}
}

// recvResult 是一次 stream.Recv 的结果。
type recvResult struct {
	req *agentv1.OpenSessionRequest
	err error
}

// recvLoop 持续接收上行消息并交给 out，直到出错或会话结束。
func recvLoop(stream agentv1.AgentService_OpenSessionServer, out chan<- recvResult) {
	for {
		req, err := stream.Recv()
		select {
		case out <- recvResult{req: req, err: err}:
		case <-stream.Context().Done():
			return
		}
		if err != nil {
			return
		}
	}
}

// recvHello 读取会话的第一条消息，并确认它是有效的 Hello。
func recvHello(stream agentv1.AgentService_OpenSessionServer) (*agentv1.Hello, error) {
	first, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	hello := first.GetHello()
	if hello == nil {
		return nil, status.Error(codes.FailedPrecondition, "first message must be hello")
	}
	if hello.GetNodeId() == "" || hello.GetInstanceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "hello must carry node_id and instance_id")
	}
	return hello, nil
}

// handleMetrics 记录一批指标并回复确认。
//
// 按协议应在写入存储之后再确认。还没有存储，记入注册表即视为写入。
func handleMetrics(
	stream agentv1.AgentService_OpenSessionServer,
	sess *registry.Session,
	log *slog.Logger,
	batch *agentv1.MetricsBatch,
) error {
	for _, sample := range batch.GetSamples() {
		sess.RecordSample(sample)
	}
	log.Debug("metrics received", "seq", batch.GetSeq(), "sample_count", len(batch.GetSamples()))

	return stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_MetricsAck{MetricsAck: &agentv1.MetricsAck{
			AckedSeq: batch.GetSeq(),
		}},
	})
}

// peerIP 返回 gRPC 连接的来源 IP；取不到时（如测试中的内存连接）返回空字符串。
func peerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	if a, ok := p.Addr.(*net.TCPAddr); ok {
		return a.IP.String()
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return ""
	}
	return host
}
