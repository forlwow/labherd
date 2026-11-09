import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { makeNode } from '../../test/fixtures';
import { renderRoute } from '../../test/render';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('NodeDetailPage', () => {
  it('shows host info, gpus and all disks', async () => {
    const fetchMock = vi.fn().mockResolvedValue(Response.json(makeNode()));
    vi.stubGlobal('fetch', fetchMock);
    renderRoute('/nodes/gpu-01');

    expect(await screen.findByRole('heading', { name: '10.10.3.21' })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/nodes/gpu-01', expect.anything());
    expect(screen.getByText('Intel(R) Xeon(R) Gold 6248R')).toBeInTheDocument();
    expect(screen.getByText('GPU（3）')).toBeInTheDocument();
    expect(screen.getByText('磁盘（6）')).toBeInTheDocument();
    // 全部磁盘都列出，剩余最多的 /data3 排第一行。
    const rows = screen.getAllByRole('row');
    expect(rows.some((r) => r.textContent?.startsWith('/data3'))).toBe(true);
  });

  it('shows not found for unknown nodes', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(Response.json({ error: 'node not found' }, { status: 404 })),
    );
    renderRoute('/nodes/nope');
    expect(await screen.findByText('节点不存在')).toBeInTheDocument();
  });
});
