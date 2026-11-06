package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

const testTimeout = 10 * time.Second

// fakeClock 是可以手动拨动的时钟。注册表在持锁时读取它，测试只在单个协程中拨动，不需要加锁。
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestRegistry() (*Registry, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)}
	r := New(testTimeout)
	r.now = clock.now
	return r, clock
}

func hello(nodeID, instanceID string) *agentv1.Hello {
	return &agentv1.Hello{
		NodeId:     nodeID,
		InstanceId: instanceID,
		Hostname:   nodeID + "-host",
		Gpus:       []*agentv1.GpuInfo{{Uuid: "GPU-" + nodeID}},
	}
}

func sampleAt(t time.Time) *agentv1.MetricsSample {
	return &agentv1.MetricsSample{CollectedAt: timestamppb.New(t)}
}

func mustGet(t *testing.T, r *Registry, nodeID string) Node {
	t.Helper()
	n, ok := r.Get(nodeID)
	if !ok {
		t.Fatalf("node %s not found", nodeID)
	}
	return n
}

func TestRegistry_Status(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// act 在会话建立后执行，返回之前拨动时钟或结束会话。
		act  func(s *Session, clock *fakeClock)
		want Status
	}{
		{"just connected", func(*Session, *fakeClock) {}, StatusOnline},
		{"seen within timeout", func(s *Session, c *fakeClock) {
			c.advance(testTimeout)
			s.Touch()
			c.advance(testTimeout)
		}, StatusOnline},
		{"silent past timeout", func(_ *Session, c *fakeClock) { c.advance(testTimeout + time.Second) }, StatusLost},
		{"closed cleanly", func(s *Session, _ *fakeClock) { s.Close(nil) }, StatusOffline},
		{"closed with error", func(s *Session, _ *fakeClock) { s.Close(errors.New("broken pipe")) }, StatusLost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, clock := newTestRegistry()
			s, _ := r.Connect(t.Context(), hello("node-1", "inst-1"))
			tt.act(s, clock)
			if got := mustGet(t, r, "node-1").Status; got != tt.want {
				t.Errorf("status = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRegistry_ConnectStoresHello(t *testing.T) {
	t.Parallel()
	r, clock := newTestRegistry()
	r.Connect(t.Context(), hello("node-1", "inst-1"))

	n := mustGet(t, r, "node-1")
	if n.InstanceID != "inst-1" || n.Hostname != "node-1-host" ||
		len(n.GPUs) != 1 || !n.ConnectedAt.Equal(clock.t) {
		t.Errorf("unexpected node: %+v", n)
	}
	if _, ok := r.Get("node-2"); ok {
		t.Error("unknown node should not be found")
	}
}

func TestRegistry_NewSessionSupersedesOld(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()
	oldSess, oldCtx := r.Connect(t.Context(), hello("node-1", "inst-1"))
	newSess, newCtx := r.Connect(t.Context(), hello("node-1", "inst-1"))

	if !errors.Is(context.Cause(oldCtx), ErrSuperseded) {
		t.Errorf("old ctx cause = %v, want ErrSuperseded", context.Cause(oldCtx))
	}
	if newCtx.Err() != nil {
		t.Errorf("new ctx should stay alive, err = %v", newCtx.Err())
	}

	// 旧会话随后报错结束，不能把已被新会话接管的节点标为失联。
	oldSess.Close(errors.New("stream canceled"))
	if got := mustGet(t, r, "node-1").Status; got != StatusOnline {
		t.Errorf("status after old session closed = %s, want online", got)
	}

	newSess.Close(nil)
	if got := mustGet(t, r, "node-1").Status; got != StatusOffline {
		t.Errorf("status after new session closed = %s, want offline", got)
	}
}

func TestRegistry_CloseCancelsSessionContext(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()
	s, ctx := r.Connect(t.Context(), hello("node-1", "inst-1"))
	s.Close(nil)
	s.Close(nil) // 重复调用不应 panic
	if ctx.Err() == nil {
		t.Error("ctx should be canceled after Close")
	}
}

func TestRegistry_ClosedSessionIgnoresUpdates(t *testing.T) {
	t.Parallel()
	r, clock := newTestRegistry()
	s, _ := r.Connect(t.Context(), hello("node-1", "inst-1"))
	s.Close(nil)

	clock.advance(time.Second)
	s.Touch()
	s.RecordSample(sampleAt(clock.t))

	n := mustGet(t, r, "node-1")
	if n.Status != StatusOffline || n.LastSample != nil || n.LastSeenAt.Equal(clock.t) {
		t.Errorf("closed session should not update node: %+v", n)
	}
}

func TestRegistry_RecordSampleKeepsNewest(t *testing.T) {
	t.Parallel()
	r, clock := newTestRegistry()
	s, _ := r.Connect(t.Context(), hello("node-1", "inst-1"))

	newer := sampleAt(clock.t)
	s.RecordSample(newer)
	// 断线补发的旧采样晚到，不应覆盖较新的采样。
	s.RecordSample(sampleAt(clock.t.Add(-time.Minute)))

	if got := mustGet(t, r, "node-1").LastSample; got != newer {
		t.Errorf("last sample = %v, want the newer one", got)
	}
}

func TestRegistry_SampleResetOnAgentRestart(t *testing.T) {
	t.Parallel()
	r, clock := newTestRegistry()

	s, _ := r.Connect(t.Context(), hello("node-1", "inst-1"))
	s.RecordSample(sampleAt(clock.t))
	s.Close(errors.New("network down"))

	// 同一 Agent 进程重连：保留上次的采样。
	s, _ = r.Connect(t.Context(), hello("node-1", "inst-1"))
	if mustGet(t, r, "node-1").LastSample == nil {
		t.Error("sample should survive reconnect of the same instance")
	}
	s.Close(nil)

	// Agent 重启：丢弃旧进程的采样。
	r.Connect(t.Context(), hello("node-1", "inst-2"))
	if mustGet(t, r, "node-1").LastSample != nil {
		t.Error("sample should be reset after agent restart")
	}
}

func TestRegistry_ListSortedByNodeID(t *testing.T) {
	t.Parallel()
	r, _ := newTestRegistry()
	for _, id := range []string{"node-c", "node-a", "node-b"} {
		r.Connect(t.Context(), hello(id, "inst-1"))
	}

	nodes := r.List()
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	for i, want := range []string{"node-a", "node-b", "node-c"} {
		if nodes[i].NodeID != want {
			t.Errorf("nodes[%d] = %s, want %s", i, nodes[i].NodeID, want)
		}
	}
}

// TestRegistry_ConcurrentAccess 供 -race 检查并发安全，不检查结果。
func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	r := New(testTimeout)
	done := make(chan struct{})
	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				s, _ := r.Connect(t.Context(), hello("node-1", "inst-1"))
				s.Touch()
				s.RecordSample(sampleAt(time.Now()))
				_ = r.List()
				s.Close(nil)
			}
		}()
	}
	for range 4 {
		<-done
	}
}
