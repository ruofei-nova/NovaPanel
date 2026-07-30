import { useCallback, useEffect, useState } from 'react';
import { EnvironmentOutlined, LoadingOutlined } from '@ant-design/icons';
import { Button, Card, ConfigProvider, Layout, Space, Statistic, Tag } from 'antd';

import { useAccount } from '@/api/account';
import { useNetworkMapQuery } from '@/api/queries/useNetworkMapQuery';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import GlobalNetworkMap from './GlobalNetworkMap';
import './CustomerDashboard.css';

type LocationState = 'idle' | 'locating' | 'granted' | 'denied' | 'unsupported' | 'insecure' | 'error';

export default function CustomerDashboard() {
  const { account } = useAccount();
  const { data, refresh } = useNetworkMapQuery();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [clock, setClock] = useState(() => new Date());
  const [locationState, setLocationState] = useState<LocationState>('idle');

  useEffect(() => {
    const timer = window.setInterval(() => setClock(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const requestPreciseLocation = useCallback(() => {
    if (!window.isSecureContext) {
      setLocationState('insecure');
      return;
    }
    if (!navigator.geolocation) {
      setLocationState('unsupported');
      return;
    }
    setLocationState('locating');
    navigator.geolocation.getCurrentPosition(
      async (position) => {
        const msg = await HttpUtil.post('/panel/api/network/location', {
          latitude: position.coords.latitude,
          longitude: position.coords.longitude,
          accuracyM: position.coords.accuracy,
        }, { silentSuccess: true });
        if (!msg.success) {
          setLocationState('error');
          return;
        }
        setLocationState('granted');
        await refresh();
      },
      (error) => {
        setLocationState(error.code === error.PERMISSION_DENIED ? 'denied' : 'error');
      },
      { enableHighAccuracy: true, timeout: 15_000, maximumAge: 300_000 },
    );
  }, [refresh]);

  useEffect(() => {
    if (!window.isSecureContext || !navigator.geolocation || !navigator.permissions) return;
    void navigator.permissions.query({ name: 'geolocation' }).then((permission) => {
      if (permission.state === 'granted') requestPreciseLocation();
    }).catch(() => undefined);
  }, [requestPreciseLocation]);

  const online = data.nodes.filter((node) => node.status === 'online').length;
  const active = data.connections.reduce((sum, item) => sum + item.activeCount, 0);
  const gpsEnabled = locationState === 'granted' || data.gpsReady;
  const locationLabel = gpsEnabled
    ? 'GPS 精准定位已启用'
    : locationState === 'insecure'
      ? '需要 HTTPS 才能启用 GPS'
      : locationState === 'denied'
        ? '定位权限已拒绝，使用 IP 定位'
        : locationState === 'unsupported'
          ? '设备不支持 GPS，使用 IP 定位'
          : data.geoReady ? '当前使用 IP 城市定位' : '等待定位数据';

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={`index-page customer-dashboard ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area customer-content">
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
              <Card><Statistic title="我的专线网络" value={data.nodes.length} /></Card>
              <Card><Statistic title="在线网络" value={online} /></Card>
              <Card><Statistic title="活跃连接" value={active} /></Card>
              <Card>
                <Space direction="vertical" size={4}>
                  <span className="ant-statistic-title">定位状态</span>
                  <strong>{locationLabel}</strong>
                  {!gpsEnabled && (
                    <Button
                      type="link"
                      size="small"
                      className="gps-consent-button"
                      icon={locationState === 'locating' ? <LoadingOutlined /> : <EnvironmentOutlined />}
                      loading={locationState === 'locating'}
                      onClick={requestPreciseLocation}
                    >
                      允许精准定位
                    </Button>
                  )}
                </Space>
              </Card>
            </div>
            <GlobalNetworkMap />
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
