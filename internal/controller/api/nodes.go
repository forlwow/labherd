package api

import (
	"time"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"github.com/forlwow/labherd/internal/controller/registry"
)

// 以下类型是接口返回的 JSON 结构。字段名用小写下划线，单位写进字段名，与 proto 保持一致。
// 读不到或尚无采样的数值用指针，输出为 null，以便前端与 0 区分。

// nodeListResponse 是 GET /api/v1/nodes 的响应。
type nodeListResponse struct {
	Nodes []nodeJSON `json:"nodes"`
}

// nodeJSON 是一个节点的静态信息、在线状态和最近一次采样。
type nodeJSON struct {
	NodeID       string `json:"node_id"`
	InstanceID   string `json:"instance_id"`
	Hostname     string `json:"hostname"`
	AgentVersion string `json:"agent_version"`

	Status         string     `json:"status"`
	ConnectedAt    time.Time  `json:"connected_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	DisconnectedAt *time.Time `json:"disconnected_at"`
	// SampledAt 是最近一次采样的时刻；尚未收到指标时为 null。
	SampledAt *time.Time `json:"sampled_at"`

	Host hostJSON  `json:"host"`
	GPUs []gpuJSON `json:"gpus"`
}

// hostJSON 是主机的静态信息与最近一次采样的主机指标。
type hostJSON struct {
	OS               string `json:"os"`
	KernelVersion    string `json:"kernel_version"`
	CPUModel         string `json:"cpu_model"`
	CPULogicalCores  uint32 `json:"cpu_logical_cores"`
	MemoryTotalBytes uint64 `json:"memory_total_bytes"`
	GPUDriverVersion string `json:"gpu_driver_version"`

	CPUUtilizationRatio  *float64   `json:"cpu_utilization_ratio"`
	Load1                *float64   `json:"load1"`
	MemoryAvailableBytes *uint64    `json:"memory_available_bytes"`
	MemoryUsedBytes      *uint64    `json:"memory_used_bytes"`
	Disks                []diskJSON `json:"disks"`
}

// diskJSON 是一个挂载点的空间。
type diskJSON struct {
	MountPoint     string `json:"mount_point"`
	TotalBytes     uint64 `json:"total_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}

// gpuJSON 是一张 GPU 的静态信息与最近一次采样的指标。
type gpuJSON struct {
	UUID             string `json:"uuid"`
	Index            uint32 `json:"index"`
	Model            string `json:"model"`
	MemoryTotalBytes uint64 `json:"memory_total_bytes"`

	UtilizationRatio   *float64 `json:"utilization_ratio"`
	MemoryUsedBytes    *uint64  `json:"memory_used_bytes"`
	TemperatureCelsius *float64 `json:"temperature_celsius"`
	PowerWatts         *float64 `json:"power_watts"`
}

// toNodeJSON 把注册表中的节点转换为 JSON 结构。
func toNodeJSON(n registry.Node) nodeJSON {
	h := n.Host
	out := nodeJSON{
		NodeID:       n.NodeID,
		InstanceID:   n.InstanceID,
		Hostname:     n.Hostname,
		AgentVersion: n.AgentVersion,
		Status:       string(n.Status),
		ConnectedAt:  n.ConnectedAt,
		LastSeenAt:   n.LastSeenAt,
		Host: hostJSON{
			OS:               h.GetOs(),
			KernelVersion:    h.GetKernelVersion(),
			CPUModel:         h.GetCpuModel(),
			CPULogicalCores:  h.GetCpuLogicalCores(),
			MemoryTotalBytes: h.GetMemoryTotalBytes(),
			GPUDriverVersion: h.GetGpuDriverVersion(),
			Disks:            []diskJSON{},
		},
		GPUs: make([]gpuJSON, 0, len(n.GPUs)),
	}
	if !n.DisconnectedAt.IsZero() {
		out.DisconnectedAt = &n.DisconnectedAt
	}

	// GPU 以 Hello 中的清单为准，按 UUID 匹配最近一次采样的指标。
	var gpuMetrics map[string]*agentv1.GpuMetrics
	if s := n.LastSample; s != nil {
		sampledAt := s.GetCollectedAt().AsTime()
		out.SampledAt = &sampledAt
		fillHostMetrics(&out.Host, s.GetHost())

		gpuMetrics = make(map[string]*agentv1.GpuMetrics, len(s.GetGpus()))
		for _, m := range s.GetGpus() {
			gpuMetrics[m.GetUuid()] = m
		}
	}

	for _, g := range n.GPUs {
		gj := gpuJSON{
			UUID:             g.GetUuid(),
			Index:            g.GetIndex(),
			Model:            g.GetModel(),
			MemoryTotalBytes: g.GetMemoryTotalBytes(),
		}
		if m, ok := gpuMetrics[g.GetUuid()]; ok {
			gj.UtilizationRatio = m.UtilizationRatio
			gj.MemoryUsedBytes = m.MemoryUsedBytes
			gj.TemperatureCelsius = m.TemperatureCelsius
			gj.PowerWatts = m.PowerWatts
		}
		out.GPUs = append(out.GPUs, gj)
	}
	return out
}

// fillHostMetrics 填入采样中的主机指标。
// 已用内存按 proto 的约定由 Controller 计算：总量减去可用量。
func fillHostMetrics(h *hostJSON, m *agentv1.HostMetrics) {
	if m == nil {
		return
	}
	cpu := m.GetCpuUtilizationRatio()
	load1 := m.GetLoad1()
	available := m.GetMemoryAvailableBytes()
	h.CPUUtilizationRatio = &cpu
	h.Load1 = &load1
	h.MemoryAvailableBytes = &available
	if h.MemoryTotalBytes >= available {
		used := h.MemoryTotalBytes - available
		h.MemoryUsedBytes = &used
	}
	for _, d := range m.GetDisks() {
		h.Disks = append(h.Disks, diskJSON{
			MountPoint:     d.GetMountPoint(),
			TotalBytes:     d.GetTotalBytes(),
			AvailableBytes: d.GetAvailableBytes(),
		})
	}
}
