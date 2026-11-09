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
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/forlwow/labherd/internal/controller/registry"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// openSession 在内存中启动一个 Server，并以 Agent 身份打开一条会话。
// bufconn 不占用真实端口，测试可以并行运行。
func openSession(t *testing.T) agentv1.AgentService_OpenSessionClient {
	t.Helper()
	return startServer(t).open(t)
}

// testServer 是在内存中运行的 Server 及其注册表。
type testServer struct {
	lis *bufconn.Listener
	reg *registry.Registry
}

func startServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{lis: bufconn.Listen(1 << 20), reg: registry.New(time.Minute)}
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, NewServer(ts.reg, slog.New(slog.DiscardHandler)))
	go func() { _ = srv.Serve(ts.lis) }()
	t.Cleanup(srv.Stop)
	return ts
}

// open 以 Agent 身份打开一条会话。
func (ts *testServer) open(t *testing.T) agentv1.AgentService_OpenSessionClient {
	t.Helper()
	lis := ts.lis
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

// waitStatus 等待节点进入 want 状态。会话结束后注册表的更新发生在服务端协程中，需要轮询。
func waitStatus(t *testing.T, reg *registry.Registry, nodeID string, want registry.Status) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, ok := reg.Get(nodeID)
		if ok && n.Status == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("node %s status = %q (found=%v), want %q", nodeID, n.Status, ok, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestServer_RegistersNodeAndSample(t *testing.T) {
	t.Parallel()
	ts := startServer(t)
	stream := ts.open(t)

	mustSend(t, stream, helloReq())
	mustRecv(t, stream)
	waitStatus(t, ts.reg, "node-1", registry.StatusOnline)

	mustSend(t, stream, metricsReq(1))
	mustRecv(t, stream)
	if n, _ := ts.reg.Get("node-1"); n.LastSample == nil {
		t.Error("last sample should be recorded after metrics ack")
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close send: %v", err)
	}
	waitStatus(t, ts.reg, "node-1", registry.StatusOffline)
}

func TestServer_NewSessionSupersedesOld(t *testing.T) {
	t.Parallel()
	ts := startServer(t)

	oldStream := ts.open(t)
	mustSend(t, oldStream, helloReq())
	mustRecv(t, oldStream)

	newStream := ts.open(t)
	mustSend(t, newStream, helloReq())
	mustRecv(t, newStream)

	// 旧会话被服务端以 Aborted 关闭。
	if _, err := oldStream.Recv(); status.Code(err) != codes.Aborted {
		t.Errorf("old session err = %v, want Aborted", err)
	}

	// 新会话不受影响，节点保持在线。
	mustSend(t, newStream, metricsReq(1))
	if mustRecv(t, newStream).GetMetricsAck() == nil {
		t.Error("new session should keep working")
	}
	waitStatus(t, ts.reg, "node-1", registry.StatusOnline)
}

func TestServer_KeepsAdvertisedIP(t *testing.T) {
	t.Parallel()
	ts := startServer(t)
	stream := ts.open(t)

	req := helloReq()
	req.GetHello().Ip = "10.10.3.21"
	mustSend(t, stream, req)
	mustRecv(t, stream)

	if n, _ := ts.reg.Get("node-1"); n.IP != "10.10.3.21" {
		t.Errorf("ip = %q, want 10.10.3.21", n.IP)
	}
}

// fakeAddr 是非 TCP 的连接地址，如 bufconn。
type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) }

func TestPeerIP(t *testing.T) {
	t.Parallel()
	withPeer := func(addr net.Addr) context.Context {
		return peer.NewContext(context.Background(), &peer.Peer{Addr: addr})
	}
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"ipv4", withPeer(&net.TCPAddr{IP: net.ParseIP("10.10.3.21"), Port: 51234}), "10.10.3.21"},
		{"ipv6", withPeer(&net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 51234}), "fe80::1"},
		{"host:port string", withPeer(fakeAddr("192.168.1.2:9000")), "192.168.1.2"},
		{"not host:port", withPeer(fakeAddr("bufconn")), ""},
		{"no peer", context.Background(), ""},
	}
	for _, tt := range tests {
		if got := peerIP(tt.ctx); got != tt.want {
			t.Errorf("%s: peerIP = %q, want %q", tt.name, got, tt.want)
		}
	}
}
