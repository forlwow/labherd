package transport

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// fakeController 是测试用的 Controller：回复 HelloAck，确认每个批次，
// 并把收到的 Hello 与批次序号交给测试。
// agent 不能导入 controller 的实现，所以这里单独写一个。
type fakeController struct {
	agentv1.UnimplementedAgentServiceServer

	hellos chan *agentv1.Hello
	seqs   chan uint64
	// closeAfter > 0 时，收到这么多个批次后主动结束会话。
	closeAfter int
}

func (f *fakeController) OpenSession(stream agentv1.AgentService_OpenSessionServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	f.hellos <- first.GetHello()

	err = stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_HelloAck{HelloAck: &agentv1.HelloAck{
			ServerTime: timestamppb.Now(),
		}},
	})
	if err != nil {
		return err
	}

	for received := 0; ; {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		batch := req.GetMetrics()
		if batch == nil {
			continue
		}
		f.seqs <- batch.GetSeq()
		err = stream.Send(&agentv1.OpenSessionResponse{
			Payload: &agentv1.OpenSessionResponse_MetricsAck{MetricsAck: &agentv1.MetricsAck{
				AckedSeq: batch.GetSeq(),
			}},
		})
		if err != nil {
			return err
		}
		received++
		if f.closeAfter > 0 && received >= f.closeAfter {
			return nil
		}
	}
}

// startFake 在本机随机端口启动 fakeController，返回其地址。
func startFake(t *testing.T, f *fakeController) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func newFake(closeAfter int) *fakeController {
	return &fakeController{
		hellos:     make(chan *agentv1.Hello, 1),
		seqs:       make(chan uint64, 100),
		closeAfter: closeAfter,
	}
}

func testConfig(addr string) Config {
	return Config{
		ControllerAddr: addr,
		NodeID:         "node-1",
		InstanceID:     "instance-1",
		Hostname:       "host-1",
		AgentVersion:   "test",
		Interval:       10 * time.Millisecond,
	}
}

// runAsync 在后台运行 Run，返回接收其结果的通道。
func runAsync(ctx context.Context, cfg Config) <-chan error {
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, slog.New(slog.DiscardHandler)) }()
	return done
}

func waitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return in time")
		return nil
	}
}

func TestRun_SendsHelloThenSequentialBatches(t *testing.T) {
	t.Parallel()
	fake := newFake(0)
	addr := startFake(t, fake)

	ctx, cancel := context.WithCancel(t.Context())
	done := runAsync(ctx, testConfig(addr))

	hello := <-fake.hellos
	if hello.GetNodeId() != "node-1" || hello.GetInstanceId() != "instance-1" {
		t.Errorf("hello = %v, want node-1/instance-1", hello)
	}
	for want := uint64(1); want <= 3; want++ {
		if got := <-fake.seqs; got != want {
			t.Fatalf("seq = %d, want %d", got, want)
		}
	}

	cancel()
	if err := waitResult(t, done); err != nil {
		t.Errorf("Run after cancel = %v, want nil", err)
	}
}

func TestRun_ReturnsErrSessionClosedWhenControllerEnds(t *testing.T) {
	t.Parallel()
	addr := startFake(t, newFake(1))

	err := waitResult(t, runAsync(t.Context(), testConfig(addr)))
	if !errors.Is(err, ErrSessionClosed) {
		t.Errorf("err = %v, want ErrSessionClosed", err)
	}
}

func TestRun_ReturnsUnavailableWithoutController(t *testing.T) {
	t.Parallel()

	// 取得一个空闲端口后立即关闭，保证上面没有服务。
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	err = waitResult(t, runAsync(t.Context(), testConfig(addr)))
	// status.Code 能识别经过 %w 包装的 gRPC 错误。
	if code := status.Code(err); code != codes.Unavailable {
		t.Errorf("err = %v, want Unavailable", err)
	}
}
