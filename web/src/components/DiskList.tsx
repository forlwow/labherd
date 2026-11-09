import { Button, Collapse, Group, Progress, SimpleGrid, Stack, Text, Tooltip } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import type { Disk } from '../api/types';
import { formatBytes, formatPercent } from '../utils/format';
import { diskColor, diskUsedRatio, sortDisksByFree } from '../utils/usage';

// 默认显示的磁盘块数，其余折叠。
export const VISIBLE_DISK_COUNT = 4;

function DiskItem({ disk }: { disk: Disk }) {
  const ratio = diskUsedRatio(disk);
  const color = diskColor(disk);
  return (
    <Stack gap={3} data-testid="disk-item">
      <Group justify="space-between" gap="xs" wrap="nowrap">
        <Tooltip label={disk.mount_point} openDelay={400}>
          <Text size="xs" ff="monospace" truncate>
            {disk.mount_point}
          </Text>
        </Tooltip>
        <Text
          size="xs"
          ff="monospace"
          c={color === 'gray.6' ? 'dimmed' : color}
          style={{ flexShrink: 0 }}
        >
          剩 {formatBytes(disk.available_bytes)} · {formatPercent(ratio)}
        </Text>
      </Group>
      <Progress value={ratio * 100} color={color} size="xs" />
    </Stack>
  );
}

// DiskList 以两列显示磁盘，剩余空间多的在前；超过 VISIBLE_DISK_COUNT 块时折叠其余。
export function DiskList({ disks }: { disks: Disk[] }) {
  const [isExpanded, { toggle }] = useDisclosure(false);
  if (disks.length === 0) {
    return (
      <Text size="xs" c="dimmed">
        暂无磁盘数据
      </Text>
    );
  }

  const sorted = sortDisksByFree(disks);
  const visible = sorted.slice(0, VISIBLE_DISK_COUNT);
  const hidden = sorted.slice(VISIBLE_DISK_COUNT);
  const grid = (items: Disk[]) => (
    <SimpleGrid cols={{ base: 1, xs: 2 }} spacing="md" verticalSpacing="xs">
      {items.map((d) => (
        <DiskItem key={d.mount_point} disk={d} />
      ))}
    </SimpleGrid>
  );

  return (
    <Stack gap="xs">
      {grid(visible)}
      {hidden.length > 0 && (
        <>
          <Collapse expanded={isExpanded}>{grid(hidden)}</Collapse>
          <Button variant="subtle" size="compact-xs" color="gray" onClick={toggle} w="fit-content">
            {isExpanded ? '收起' : `展开其余 ${hidden.length} 块`}
          </Button>
        </>
      )}
    </Stack>
  );
}
