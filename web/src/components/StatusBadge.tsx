import { Badge } from '@mantine/core';
import type { Node } from '../api/types';
import { formatAgo } from '../utils/format';

const LABEL = { online: '在线', lost: '失联', offline: '离线' } as const;
const COLOR = { online: 'teal', lost: 'yellow', offline: 'gray' } as const;

// StatusBadge 显示节点状态；失联和离线时附上最后一次收到消息的时间。
export function StatusBadge({ node }: { node: Node }) {
  const since = node.status === 'online' ? '' : ` · ${formatAgo(node.last_seen_at)}`;
  return (
    <Badge
      color={COLOR[node.status]}
      variant="light"
      size="sm"
      radius="sm"
      style={{ flexShrink: 0 }}
    >
      {LABEL[node.status]}
      {since}
    </Badge>
  );
}
