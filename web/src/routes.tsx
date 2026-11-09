import type { RouteObject } from 'react-router';
import { Layout } from './components/Layout';
import { NodeListPage } from './pages/node-list/NodeListPage';
import { NotFoundPage } from './pages/not-found/NotFoundPage';

export const routes: RouteObject[] = [
  {
    element: <Layout />,
    children: [
      { index: true, element: <NodeListPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];
