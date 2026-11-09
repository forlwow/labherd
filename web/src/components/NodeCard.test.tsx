import { MantineProvider } from '@mantine/core';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { makeDisk, makeNode } from '../test/fixtures';
import { NodeCard } from './NodeCard';

function renderCard(node = makeNode()) {
  return render(
    <MantineProvider>
      <NodeCard node={node} />
    </MantineProvider>,
  );
}

describe('NodeCard', () => {
  it('shows ip as title with hostname beside it', () => {
    renderCard();
    expect(screen.getByText('10.10.3.21')).toBeInTheDocument();
    expect(screen.getByText('gpu-01')).toBeInTheDocument();
  });

  it('falls back to hostname without ip', () => {
    renderCard(makeNode({ ip: undefined }));
    expect(screen.getByText('gpu-01')).toBeInTheDocument();
    expect(screen.queryByText('10.10.3.21')).not.toBeInTheDocument();
  });

  it('renders one row per gpu', () => {
    renderCard();
    const rows = screen.getAllByTestId('gpu-row');
    expect(rows).toHaveLength(3);
    expect(within(rows[2]!).getByText('空闲')).toBeInTheDocument();
  });

  it('shows the four disks with most free space and folds the rest', async () => {
    renderCard();
    expect(screen.getAllByTestId('disk-item')[0]).toHaveTextContent('/data3');
    expect(screen.getByText('展开其余 2 块')).toBeInTheDocument();
    await userEvent.click(screen.getByText('展开其余 2 块'));
    expect(screen.getByText('收起')).toBeInTheDocument();
  });

  it('flags nearly full disks', () => {
    renderCard();
    expect(screen.getByText('1 块将满')).toBeInTheDocument();
  });

  it('does not fold four or fewer disks', () => {
    const node = makeNode();
    node.host.disks = [makeDisk('/', 1, 600), makeDisk('/data', 4, 100)];
    renderCard(node);
    expect(screen.queryByText(/展开其余/)).not.toBeInTheDocument();
  });

  it('shows how long a lost node has been silent', () => {
    renderCard(
      makeNode({ status: 'lost', last_seen_at: new Date(Date.now() - 180_000).toISOString() }),
    );
    expect(screen.getByText(/失联 · 3 分钟前/)).toBeInTheDocument();
  });
});
