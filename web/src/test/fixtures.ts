import type { Disk, Gpu, Node } from '../api/types';

const GIB = 1024 ** 3;
const TIB = 1024 * GIB;

export function makeGpu(index: number, utilization: number | null, usedGiB: number | null): Gpu {
  return {
    uuid: `GPU-${index}`,
    index,
    model: 'NVIDIA GeForce RTX 3090',
    memory_total_bytes: 24 * GIB,
    utilization_ratio: utilization,
    memory_used_bytes: usedGiB === null ? null : usedGiB * GIB,
    temperature_celsius: utilization === null ? null : 40 + utilization * 40,
    power_watts: utilization === null ? null : 30 + utilization * 300,
  };
}

export function makeDisk(mountPoint: string, totalTiB: number, availableGiB: number): Disk {
  return {
    mount_point: mountPoint,
    total_bytes: totalTiB * TIB,
    available_bytes: availableGiB * GIB,
  };
}

export function makeNode(overrides: Partial<Node> = {}): Node {
  return {
    node_id: 'gpu-01',
    instance_id: 'inst-1',
    hostname: 'gpu-01',
    ip: '10.10.3.21',
    agent_version: 'v0.1.0',
    status: 'online',
    connected_at: '2026-11-09T08:00:00Z',
    last_seen_at: '2026-11-09T09:00:00Z',
    disconnected_at: null,
    sampled_at: '2026-11-09T09:00:00Z',
    host: {
      os: 'Ubuntu 22.04.4 LTS',
      kernel_version: '5.15.0-105-generic',
      cpu_model: 'Intel(R) Xeon(R) Gold 6248R',
      cpu_logical_cores: 96,
      memory_total_bytes: 256 * GIB,
      gpu_driver_version: '550.54.15',
      cpu_utilization_ratio: 0.42,
      load1: 12.5,
      memory_available_bytes: 100 * GIB,
      memory_used_bytes: 156 * GIB,
      disks: [
        makeDisk('/', 1, 600),
        makeDisk('/home', 2, 340),
        makeDisk('/data1', 8, 1126),
        makeDisk('/data2', 4, 38),
        makeDisk('/data3', 8, 4000),
        makeDisk('/data4', 4, 2000),
      ],
    },
    gpus: [makeGpu(0, 0.98, 22.1), makeGpu(1, 0.31, 10.4), makeGpu(2, 0, 0.3)],
    ...overrides,
  };
}
