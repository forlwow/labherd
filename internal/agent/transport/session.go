// Package transport 负责 Agent 与 Controller 之间的会话：建立连接、握手、收发消息。
package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// ErrSessionClosed 表示 Controller 主动正常结束了会话。
var ErrSessionClosed = errors.New("session closed by controller")

// maxClockSkew 是本地时钟与 Controller 时钟允许的最大偏差，超过时记录警告。
const maxClockSkew = 5 * time.Second

// Config 是建立会话所需的参数。
type Config struct {
	// ControllerAddr 是 Controller 的 gRPC 地址，如 "10.0.0.1:9090"。
	ControllerAddr string
	// NodeID 是节点的稳定标识。
	NodeID string
	// InstanceID 在 Agent 进程每次启动时随机生成。
	InstanceID string
	// Hostname 仅用于显示。
	Hostname string
	// AgentVersion 是 Agent 的版本号。
	AgentVersion string
	// Interval 是发送指标的周期。
	Interval time.Duration
}

// Run 建立一条会话并周期性发送指标，直到 ctx 被取消或会话出错。
//
// ctx 被取消时返回 nil；其他情况返回导致会话结束的错误。
// 断线重连不在这里处理，由调用方决定是否重新调用 Run。
func Run(ctx context.Context, cfg Config, log *slog.Logger) error {
	conn, err := grpc.NewClient(cfg.ControllerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create grpc client: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 函数返回时取消 ctx，使流和接收协程一起结束。
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := agentv1.NewAgentServiceClient(conn).OpenSession(ctx)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}

	if err := handshake(stream, cfg, log); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	// gRPC 允许一个协程 Send、另一个协程 Recv 同时进行。
	// 接收协程处理 Controller 的回复，流结束时把原因交给主循环。
	recvErr := make(chan error, 1)
	go func() { recvErr <- receive(stream, log) }()

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	var seq uint64
	for {
		select {
		case <-ctx.Done():
			_ = stream.CloseSend()
			return nil

		case err := <-recvErr:
			// ctx 被取消时 Recv 也会返回错误，此时属于正常退出。
			if ctx.Err() != nil {
				return nil
			}
			return err

		case now := <-ticker.C:
			seq++
			if err := stream.Send(metricsBatch(seq, now)); err != nil {
				// 流断开后 Send 只返回 io.EOF，真正的原因由 Recv 得到。
				if errors.Is(err, io.EOF) {
					err = <-recvErr
				} else {
					err = fmt.Errorf("send metrics: %w", err)
				}
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}
}

// handshake 发送 Hello 并等待 HelloAck。
func handshake(stream agentv1.AgentService_OpenSessionClient, cfg Config, log *slog.Logger) error {
	err := stream.Send(&agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Hello{Hello: &agentv1.Hello{
			NodeId:       cfg.NodeID,
			InstanceId:   cfg.InstanceID,
			Hostname:     cfg.Hostname,
			AgentVersion: cfg.AgentVersion,
		}},
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("send hello: %w", err)
	}

	// Send 返回 io.EOF 时，这里的 Recv 会拿到真正的错误。
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive hello ack: %w", err)
	}
	ack := resp.GetHelloAck()
	if ack == nil {
		return errors.New("first response is not hello ack")
	}

	skew := time.Since(ack.GetServerTime().AsTime())
	if skew > maxClockSkew || skew < -maxClockSkew {
		log.Warn("clock skew with controller", "skew", skew)
	}
	log.Info("session established",
		"controller_addr", cfg.ControllerAddr,
		"last_acked_seq", ack.GetLastAckedSeq())
	return nil
}

// receive 读取 Controller 的回复，直到流结束，并返回结束的原因。
func receive(stream agentv1.AgentService_OpenSessionClient, log *slog.Logger) error {
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return ErrSessionClosed
		}
		if err != nil {
			return fmt.Errorf("receive: %w", err)
		}

		switch p := resp.GetPayload().(type) {
		case *agentv1.OpenSessionResponse_MetricsAck:
			// 缓冲还没有实现，确认暂时只记录日志。
			log.Debug("metrics acked", "acked_seq", p.MetricsAck.GetAckedSeq())
		case *agentv1.OpenSessionResponse_HelloAck:
			log.Warn("unexpected hello ack ignored")
		default:
			log.Warn("unknown payload ignored")
		}
	}
}

// metricsBatch 构造只含采样时刻的指标批次，真实指标由采集器的 Issue 填充。
func metricsBatch(seq uint64, collectedAt time.Time) *agentv1.OpenSessionRequest {
	return &agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Metrics{Metrics: &agentv1.MetricsBatch{
			Seq: seq,
			Samples: []*agentv1.MetricsSample{
				{CollectedAt: timestamppb.New(collectedAt)},
			},
		}},
	}
}
