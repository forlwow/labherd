import { Badge, Group, Progress, Table, Text } from '@mantine/core';
import type { Gpu } from '../../api/types';
import { formatGiB, formatPercent } from '../../utils/format';
import { GPU_LOAD_COLOR, gpuLoad } from '../../utils/usage';

const LOAD_LABEL = { free: '空闲', partial: '部分', busy: '满载', unknown: '无数据' } as const;

function Bar({ ratio, color, label }: { ratio: number | null; color: string; label: string }) {
  return (
    <Group gap="xs" wrap="nowrap">
      <Progress value={(ratio ?? 0) * 100} color={color} size="sm" w={96} />
      <Text size="sm" ff="monospace">
        {label}
      </Text>
    </Group>
  );
}

// GpuTable 是详情页的 GPU 表格，比首页多出温度和功耗。
export function GpuTable({ gpus }: { gpus: Gpu[] }) {
  if (gpus.length === 0) {
    return <Text c="dimmed">没有检测到 GPU</Text>;
  }
  return (
    <Table.ScrollContainer minWidth={720}>
      <Table verticalSpacing="xs" highlightOnHover>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>#</Table.Th>
            <Table.Th>型号</Table.Th>
            <Table.Th>利用率</Table.Th>
            <Table.Th>显存</Table.Th>
            <Table.Th>温度</Table.Th>
            <Table.Th>功耗</Table.Th>
            <Table.Th>状态</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {gpus.map((g) => {
            const load = gpuLoad(g);
            const color = GPU_LOAD_COLOR[load];
            const memRatio =
              g.memory_used_bytes === null || g.memory_total_bytes === 0
                ? null
                : g.memory_used_bytes / g.memory_total_bytes;
            return (
              <Table.Tr key={g.uuid}>
                <Table.Td ff="monospace">{g.index}</Table.Td>
                <Table.Td>{g.model}</Table.Td>
                <Table.Td>
                  <Bar
                    ratio={g.utilization_ratio}
                    color={color}
                    label={formatPercent(g.utilization_ratio)}
                  />
                </Table.Td>
                <Table.Td>
                  <Bar
                    ratio={memRatio}
                    color={color}
                    label={`${g.memory_used_bytes === null ? '—' : formatGiB(g.memory_used_bytes)} / ${formatGiB(g.memory_total_bytes)} GB`}
                  />
                </Table.Td>
                <Table.Td ff="monospace">
                  {g.temperature_celsius === null ? '—' : `${Math.round(g.temperature_celsius)}°C`}
                </Table.Td>
                <Table.Td ff="monospace">
                  {g.power_watts === null ? '—' : `${Math.round(g.power_watts)} W`}
                </Table.Td>
                <Table.Td>
                  <Badge color={color} variant="light" size="sm" radius="sm">
                    {LOAD_LABEL[load]}
                  </Badge>
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}
