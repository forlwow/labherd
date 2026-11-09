import { Anchor, Stack, Text, Title } from '@mantine/core';
import { Link } from 'react-router';

export function NotFoundPage() {
  return (
    <Stack align="center" mt="xl" gap="xs">
      <Title order={2}>页面不存在</Title>
      <Text c="dimmed">地址可能拼错了，或者页面已经移走。</Text>
      <Anchor component={Link} to="/">
        回到首页
      </Anchor>
    </Stack>
  );
}
