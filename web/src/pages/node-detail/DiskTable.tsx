import { Group, Progress, Table, Text } from '@mantine/core';
import type { Disk } from '../../api/types';
import { formatBytes, formatPercent } from '../../utils/format';
import { diskColor, diskUsedRatio, sortDisksByFree } from '../../utils/usage';

// DiskTable 列出全部磁盘，剩余空间多的在前。
export function DiskTable({ disks }: { disks: Disk[] }) {
  if (disks.length === 0) {
    return <Text c="dimmed">暂无磁盘数据</Text>;
  }
  return (
    <Table.ScrollContainer minWidth={560}>
      <Table verticalSpacing="xs" highlightOnHover>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>挂载点</Table.Th>
            <Table.Th ta="right">剩余</Table.Th>
            <Table.Th ta="right">总量</Table.Th>
            <Table.Th>使用率</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {sortDisksByFree(disks).map((d) => {
            const ratio = diskUsedRatio(d);
            const color = diskColor(d);
            return (
              <Table.Tr key={d.mount_point}>
                <Table.Td ff="monospace">{d.mount_point}</Table.Td>
                <Table.Td ff="monospace" ta="right" c={color === 'gray.6' ? undefined : color}>
                  {formatBytes(d.available_bytes)}
                </Table.Td>
                <Table.Td ff="monospace" ta="right" c="dimmed">
                  {formatBytes(d.total_bytes)}
                </Table.Td>
                <Table.Td>
                  <Group gap="xs" wrap="nowrap">
                    <Progress value={ratio * 100} color={color} size="sm" w={160} />
                    <Text size="sm" ff="monospace">
                      {formatPercent(ratio)}
                    </Text>
                  </Group>
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}
