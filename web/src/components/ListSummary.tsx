import { Group, Paper, Text } from '@mantine/core';
import type { ReactNode } from 'react';
import type { Node } from '../api/types';
import { formatClock } from '../utils/format';
import { countGpus } from '../utils/usage';

interface ListSummaryProps {
  nodes: Node[];
  // 最近一次成功拉取的时刻。
  updatedAtMs: number;
  // 最近一次拉取失败，页面显示的是旧数据。
  isStale: boolean;
}

function Stat({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text ff="monospace" fw={700} size="xl" lh={1.2}>
        {children}
      </Text>
    </div>
  );
}

// ListSummary 是首页顶部的汇总栏：空闲 GPU 数、各状态节点数、数据是否最新。
export function ListSummary({ nodes, updatedAtMs, isStale }: ListSummaryProps) {
  const { freeCount, totalCount } = countGpus(nodes);
  const count = (s: Node['status']) => nodes.filter((n) => n.status === s).length;

  return (
    <Paper withBorder p="md">
      <Group justify="space-between" align="flex-end">
        <Group gap="xl">
          <Stat label="空闲 GPU">
            <Text span inherit c={freeCount > 0 ? 'teal' : 'red'}>
              {freeCount}
            </Text>
            <Text span inherit c="dimmed" fw={400}>
              {' '}
              / {totalCount}
            </Text>
          </Stat>
          <Stat label="在线">{count('online')}</Stat>
          <Stat label="失联">
            <Text span inherit c={count('lost') > 0 ? 'yellow' : undefined}>
              {count('lost')}
            </Text>
          </Stat>
          <Stat label="离线">{count('offline')}</Stat>
        </Group>
        <Text size="xs" c={isStale ? 'red' : 'dimmed'}>
          {isStale
            ? `与服务器断开，显示的是 ${formatClock(updatedAtMs)} 的数据`
            : `更新于 ${formatClock(updatedAtMs)}`}
        </Text>
      </Group>
    </Paper>
  );
}
