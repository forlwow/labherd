import type { Disk, Gpu, Node, NodeStatus } from '../api/types';

// GPU 的占用程度。free 表示可以直接拿来用。
export type GpuLoad = 'free' | 'partial' | 'busy' | 'unknown';

const GPU_FREE_RATIO = 0.1;
const GPU_BUSY_RATIO = 0.8;

// gpuLoad 同时看利用率和显存：有人占着显存但没在算，也不算空闲。
export function gpuLoad(gpu: Gpu): GpuLoad {
  if (gpu.utilization_ratio === null || gpu.memory_used_bytes === null) return 'unknown';
  const memRatio = gpu.memory_total_bytes > 0 ? gpu.memory_used_bytes / gpu.memory_total_bytes : 0;
  const peak = Math.max(memRatio, gpu.utilization_ratio);
  if (peak < GPU_FREE_RATIO) return 'free';
  if (peak >= GPU_BUSY_RATIO) return 'busy';
  return 'partial';
}

export const GPU_LOAD_COLOR: Record<GpuLoad, string> = {
  free: 'teal',
  partial: 'yellow',
  busy: 'red',
  unknown: 'gray',
};

export function diskUsedRatio(disk: Disk): number {
  if (disk.total_bytes <= 0) return 0;
  return 1 - disk.available_bytes / disk.total_bytes;
}

const DISK_WARN_RATIO = 0.9;
const DISK_CRITICAL_RATIO = 0.95;

// diskColor 只在快满时着色，正常的磁盘用中性色。
export function diskColor(disk: Disk): string {
  const ratio = diskUsedRatio(disk);
  if (ratio >= DISK_CRITICAL_RATIO) return 'red';
  if (ratio >= DISK_WARN_RATIO) return 'yellow';
  return 'gray';
}

export function isDiskNearlyFull(disk: Disk): boolean {
  return diskUsedRatio(disk) >= DISK_WARN_RATIO;
}

// sortDisksByFree 按剩余空间从多到少排序，找地方放数据时一眼就能看到。
export function sortDisksByFree(disks: Disk[]): Disk[] {
  return [...disks].sort((a, b) => b.available_bytes - a.available_bytes);
}

const STATUS_ORDER: Record<NodeStatus, number> = { online: 0, lost: 1, offline: 2 };

// sortNodes 在线的排前面；同一状态内按 IP（没有 IP 时按主机名）自然排序。
export function sortNodes(nodes: Node[]): Node[] {
  return [...nodes].sort(
    (a, b) =>
      STATUS_ORDER[a.status] - STATUS_ORDER[b.status] ||
      nodeLabel(a).localeCompare(nodeLabel(b), undefined, { numeric: true }),
  );
}

export function nodeLabel(node: Node): string {
  return node.ip || node.hostname || node.node_id;
}

// countGpus 统计在线节点上的空闲 GPU 与全部 GPU。失联和离线节点的数据不可信，不计入。
export function countGpus(nodes: Node[]): { freeCount: number; totalCount: number } {
  let freeCount = 0;
  let totalCount = 0;
  for (const node of nodes) {
    if (node.status !== 'online') continue;
    for (const gpu of node.gpus) {
      totalCount++;
      if (gpuLoad(gpu) === 'free') freeCount++;
    }
  }
  return { freeCount, totalCount };
}
