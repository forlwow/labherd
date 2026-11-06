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

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// ErrSessionClosed 表示 Controller 主动正常结束了会话。
var ErrSessionClosed = errors.New("session closed by controller")

// closeTimeout 是正常结束会话时，等待 Controller 确认的最长时间。
const closeTimeout = 2 * time.Second

// maxClockSkew 是本地时钟与 Controller 时钟允许的最大偏差，超过时记录警告。
const maxClockSkew = 5 * time.Second

// Sampler 在给定时刻采集一次整机指标。
// 由 collector 包实现；定义在使用方，测试时可以替换为假实现。
type Sampler interface {
	Sample(ctx context.Context, now time.Time) *agentv1.MetricsSample
}

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
	// Host 是主机的静态信息，随 Hello 发送。
	Host *agentv1.HostInfo
	// GPUs 是本机的 GPU 清单，随 Hello 发送；没有 GPU 时为空。
	GPUs []*agentv1.GpuInfo
	// Sampler 每个周期采集一次指标。
	Sampler Sampler
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

	// 流不随 ctx 取消：ctx 取消（Ctrl+C、systemd stop）后，还要先通知 Controller 正常结束，
	// 等它确认后再断开，Controller 才会把节点标为离线而不是失联。
	// 函数返回时取消 streamCtx，使流和接收协程一起结束。
	streamCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	stream, err := agentv1.NewAgentServiceClient(conn).OpenSession(streamCtx)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}

	// 握手期间 ctx 被取消时直接断开，不必等待 HelloAck。
	stopWatch := context.AfterFunc(ctx, cancel)
	err = handshake(stream, cfg, log)
	stopWatch()
	if err != nil {
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
			closeSession(stream, recvErr, log)
			return nil

		case err := <-recvErr:
			// ctx 被取消时 Recv 也会返回错误，此时属于正常退出。
			if ctx.Err() != nil {
				return nil
			}
			return err

		case now := <-ticker.C:
			seq++
			sample := cfg.Sampler.Sample(ctx, now)
			if err := stream.Send(metricsBatch(seq, sample)); err != nil {
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
			logSample(log, seq, sample)
		}
	}
}

// closeSession 通知 Controller 不再发送消息，并等待它结束会话，最多等待 closeTimeout。
// Controller 收到 EOF 后正常返回，接收协程随之得到 ErrSessionClosed。
func closeSession(stream agentv1.AgentService_OpenSessionClient, recvErr <-chan error, log *slog.Logger) {
	if err := stream.CloseSend(); err != nil {
		return
	}
	select {
	case err := <-recvErr:
		if !errors.Is(err, ErrSessionClosed) {
			log.Debug("session ended while closing", "err", err)
		}
	case <-time.After(closeTimeout):
		log.Warn("controller did not confirm session close in time")
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
			Host:         cfg.Host,
			Gpus:         cfg.GPUs,
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

// metricsBatch 把一个采样包装成批次。断线补发时一批会有多个采样，由缓冲的 Issue 实现。
func metricsBatch(seq uint64, sample *agentv1.MetricsSample) *agentv1.OpenSessionRequest {
	return &agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Metrics{Metrics: &agentv1.MetricsBatch{
			Seq:     seq,
			Samples: []*agentv1.MetricsSample{sample},
		}},
	}
}

// logSample 以 DEBUG 级别记录发出的指标，便于本地调试：一行主机指标，每张 GPU 各一行。
// GPU 指标读不到时不设置，日志中也不输出该字段，以便与值为 0 区分。
func logSample(log *slog.Logger, seq uint64, sample *agentv1.MetricsSample) {
	if !log.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	h := sample.GetHost()
	log.Debug("metrics sent",
		"seq", seq,
		"cpu_utilization_ratio", h.GetCpuUtilizationRatio(),
		"load1", h.GetLoad1(),
		"memory_available_bytes", h.GetMemoryAvailableBytes(),
		"disk_count", len(h.GetDisks()),
		"gpu_count", len(sample.GetGpus()))

	for _, g := range sample.GetGpus() {
		attrs := []any{"seq", seq, "uuid", g.GetUuid()}
		if g.UtilizationRatio != nil {
			attrs = append(attrs, "utilization_ratio", g.GetUtilizationRatio())
		}
		if g.MemoryUsedBytes != nil {
			attrs = append(attrs, "memory_used_bytes", g.GetMemoryUsedBytes())
		}
		if g.TemperatureCelsius != nil {
			attrs = append(attrs, "temperature_celsius", g.GetTemperatureCelsius())
		}
		if g.PowerWatts != nil {
			attrs = append(attrs, "power_watts", g.GetPowerWatts())
		}
		log.Debug("gpu metrics sent", attrs...)
	}
}
