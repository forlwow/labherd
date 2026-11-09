import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { makeGpu, makeNode } from '../../test/fixtures';
import { renderRoute } from '../../test/render';

function stubNodes(nodes: unknown[]) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation(() => Promise.resolve(Response.json({ nodes }))),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('NodeListPage', () => {
  it('lists online nodes before lost ones and counts free gpus', async () => {
    stubNodes([
      makeNode({ node_id: 'b', ip: '10.0.0.9', status: 'lost' }),
      makeNode({ node_id: 'a', ip: '10.0.0.10', gpus: [makeGpu(0, 0, 0), makeGpu(1, 0.9, 20)] }),
    ]);
    renderRoute('/');
    const cards = await screen.findAllByTestId('node-card');
    expect(cards[0]).toHaveTextContent('10.0.0.10');
    expect(cards[1]).toHaveTextContent('10.0.0.9');
    // 失联节点的 GPU 不计入：在线节点 2 张卡，1 张空闲。
    expect(screen.getByText('空闲 GPU').nextSibling).toHaveTextContent('1 / 2');
  });

  it('shows an empty state', async () => {
    stubNodes([]);
    renderRoute('/');
    expect(await screen.findByText(/还没有节点接入/)).toBeInTheDocument();
  });

  it('shows an error when the first request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));
    renderRoute('/');
    expect(await screen.findByText('无法连接 Controller')).toBeInTheDocument();
  });
});
