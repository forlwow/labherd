// 与 Controller 的 GET /api/v1/nodes 响应一一对应，见 internal/controller/api/nodes.go。
// 字段名保持接口中的小写下划线；读不到或尚无采样的数值为 null。

export type NodeStatus = 'online' | 'lost' | 'offline';

export interface Disk {
  mount_point: string;
  total_bytes: number;
  available_bytes: number;
}

export interface Host {
  os: string;
  kernel_version: string;
  cpu_model: string;
  cpu_logical_cores: number;
  memory_total_bytes: number;
  gpu_driver_version: string;

  cpu_utilization_ratio: number | null;
  load1: number | null;
  memory_available_bytes: number | null;
  memory_used_bytes: number | null;
  disks: Disk[];
}

export interface Gpu {
  uuid: string;
  index: number;
  model: string;
  memory_total_bytes: number;

  utilization_ratio: number | null;
  memory_used_bytes: number | null;
  temperature_celsius: number | null;
  power_watts: number | null;
}

export interface Node {
  node_id: string;
  instance_id: string;
  hostname: string;
  // 节点 IP，大家习惯用它认服务器。Controller 尚未提供时缺省，页面退回显示主机名。
  ip?: string | null;
  agent_version: string;

  status: NodeStatus;
  connected_at: string;
  last_seen_at: string;
  disconnected_at: string | null;
  sampled_at: string | null;

  host: Host;
  gpus: Gpu[];
}

export interface NodeListResponse {
  nodes: Node[];
}
