import { useEffect, useState } from 'react';
import { Button, Card, ConfigProvider, Layout, Space, Statistic, Tag } from 'antd';
import { DashboardOutlined, LogoutOutlined, SafetyCertificateOutlined } from '@ant-design/icons';

import { useAccount } from '@/api/account';
import { useNetworkMapQuery } from '@/api/queries/useNetworkMapQuery';
import { useTheme } from '@/hooks/useTheme';
import { HttpUtil } from '@/utils';
import GlobalNetworkMap from './GlobalNetworkMap';
import './CustomerDashboard.css';

export default function CustomerDashboard() {
  const { account } = useAccount();
  const { data } = useNetworkMapQuery();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [clock, setClock] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setClock(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const online = data.nodes.filter((node) => node.status === 'online').length;
  const active = data.connections.reduce((sum, item) => sum + item.activeCount, 0);

  async function logout() {
    await HttpUtil.post('/logout');
    window.location.href = window.X_UI_BASE_PATH || '/';
  }

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={`index-page customer-dashboard ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`}>
        <aside className="customer-sidebar">
          <div className="customer-brand"><SafetyCertificateOutlined /> N</div>
          <DashboardOutlined className="customer-nav-active" />
          <Button type="text" icon={<LogoutOutlined />} aria-label="退出登录" onClick={logout} />
        </aside>
        <Layout.Content className="customer-content">
          <header className="dashboard-command-header">
            <div>
              <span className="dashboard-kicker">NOVAPANEL / PRIVATE NETWORK</span>
              <h1>网络控制中心</h1>
            </div>
            <div className="dashboard-live-meta">
              <time dateTime={clock.toISOString()}>{clock.toLocaleString('zh-CN', { hour12: false })}</time>
              <Tag color="cyan">{account?.username}</Tag>
            </div>
          </header>
          <div className="customer-stats">
            <Card><Statistic title="我的 VPS" value={data.nodes.length} /></Card>
            <Card><Statistic title="在线节点" value={online} /></Card>
            <Card><Statistic title="活跃连接" value={active} /></Card>
            <Card>
              <Space direction="vertical" size={2}>
                <span className="ant-statistic-title">定位状态</span>
                <strong>{data.geoReady ? '城市级定位已启用' : '等待 GeoIP 数据库'}</strong>
              </Space>
            </Card>
          </div>
          <GlobalNetworkMap />
        </Layout.Content>
      </Layout>
    </ConfigProvider>
  );
}
