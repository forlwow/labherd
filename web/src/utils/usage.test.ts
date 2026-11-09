import { describe, expect, it } from 'vitest';
import { makeDisk, makeGpu, makeNode } from '../test/fixtures';
import { countGpus, diskColor, gpuLoad, sortDisksByFree, sortNodes } from './usage';

describe('gpuLoad', () => {
  it('classifies by the higher of utilization and memory', () => {
    expect(gpuLoad(makeGpu(0, 0, 0.3))).toBe('free');
    expect(gpuLoad(makeGpu(0, 0.31, 10.4))).toBe('partial');
    expect(gpuLoad(makeGpu(0, 0.98, 22))).toBe('busy');
  });

  it('treats idle gpu holding memory as busy', () => {
    expect(gpuLoad(makeGpu(0, 0, 23))).toBe('busy');
  });

  it('is unknown without samples', () => {
    expect(gpuLoad(makeGpu(0, null, null))).toBe('unknown');
  });
});

describe('disks', () => {
  it('sorts by available space, most first', () => {
    const sorted = sortDisksByFree([
      makeDisk('/a', 1, 10),
      makeDisk('/b', 1, 500),
      makeDisk('/c', 8, 100),
    ]);
    expect(sorted.map((d) => d.mount_point)).toEqual(['/b', '/c', '/a']);
  });

  it('colors only nearly full disks', () => {
    expect(diskColor(makeDisk('/', 1, 600))).toBe('gray.6');
    expect(diskColor(makeDisk('/', 1, 80))).toBe('yellow');
    expect(diskColor(makeDisk('/', 1, 20))).toBe('red');
  });
});

describe('sortNodes', () => {
  it('puts online first and orders ips numerically', () => {
    const nodes = [
      makeNode({ node_id: 'a', ip: '10.0.0.10', status: 'offline' }),
      makeNode({ node_id: 'b', ip: '10.0.0.10' }),
      makeNode({ node_id: 'c', ip: '10.0.0.9' }),
      makeNode({ node_id: 'd', ip: '10.0.0.2', status: 'lost' }),
    ];
    expect(sortNodes(nodes).map((n) => n.node_id)).toEqual(['c', 'b', 'd', 'a']);
  });
});

describe('countGpus', () => {
  it('counts only online nodes', () => {
    const nodes = [makeNode(), makeNode({ status: 'lost' })];
    expect(countGpus(nodes)).toEqual({ freeCount: 1, totalCount: 3 });
  });
});
