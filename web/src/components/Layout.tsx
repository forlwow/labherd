import { AppShell, Container, Group, Text } from '@mantine/core';
import { Link, Outlet } from 'react-router';

export function Layout() {
  return (
    <AppShell header={{ height: 52 }} padding="md">
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between">
            <Text component={Link} to="/" fw={700} size="lg">
              LabHerd
            </Text>
          </Group>
        </Container>
      </AppShell.Header>
      <AppShell.Main>
        <Container size="xl" px={0}>
          <Outlet />
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
