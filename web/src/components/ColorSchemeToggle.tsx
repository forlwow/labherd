import { ActionIcon, Tooltip, useComputedColorScheme, useMantineColorScheme } from '@mantine/core';

function SunIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      aria-hidden
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  );
}

// ColorSchemeToggle 在白天与夜间配色之间切换。
// 未手动切换时跟随系统设置；切换后的选择保存在浏览器中，下次打开保持不变。
export function ColorSchemeToggle() {
  const { setColorScheme } = useMantineColorScheme();
  const computed = useComputedColorScheme('dark', { getInitialValueInEffect: false });
  const isDark = computed === 'dark';
  const label = isDark ? '切换到白天配色' : '切换到夜间配色';

  return (
    <Tooltip label={label} openDelay={400}>
      <ActionIcon
        variant="subtle"
        color="gray"
        size="lg"
        aria-label={label}
        onClick={() => setColorScheme(isDark ? 'light' : 'dark')}
      >
        {isDark ? <SunIcon /> : <MoonIcon />}
      </ActionIcon>
    </Tooltip>
  );
}
