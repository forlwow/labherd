import { Badge, Progress, Text, Tooltip } from '@mantine/core';
import type { Gpu } from '../api/types';
import { formatGiB, formatPercent, shortGpuModel } from '../utils/format';
import { GPU_LOAD_COLOR, gpuLoad } from '../utils/usage';
import classes from './GpuRow.module.css';

const LOAD_LABEL = { free: '空闲', partial: '部分', busy: '满载', unknown: '无数据' } as const;

// GpuRow 是节点卡片中一张 GPU 的一行：利用率、显存、空闲标记。
export function GpuRow({ gpu }: { gpu: Gpu }) {
  const load = gpuLoad(gpu);
  const color = GPU_LOAD_COLOR[load];
  const memRatio =
    gpu.memory_used_bytes !== null && gpu.memory_total_bytes > 0
      ? gpu.memory_used_bytes / gpu.memory_total_bytes
      : null;

  return (
    <div className={classes.row} data-testid="gpu-row">
      <Text ff="monospace" size="sm" c="dimmed">
        #{gpu.index}
      </Text>
      <Tooltip label={gpu.model} openDelay={400}>
        <Text size="sm" truncate>
          {shortGpuModel(gpu.model)}
        </Text>
      </Tooltip>
      <div className={classes.metric}>
        <Text size="xs" ff="monospace">
          {formatPercent(gpu.utilization_ratio)}
        </Text>
        <Progress value={(gpu.utilization_ratio ?? 0) * 100} color={color} size="sm" />
      </div>
      <div className={classes.metric}>
        <Text size="xs" ff="monospace">
          {gpu.memory_used_bytes === null ? '—' : formatGiB(gpu.memory_used_bytes)} /{' '}
          {formatGiB(gpu.memory_total_bytes)} GB
        </Text>
        <Progress value={(memRatio ?? 0) * 100} color={color} size="sm" />
      </div>
      <Badge className={classes.badge} color={color} variant="light" size="sm" radius="sm">
        {LOAD_LABEL[load]}
      </Badge>
    </div>
  );
}
