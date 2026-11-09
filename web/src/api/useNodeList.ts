import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { fetchNodes } from './client';

export const REFRESH_INTERVAL_MS = 5000;

// useNodeList 每隔 REFRESH_INTERVAL_MS 拉取一次节点列表。
// 请求失败时保留上一次成功的数据，页面据此提示"数据已过期"而不是清空。
export function useNodeList() {
  return useQuery({
    queryKey: ['nodes'],
    queryFn: ({ signal }) => fetchNodes(signal),
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
    placeholderData: keepPreviousData,
  });
}
