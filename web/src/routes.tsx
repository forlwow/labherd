import type { RouteObject } from 'react-router';
import { Layout } from './components/Layout';
import { NodeDetailPage } from './pages/node-detail/NodeDetailPage';
import { NodeListPage } from './pages/node-list/NodeListPage';
import { NotFoundPage } from './pages/not-found/NotFoundPage';

export const routes: RouteObject[] = [
  {
    element: <Layout />,
    children: [
      { index: true, element: <NodeListPage /> },
      { path: 'nodes/:nodeID', element: <NodeDetailPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];
