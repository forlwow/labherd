// Package session 实现 Controller 端的 AgentService：接受 Agent 的会话并处理上行消息。
package session

import (
	"errors"
	"io"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// Server 实现 agentv1.AgentServiceServer。
//
// 当前为骨架阶段：没有节点表，也没有存储，收到的消息只打印日志。
type Server struct {
	agentv1.UnimplementedAgentServiceServer

	log *slog.Logger
}

// NewServer 创建 Server。
func NewServer(log *slog.Logger) *Server {
	return &Server{log: log}
}

// OpenSession 处理一条 Agent 会话，直到 Agent 断开或协议出错。
func (s *Server) OpenSession(stream agentv1.AgentService_OpenSessionServer) error {
	hello, err := recvHello(stream)
	if err != nil {
		return err
	}

	log := s.log.With("node_id", hello.GetNodeId(), "instance_id", hello.GetInstanceId())

	// 还没有存储，不知道之前确认到哪里，所以 last_acked_seq 恒为 0。
	// 按协议，Agent 会因此重发全部缓冲，重复数据由存储层去重。
	err = stream.Send(&agentv1.OpenSessionResponse{
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
		"gpu_count", len(hello.GetGpus()))

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			log.Info("agent closed session")
			return nil
		}
		if err != nil {
			log.Info("agent disconnected", "err", err)
			return err
		}

		switch p := req.GetPayload().(type) {
		case *agentv1.OpenSessionRequest_Heartbeat:
			log.Debug("heartbeat received", "pending_batches", p.Heartbeat.GetPendingBatches())
		case *agentv1.OpenSessionRequest_Metrics:
			if err := s.handleMetrics(stream, log, p.Metrics); err != nil {
				return err
			}
		case *agentv1.OpenSessionRequest_Processes:
			log.Info("processes received", "count", len(p.Processes.GetProcesses()))
		case *agentv1.OpenSessionRequest_Hello:
			return status.Error(codes.FailedPrecondition, "hello must be sent only once per session")
		default:
			// 新版 Agent 可能发送本版本不认识的消息类型，忽略而不断开，保持向前兼容。
			log.Warn("unknown payload ignored")
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

// handleMetrics 处理一批指标并回复确认。
//
// 按协议应在写入存储之后再确认。骨架阶段没有存储，打印日志即视为写入。
func (s *Server) handleMetrics(
	stream agentv1.AgentService_OpenSessionServer,
	log *slog.Logger,
	batch *agentv1.MetricsBatch,
) error {
	for _, sample := range batch.GetSamples() {
		log.Info("metrics received",
			"seq", batch.GetSeq(),
			"collected_at", sample.GetCollectedAt().AsTime(),
			"gpu_count", len(sample.GetGpus()))
	}

	return stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_MetricsAck{MetricsAck: &agentv1.MetricsAck{
			AckedSeq: batch.GetSeq(),
		}},
	})
}
