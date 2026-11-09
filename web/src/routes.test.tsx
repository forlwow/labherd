import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderRoute } from './test/render';

describe('routes', () => {
  it('renders the layout header', () => {
    renderRoute('/');
    expect(screen.getByText('LabHerd')).toBeInTheDocument();
  });

  it('shows not found for unknown paths', () => {
    renderRoute('/no-such-page');
    expect(screen.getByText('页面不存在')).toBeInTheDocument();
  });
});
