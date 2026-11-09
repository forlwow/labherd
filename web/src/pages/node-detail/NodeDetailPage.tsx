import {
  Alert,
  Anchor,
  Group,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import type { ReactNode } from 'react';
import { Link, useParams } from 'react-router';
import { ApiError } from '../../api/client';
import type { Node } from '../../api/types';
import { useNode } from '../../api/useNode';
import { StatusBadge } from '../../components/StatusBadge';
import { formatAgo, formatBytes, formatClock, formatPercent } from '../../utils/format';
import { DiskTable } from './DiskTable';
import { GpuTable } from './GpuTable';

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Paper withBorder p="md">
      <Stack gap="sm">
        <Title order={4}>{title}</Title>
        {children}
      </Stack>
    </Paper>
  );
}

function Field({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text size="sm" ff="monospace">
        {value || '—'}
      </Text>
    </div>
  );
}

function HostInfo({ node }: { node: Node }) {
  const { host } = node;
  const memory =
    host.memory_used_bytes === null
      ? formatBytes(host.memory_total_bytes)
      : `${formatBytes(host.memory_used_bytes)} / ${formatBytes(host.memory_total_bytes)}`;
  return (
    <SimpleGrid cols={{ base: 2, sm: 3, lg: 4 }} spacing="md" verticalSpacing="sm">
      <Field label="系统" value={host.os} />
      <Field label="内核" value={host.kernel_version} />
      <Field label="CPU" value={host.cpu_model} />
      <Field label="逻辑核数" value={host.cpu_logical_cores} />
      <Field label="CPU 利用率" value={formatPercent(host.cpu_utilization_ratio)} />
      <Field label="1 分钟负载" value={host.load1?.toFixed(2)} />
      <Field label="内存" value={memory} />
      <Field label="GPU 驱动" value={host.gpu_driver_version} />
      <Field label="Agent 版本" value={node.agent_version} />
      <Field label="节点 ID" value={node.node_id} />
      <Field label="连接时间" value={new Date(node.connected_at).toLocaleString('zh-CN')} />
      <Field label="最后消息" value={formatAgo(node.last_seen_at)} />
    </SimpleGrid>
  );
}

// NodeDetailPage 是 /nodes/:nodeID：主机信息、GPU 表格、全部磁盘。
export function NodeDetailPage() {
  const { nodeID = '' } = useParams();
  const { data: node, error, isPending, isError, dataUpdatedAt } = useNode(nodeID);

  const back = (
    <Anchor component={Link} to="/" size="sm">
      ← 全部节点
    </Anchor>
  );

  if (isPending) {
    return (
      <Stack>
        {back}
        <Skeleton h={160} radius="md" />
        <Skeleton h={240} radius="md" />
      </Stack>
    );
  }

  if (!node || (error instanceof ApiError && error.status === 404)) {
    const isNotFound = error instanceof ApiError && error.status === 404;
    return (
      <Stack>
        {back}
        <Alert
          color={isNotFound ? 'gray' : 'red'}
          title={isNotFound ? '节点不存在' : '无法连接 Controller'}
        >
          {isNotFound ? `没有 ID 为 ${nodeID} 的节点。` : error?.message}
        </Alert>
      </Stack>
    );
  }

  return (
    <Stack gap="md">
      {back}
      <Group justify="space-between" align="flex-end">
        <Group gap="sm" align="baseline">
          <Title order={2} ff="monospace">
            {node.ip || node.hostname}
          </Title>
          {node.ip && (
            <Text c="dimmed" size="lg">
              {node.hostname}
            </Text>
          )}
          <StatusBadge node={node} />
        </Group>
        <Text size="xs" c={isError ? 'red' : 'dimmed'}>
          {isError
            ? `与服务器断开，显示的是 ${formatClock(dataUpdatedAt)} 的数据`
            : `更新于 ${formatClock(dataUpdatedAt)}`}
        </Text>
      </Group>
      <Section title="主机">
        <HostInfo node={node} />
      </Section>
      <Section title={`GPU（${node.gpus.length}）`}>
        <GpuTable gpus={node.gpus} />
      </Section>
      <Section title={`磁盘（${node.host.disks.length}）`}>
        <DiskTable disks={node.host.disks} />
      </Section>
    </Stack>
  );
}
