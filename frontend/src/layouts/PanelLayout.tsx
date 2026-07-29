import { Spin } from 'antd';
import { Navigate, Outlet, useLocation } from 'react-router';

import { AccountProvider, useAccount } from '@/api/account';
import { useWebSocketBridge } from '@/api/websocketBridge';
import { usePageTitle } from '@/hooks/usePageTitle';

const CUSTOMER_ROUTES = new Set([
  '/',
  '/inbounds',
  '/clients',
  '/groups',
  '/hosts',
  '/nodes',
]);

function AdminPanel() {
  useWebSocketBridge();
  usePageTitle();
  return <Outlet />;
}

function PanelAccess() {
  const { account, loading } = useAccount();
  const location = useLocation();
  if (loading) return <Spin fullscreen />;
  if (account?.role === 'customer') {
    const pathname = location.pathname.replace(/\/+$/, '') || '/';
    if (!CUSTOMER_ROUTES.has(pathname)) return <Navigate to="/" replace />;
    return <Outlet />;
  }
  return <AdminPanel />;
}

export default function PanelLayout() {
  return (
    <AccountProvider>
      <PanelAccess />
    </AccountProvider>
  );
}
