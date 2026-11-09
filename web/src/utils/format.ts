const UNITS = ['B', 'K', 'M', 'G', 'T', 'P'];

// formatBytes 以 1024 为底输出紧凑的容量，如 1.1T、340G、512M。
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0B';
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024;
    unit++;
  }
  const digits = value >= 100 || unit === 0 ? 0 : 1;
  return `${value.toFixed(digits)}${UNITS[unit]}`;
}

// formatGiB 把字节数换算为 GB 数值字符串，用于"22.1 / 24 GB"这样的显存显示。
export function formatGiB(bytes: number): string {
  const gib = bytes / 1024 ** 3;
  return gib >= 100 ? gib.toFixed(0) : gib.toFixed(1).replace(/\.0$/, '');
}

export function formatPercent(ratio: number | null): string {
  if (ratio === null) return '—';
  return `${Math.round(ratio * 100)}%`;
}

// formatAgo 输出"刚刚""3 分钟前"这样的相对时间。
export function formatAgo(iso: string, nowMs: number = Date.now()): string {
  const diffSec = Math.max(0, Math.floor((nowMs - new Date(iso).getTime()) / 1000));
  if (diffSec < 10) return '刚刚';
  if (diffSec < 60) return `${diffSec} 秒前`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} 分钟前`;
  const diffHour = Math.floor(diffMin / 60);
  if (diffHour < 24) return `${diffHour} 小时前`;
  return `${Math.floor(diffHour / 24)} 天前`;
}

export function formatClock(ms: number): string {
  return new Date(ms).toLocaleTimeString('zh-CN', { hour12: false });
}

// shortGpuModel 去掉厂商前缀，如 "NVIDIA GeForce RTX 3090" → "RTX 3090"。
export function shortGpuModel(model: string): string {
  return model.replace(/^NVIDIA\s+/i, '').replace(/^GeForce\s+/i, '');
}
