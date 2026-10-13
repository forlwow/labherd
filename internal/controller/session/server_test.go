package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// openSession 在内存中启动一个 Server，并以 Agent 身份打开一条会话。
// bufconn 不占用真实端口，测试可以并行运行。
func openSession(t *testing.T) agentv1.AgentService_OpenSessionClient {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, NewServer(slog.New(slog.DiscardHandler)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	stream, err := agentv1.NewAgentServiceClient(conn).OpenSession(ctx)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	return stream
}

func helloReq() *agentv1.OpenSessionRequest {
	return &agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Hello{Hello: &agentv1.Hello{
			NodeId:     "node-1",
			InstanceId: "instance-1",
		}},
	}
}

func metricsReq(seq uint64) *agentv1.OpenSessionRequest {
	return &agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Metrics{Metrics: &agentv1.MetricsBatch{
			Seq:     seq,
			Samples: []*agentv1.MetricsSample{{CollectedAt: timestamppb.Now()}},
		}},
	}
}

func mustSend(t *testing.T, stream agentv1.AgentService_OpenSessionClient, req *agentv1.OpenSessionRequest) {
	t.Helper()
	if err := stream.Send(req); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func mustRecv(t *testing.T, stream agentv1.AgentService_OpenSessionClient) *agentv1.OpenSessionResponse {
	t.Helper()
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	return resp
}

func TestServer_AcksHelloAndMetrics(t *testing.T) {
	t.Parallel()
	stream := openSession(t)

	mustSend(t, stream, helloReq())
	ack := mustRecv(t, stream).GetHelloAck()
	if ack == nil {
		t.Fatal("first response is not hello_ack")
	}
	if ack.GetLastAckedSeq() != 0 {
		t.Errorf("last_acked_seq = %d, want 0", ack.GetLastAckedSeq())
	}
	if ack.GetServerTime() == nil {
		t.Error("server_time is not set")
	}

	for seq := uint64(1); seq <= 2; seq++ {
		mustSend(t, stream, metricsReq(seq))
		got := mustRecv(t, stream).GetMetricsAck()
		if got == nil {
			t.Fatalf("seq %d: response is not metrics_ack", seq)
		}
		if got.GetAckedSeq() != seq {
			t.Errorf("acked_seq = %d, want %d", got.GetAckedSeq(), seq)
		}
	}

	// Agent 主动结束会话时，服务端正常返回，客户端收到 EOF。
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close send: %v", err)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Errorf("recv after close = %v, want io.EOF", err)
	}
}

func TestServer_RejectsMissingHello(t *testing.T) {
	t.Parallel()
	stream := openSession(t)

	mustSend(t, stream, metricsReq(1))
	_, err := stream.Recv()
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition", code)
	}
}

func TestServer_RejectsEmptyNodeID(t *testing.T) {
	t.Parallel()
	stream := openSession(t)

	mustSend(t, stream, &agentv1.OpenSessionRequest{
		Payload: &agentv1.OpenSessionRequest_Hello{Hello: &agentv1.Hello{InstanceId: "instance-1"}},
	})
	_, err := stream.Recv()
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", code)
	}
}

func TestServer_RejectsDuplicateHello(t *testing.T) {
	t.Parallel()
	stream := openSession(t)

	mustSend(t, stream, helloReq())
	mustRecv(t, stream)

	mustSend(t, stream, helloReq())
	_, err := stream.Recv()
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition", code)
	}
}
