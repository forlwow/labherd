import { Alert, SimpleGrid, Skeleton, Stack, Text } from '@mantine/core';
import { useNodeList } from '../../api/useNodeList';
import { ListSummary } from '../../components/ListSummary';
import { NodeCard } from '../../components/NodeCard';
import { sortNodes } from '../../utils/usage';

// NodeListPage 是首页：汇总栏加节点卡片网格。
export function NodeListPage() {
  const { data, error, isPending, isError, dataUpdatedAt } = useNodeList();

  if (isPending) {
    return (
      <SimpleGrid cols={{ base: 1, lg: 2 }}>
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} h={320} radius="md" />
        ))}
      </SimpleGrid>
    );
  }

  if (!data) {
    return (
      <Alert color="red" title="无法连接 Controller">
        {error?.message}
      </Alert>
    );
  }

  return (
    <Stack gap="md">
      <ListSummary nodes={data} updatedAtMs={dataUpdatedAt} isStale={isError} />
      {data.length === 0 ? (
        <Text c="dimmed" ta="center" mt="xl">
          还没有节点接入。在 GPU 服务器上启动 labherd-agent 后，节点会出现在这里。
        </Text>
      ) : (
        <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md" verticalSpacing="md">
          {sortNodes(data).map((n) => (
            <NodeCard key={n.node_id} node={n} />
          ))}
        </SimpleGrid>
      )}
    </Stack>
  );
}
