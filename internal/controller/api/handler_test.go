package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/controller/registry"
)

// fakeLister 返回固定的节点。
type fakeLister []registry.Node

func (f fakeLister) List() []registry.Node { return f }

func (f fakeLister) Get(nodeID string) (registry.Node, bool) {
	for _, n := range f {
		if n.NodeID == nodeID {
			return n, true
		}
	}
	return registry.Node{}, false
}

var testTime = time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)

// testNodes 包含一个有采样的在线节点和一个尚无采样的离线节点。
func testNodes() fakeLister {
	const gib = 1 << 30
	return fakeLister{
		{
			NodeID: "gpu-01",
			Host:   &agentv1.HostInfo{CpuModel: "Test CPU", MemoryTotalBytes: 64 * gib},
			GPUs: []*agentv1.GpuInfo{
				{Uuid: "GPU-a", Index: 0, Model: "RTX 5080", MemoryTotalBytes: 16 * gib},
				{Uuid: "GPU-b", Index: 1, Model: "RTX 5080", MemoryTotalBytes: 16 * gib},
			},
			Status:      registry.StatusOnline,
			ConnectedAt: testTime,
			LastSeenAt:  testTime,
			LastSample: &agentv1.MetricsSample{
				CollectedAt: timestamppb.New(testTime),
				Host: &agentv1.HostMetrics{
					CpuUtilizationRatio:  0.5,
					MemoryAvailableBytes: 48 * gib,
					Disks:                []*agentv1.DiskUsage{{MountPoint: "/", TotalBytes: 100, AvailableBytes: 40}},
				},
				Gpus: []*agentv1.GpuMetrics{
					// GPU-b 读不到温度与功耗。
					{
						Uuid: "GPU-a", UtilizationRatio: proto.Float64(0.8), MemoryUsedBytes: proto.Uint64(gib),
						TemperatureCelsius: proto.Float64(60), PowerWatts: proto.Float64(200),
					},
					{Uuid: "GPU-b", UtilizationRatio: proto.Float64(0), MemoryUsedBytes: proto.Uint64(0)},
				},
			},
		},
		{
			NodeID:         "gpu-02",
			Status:         registry.StatusOffline,
			ConnectedAt:    testTime,
			LastSeenAt:     testTime,
			DisconnectedAt: testTime.Add(time.Minute),
		},
	}
}

// get 发出请求，检查状态码，并把响应体解码为通用的 map，便于检查 JSON 字段名与 null。
func get(t *testing.T, path string, wantCode int) map[string]any {
	t.Helper()
	h := NewHandler(testNodes(), slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != wantCode {
		t.Fatalf("GET %s: code = %d, want %d, body = %s", path, rec.Code, wantCode, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestHandler_ListNodes(t *testing.T) {
	t.Parallel()
	body := get(t, "/api/v1/nodes", http.StatusOK)
	nodes, _ := body["nodes"].([]any)
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}

	online := nodes[0].(map[string]any)
	if online["node_id"] != "gpu-01" || online["status"] != "online" || online["disconnected_at"] != nil {
		t.Errorf("unexpected online node: %v", online)
	}
	host := online["host"].(map[string]any)
	// 已用内存 = 64 GiB - 48 GiB。
	if host["memory_used_bytes"] != float64(16<<30) || host["cpu_utilization_ratio"] != 0.5 {
		t.Errorf("unexpected host: %v", host)
	}

	gpus := online["gpus"].([]any)
	a, b := gpus[0].(map[string]any), gpus[1].(map[string]any)
	if a["utilization_ratio"] != 0.8 || a["power_watts"] != float64(200) {
		t.Errorf("unexpected gpu a: %v", a)
	}
	// 读不到的指标为 null；值为 0 的指标为 0。
	if b["temperature_celsius"] != nil || b["power_watts"] != nil {
		t.Errorf("unsupported metrics should be null: %v", b)
	}
	if b["utilization_ratio"] != float64(0) {
		t.Errorf("zero utilization should be 0, got %v", b["utilization_ratio"])
	}

	offline := nodes[1].(map[string]any)
	if offline["status"] != "offline" || offline["sampled_at"] != nil || offline["disconnected_at"] == nil {
		t.Errorf("unexpected offline node: %v", offline)
	}
	// 尚无采样时，数组输出为 []，而不是 null。
	if gpus, ok := offline["gpus"].([]any); !ok || len(gpus) != 0 {
		t.Errorf("gpus = %v, want []", offline["gpus"])
	}
}

func TestHandler_GetNode(t *testing.T) {
	t.Parallel()
	body := get(t, "/api/v1/nodes/gpu-01", http.StatusOK)
	if body["node_id"] != "gpu-01" {
		t.Errorf("node_id = %v, want gpu-01", body["node_id"])
	}
}

func TestHandler_GetNodeNotFound(t *testing.T) {
	t.Parallel()
	body := get(t, "/api/v1/nodes/nope", http.StatusNotFound)
	if body["error"] != "node not found" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestHandler_RejectsOtherMethods(t *testing.T) {
	t.Parallel()
	h := NewHandler(testNodes(), slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/nodes", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
}
