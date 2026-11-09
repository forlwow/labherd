// Package registry 维护 Controller 已知的节点及其在线状态。
//
// 全部状态保存在内存中，Controller 重启后清空，Agent 重连后重新登记。
package registry

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
)

// ErrSuperseded 是旧会话被同一节点的新会话取代时，其 ctx 的取消原因。
var ErrSuperseded = errors.New("session superseded by a newer one")

// Status 是节点的在线状态。
type Status string

// 节点状态。取值直接用于 JSON 输出，修改时注意兼容前端。
const (
	// StatusOnline 表示会话存在，且超时时间内收到过消息。
	StatusOnline Status = "online"
	// StatusLost 表示会话超时没有消息，或会话异常断开（网络中断、Agent 崩溃）。
	StatusLost Status = "lost"
	// StatusOffline 表示 Agent 正常结束了会话（如停止服务）。
	StatusOffline Status = "offline"
)

// Node 是某一时刻节点信息的副本。
//
// 其中的 proto 消息与注册表共享，调用方只能读取，不得修改。
type Node struct {
	NodeID     string
	InstanceID string
	Hostname   string
	// IP 仅用于显示：Agent 指定的地址，或 Controller 看到的连接来源地址。
	IP           string
	AgentVersion string
	Host         *agentv1.HostInfo
	GPUs         []*agentv1.GpuInfo

	Status Status
	// ConnectedAt 是最近一次会话建立的时刻。
	ConnectedAt time.Time
	// LastSeenAt 是最近一次收到该节点任意消息的时刻。
	LastSeenAt time.Time
	// DisconnectedAt 是最近一次会话结束的时刻；会话仍在时为零值。
	DisconnectedAt time.Time
	// LastSample 是采样时刻最新的一次采样；尚未收到指标时为 nil。
	LastSample *agentv1.MetricsSample
}

// Registry 保存所有节点。可被多个协程同时使用。
type Registry struct {
	timeout time.Duration
	now     func() time.Time

	mu     sync.Mutex
	nodes  map[string]*entry
	nextID uint64
}

// entry 是注册表内部的节点记录。
type entry struct {
	node Node // Status 字段不使用，读取时由 status 计算

	sessionID uint64 // 当前会话的编号，用于识别已被取代的旧会话
	connected bool
	cleanExit bool
	cancel    context.CancelCauseFunc
}

// New 返回空的注册表。会话超过 timeout 没有收到消息时，节点视为失联。
func New(timeout time.Duration) *Registry {
	return &Registry{
		timeout: timeout,
		now:     time.Now,
		nodes:   make(map[string]*entry),
	}
}

// Session 是注册表中的一条会话。会话结束时必须调用 Close。
type Session struct {
	r      *Registry
	nodeID string
	id     uint64
	cancel context.CancelCauseFunc
}

// Connect 为 hello 对应的节点登记一条新会话。
//
// 返回的 ctx 派生自 parent。同一节点再次 Connect 时，旧会话的 ctx 以 ErrSuperseded 取消，
// 持有旧会话的一方应据此结束会话。按协议，同一节点同一时刻只保留一个会话：
// 断网后旧连接可能尚未被发现断开，新会话总要能顶替它。
func (r *Registry) Connect(parent context.Context, hello *agentv1.Hello) (*Session, context.Context) {
	ctx, cancel := context.WithCancelCause(parent)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	now := r.now()
	e, ok := r.nodes[hello.GetNodeId()]
	if !ok {
		e = &entry{}
		r.nodes[hello.GetNodeId()] = e
	} else if e.connected {
		e.cancel(ErrSuperseded)
	}

	// Agent 重启（instance_id 变化）后，旧进程的采样不再代表当前状态。
	lastSample := e.node.LastSample
	if e.node.InstanceID != hello.GetInstanceId() {
		lastSample = nil
	}

	e.node = Node{
		NodeID:       hello.GetNodeId(),
		InstanceID:   hello.GetInstanceId(),
		Hostname:     hello.GetHostname(),
		AgentVersion: hello.GetAgentVersion(),
		Host:         hello.GetHost(),
		GPUs:         hello.GetGpus(),
		ConnectedAt:  now,
		LastSeenAt:   now,
		LastSample:   lastSample,
		IP:           hello.GetIp(),
	}
	e.sessionID = r.nextID
	e.connected = true
	e.cleanExit = false
	e.cancel = cancel

	return &Session{r: r, nodeID: hello.GetNodeId(), id: r.nextID, cancel: cancel}, ctx
}

// Touch 记录收到了该会话的一条消息。
func (s *Session) Touch() {
	s.update(func(e *entry, now time.Time) {
		e.node.LastSeenAt = now
	})
}

// RecordSample 记录收到的采样，同时视为收到一条消息。
// 只保留采样时刻最新的一次：断线补发的旧采样不会覆盖较新的采样。
func (s *Session) RecordSample(sample *agentv1.MetricsSample) {
	s.update(func(e *entry, now time.Time) {
		e.node.LastSeenAt = now
		last := e.node.LastSample
		if last == nil || sample.GetCollectedAt().AsTime().After(last.GetCollectedAt().AsTime()) {
			e.node.LastSample = sample
		}
	})
}

// Close 结束会话。err 为 nil 表示 Agent 正常结束，节点标为离线；否则标为失联。
// 会话已被取代时，只释放资源，不改变节点状态。可以重复调用。
func (s *Session) Close(err error) {
	s.update(func(e *entry, now time.Time) {
		e.connected = false
		e.cleanExit = err == nil
		e.node.DisconnectedAt = now
	})
	s.cancel(context.Canceled)
}

// update 在会话仍是节点的当前会话时，对节点记录执行 fn。
func (s *Session) update(fn func(e *entry, now time.Time)) {
	r := s.r
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.nodes[s.nodeID]
	if !ok || e.sessionID != s.id || !e.connected {
		return
	}
	fn(e, r.now())
}

// List 返回所有节点，按 NodeID 排序。
func (r *Registry) List() []Node {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	nodes := make([]Node, 0, len(r.nodes))
	for _, e := range r.nodes {
		nodes = append(nodes, r.view(e, now))
	}
	slices.SortFunc(nodes, func(a, b Node) int { return strings.Compare(a.NodeID, b.NodeID) })
	return nodes
}

// Get 返回指定节点；节点不存在时 ok 为 false。
func (r *Registry) Get(nodeID string) (node Node, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.nodes[nodeID]
	if !ok {
		return Node{}, false
	}
	return r.view(e, r.now()), true
}

// view 返回节点记录的副本，并按当前时刻计算状态。调用方须持有 r.mu。
func (r *Registry) view(e *entry, now time.Time) Node {
	n := e.node
	switch {
	case e.connected && now.Sub(n.LastSeenAt) <= r.timeout:
		n.Status = StatusOnline
	case !e.connected && e.cleanExit:
		n.Status = StatusOffline
	default:
		n.Status = StatusLost
	}
	return n
}
