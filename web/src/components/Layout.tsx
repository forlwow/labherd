import { AppShell, Container, Group, Text } from '@mantine/core';
import { Link, Outlet } from 'react-router';
import { ColorSchemeToggle } from './ColorSchemeToggle';
import classes from './Layout.module.css';

export function Layout() {
  return (
    <AppShell header={{ height: 52 }} padding="md">
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between">
            <Text component={Link} to="/" fw={700} size="lg" c="bright">
              LabHerd
            </Text>
            <ColorSchemeToggle />
          </Group>
        </Container>
      </AppShell.Header>
      <AppShell.Main className={classes.main}>
        <Container size="xl" px={0}>
          <Outlet />
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
