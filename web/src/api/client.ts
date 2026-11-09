import type { Node, NodeListResponse } from './types';

// ApiError 表示 Controller 返回了非 2xx 的响应。
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const resp = await fetch(path, { signal, headers: { Accept: 'application/json' } });
  if (!resp.ok) {
    let message = `${resp.status} ${resp.statusText}`;
    try {
      const body = (await resp.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // 响应体不是 JSON 时沿用状态行。
    }
    throw new ApiError(resp.status, message);
  }
  return (await resp.json()) as T;
}

export async function fetchNodes(signal?: AbortSignal): Promise<Node[]> {
  const body = await getJSON<NodeListResponse>('/api/v1/nodes', signal);
  return body.nodes;
}

export function fetchNode(nodeId: string, signal?: AbortSignal): Promise<Node> {
  return getJSON<Node>(`/api/v1/nodes/${encodeURIComponent(nodeId)}`, signal);
}
