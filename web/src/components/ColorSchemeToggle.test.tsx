import { MantineProvider } from '@mantine/core';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { ColorSchemeToggle } from './ColorSchemeToggle';

function renderToggle() {
  return render(
    <MantineProvider defaultColorScheme="dark">
      <ColorSchemeToggle />
    </MantineProvider>,
  );
}

afterEach(() => {
  localStorage.clear();
});

describe('ColorSchemeToggle', () => {
  it('switches between light and dark and remembers the choice', async () => {
    renderToggle();
    await userEvent.click(screen.getByRole('button', { name: '切换到白天配色' }));
    expect(localStorage.getItem('mantine-color-scheme-value')).toBe('light');

    await userEvent.click(screen.getByRole('button', { name: '切换到夜间配色' }));
    expect(localStorage.getItem('mantine-color-scheme-value')).toBe('dark');
  });
});
