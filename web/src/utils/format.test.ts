import { describe, expect, it } from 'vitest';
import { formatAgo, formatBytes, formatGiB, formatPercent, shortGpuModel } from './format';

describe('formatBytes', () => {
  it.each([
    [0, '0B'],
    [512, '512B'],
    [1536, '1.5K'],
    [340 * 1024 ** 3, '340G'],
    [1.1 * 1024 ** 4, '1.1T'],
  ])('%d → %s', (bytes, want) => {
    expect(formatBytes(bytes)).toBe(want);
  });
});

describe('formatGiB', () => {
  it('drops trailing .0', () => {
    expect(formatGiB(24 * 1024 ** 3)).toBe('24');
    expect(formatGiB(22.1 * 1024 ** 3)).toBe('22.1');
  });
});

describe('formatPercent', () => {
  it('rounds and handles null', () => {
    expect(formatPercent(0.426)).toBe('43%');
    expect(formatPercent(null)).toBe('—');
  });
});

describe('formatAgo', () => {
  const now = Date.parse('2026-11-09T10:00:00Z');
  it.each([
    ['2026-11-09T09:59:55Z', '刚刚'],
    ['2026-11-09T09:59:30Z', '30 秒前'],
    ['2026-11-09T09:57:00Z', '3 分钟前'],
    ['2026-11-09T07:00:00Z', '3 小时前'],
    ['2026-11-07T10:00:00Z', '2 天前'],
  ])('%s → %s', (iso, want) => {
    expect(formatAgo(iso, now)).toBe(want);
  });
});

describe('shortGpuModel', () => {
  it('strips vendor prefixes', () => {
    expect(shortGpuModel('NVIDIA GeForce RTX 3090')).toBe('RTX 3090');
    expect(shortGpuModel('NVIDIA A100-SXM4-80GB')).toBe('A100-SXM4-80GB');
  });
});
