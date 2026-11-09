import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ApiError, fetchNode } from './client';
import { REFRESH_INTERVAL_MS } from './useNodeList';

// useNode 轮询单个节点。节点不存在（404）时不再重试。
export function useNode(nodeId: string) {
  return useQuery({
    queryKey: ['nodes', nodeId],
    queryFn: ({ signal }) => fetchNode(nodeId, signal),
    refetchInterval: (query) =>
      query.state.error instanceof ApiError && query.state.error.status === 404
        ? false
        : REFRESH_INTERVAL_MS,
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 3,
    placeholderData: keepPreviousData,
  });
}
