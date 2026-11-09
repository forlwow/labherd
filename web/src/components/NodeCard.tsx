import { Anchor, Badge, Card, Divider, Group, Stack, Text } from '@mantine/core';
import { Link } from 'react-router';
import type { Node } from '../api/types';
import { formatBytes, formatPercent } from '../utils/format';
import { isDiskNearlyFull } from '../utils/usage';
import { DiskList } from './DiskList';
import { GpuRow } from './GpuRow';
import { StatusBadge } from './StatusBadge';

function SectionLabel({ children }: { children: string }) {
  return (
    <Text size="xs" c="dimmed" fw={600} tt="uppercase">
      {children}
    </Text>
  );
}

// NodeCard 是首页的一张节点卡片：标题（IP + 主机名）、主机概况、每张 GPU 一行、磁盘列表。
export function NodeCard({ node }: { node: Node }) {
  const { host } = node;
  const isOnline = node.status === 'online';
  const nearlyFullCount = host.disks.filter(isDiskNearlyFull).length;

  return (
    <Card withBorder padding="md" style={{ opacity: isOnline ? 1 : 0.55 }} data-testid="node-card">
      <Stack gap="sm">
        <Group justify="space-between" gap="xs">
          <Group gap="xs" wrap="nowrap" style={{ minWidth: 0 }}>
            <Anchor
              component={Link}
              to={`/nodes/${encodeURIComponent(node.node_id)}`}
              ff="monospace"
              fw={700}
              size="lg"
              c="bright"
              style={{ flexShrink: 0 }}
            >
              {node.ip || node.hostname}
            </Anchor>
            {node.ip && (
              <Text c="dimmed" size="sm" truncate>
                {node.hostname}
              </Text>
            )}
            <StatusBadge node={node} />
          </Group>
          <Group gap="md" wrap="nowrap" style={{ flexShrink: 0 }}>
            <Text size="xs" c="dimmed">
              CPU{' '}
              <Text span ff="monospace" c="bright">
                {formatPercent(host.cpu_utilization_ratio)}
              </Text>
            </Text>
            <Text size="xs" c="dimmed">
              内存{' '}
              <Text span ff="monospace" c="bright">
                {host.memory_used_bytes === null ? '—' : formatBytes(host.memory_used_bytes)} /{' '}
                {formatBytes(host.memory_total_bytes)}
              </Text>
            </Text>
          </Group>
        </Group>

        <Stack gap={6}>
          <SectionLabel>GPU</SectionLabel>
          {node.gpus.length === 0 ? (
            <Text size="xs" c="dimmed">
              没有检测到 GPU
            </Text>
          ) : (
            node.gpus.map((g) => <GpuRow key={g.uuid} gpu={g} />)
          )}
        </Stack>

        <Divider />

        <Stack gap={6}>
          <Group justify="space-between">
            <SectionLabel>磁盘</SectionLabel>
            {nearlyFullCount > 0 && (
              <Badge color="red" variant="light" size="xs" radius="sm">
                {nearlyFullCount} 块将满
              </Badge>
            )}
          </Group>
          <DiskList disks={host.disks} />
        </Stack>
      </Stack>
    </Card>
  );
}
