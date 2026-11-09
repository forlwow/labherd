import { afterEach, describe, expect, it, vi } from 'vitest';
import { makeNode } from '../test/fixtures';
import { ApiError, fetchNode, fetchNodes } from './client';

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  vi.stubGlobal('fetch', fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchNodes', () => {
  it('returns the node array', async () => {
    const fn = mockFetch(200, { nodes: [makeNode()] });
    const nodes = await fetchNodes();
    expect(nodes).toHaveLength(1);
    expect(fn).toHaveBeenCalledWith('/api/v1/nodes', expect.anything());
  });
});

describe('fetchNode', () => {
  it('escapes the node id', async () => {
    const fn = mockFetch(200, makeNode());
    await fetchNode('a/b');
    expect(fn).toHaveBeenCalledWith('/api/v1/nodes/a%2Fb', expect.anything());
  });

  it('throws ApiError with the server message', async () => {
    mockFetch(404, { error: 'node not found' });
    const err = await fetchNode('x').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(404);
    expect((err as ApiError).message).toBe('node not found');
  });
});
