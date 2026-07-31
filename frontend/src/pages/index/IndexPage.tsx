import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  message,
  Modal,
  Result,
  Row,
  Space,
  Spin,
  Tag,
} from 'antd';
import {
  BarsOutlined,
  ControlOutlined,
  CloudServerOutlined,
  CloudDownloadOutlined,
  AreaChartOutlined,
  CopyOutlined,
  TelegramFilled,
} from '@ant-design/icons';

import { HttpUtil, ClipboardManager, FileManager } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import { activateOnKey } from '@/utils/a11y';
import { useTheme } from '@/hooks/useTheme';
import { useStatusQuery } from '@/api/queries/useStatusQuery';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { LazyMount } from '@/components/utility';
import { setMessageInstance } from '@/utils/messageBus';
import { useAccount } from '@/api/account';
import StatusCard from './StatusCard';
import XrayStatusCard from './XrayStatusCard';
import GlobalNetworkMap from './GlobalNetworkMap';
import SystemSummaryStrip from './SystemSummaryStrip';
import CustomerDashboard from './CustomerDashboard';
import type { PanelUpdateInfo } from './PanelUpdateModal';
const JsonEditor = lazy(() => import('@/components/form/JsonEditor'));
const PanelUpdateModal = lazy(() => import('./PanelUpdateModal'));
const LogModal = lazy(() => import('./LogModal'));
const BackupModal = lazy(() => import('./BackupModal'));
const SystemHistoryModal = lazy(() => import('./SystemHistoryModal'));
const XrayMetricsModal = lazy(() => import('./XrayMetricsModal'));
const XrayLogModal = lazy(() => import('./XrayLogModal'));
const VersionModal = lazy(() => import('./VersionModal'));
import './IndexPage.css';

function AdminIndexPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { status, fetched, fetchError, refresh } = useStatusQuery();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const [clock, setClock] = useState(() => new Date());
  useEffect(() => {
    const timer = window.setInterval(() => setClock(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const [accessLogEnable, setAccessLogEnable] = useState(false);
  const [devChannelEnable, setDevChannelEnable] = useState(false);
  const [panelUpdateInfo, setPanelUpdateInfo] = useState<PanelUpdateInfo>({
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
  });

  const basePath = window.X_UI_BASE_PATH || '';

  const [logsOpen, setLogsOpen] = useState(false);
  const [backupOpen, setBackupOpen] = useState(false);
  const [panelUpdateOpen, setPanelUpdateOpen] = useState(false);
  const [sysHistoryOpen, setSysHistoryOpen] = useState(false);
  const [xrayMetricsOpen, setXrayMetricsOpen] = useState(false);
  const [xrayLogsOpen, setXrayLogsOpen] = useState(false);
  const [versionOpen, setVersionOpen] = useState(false);
  const [configTextOpen, setConfigTextOpen] = useState(false);
  const [configText, setConfigText] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadingTip, setLoadingTip] = useState(t('loading'));

  useEffect(() => {
    HttpUtil.post<{ accessLogEnable?: boolean; devChannelEnable?: boolean }>(
      '/panel/api/setting/defaultSettings',
    ).then((msg) => {
      if (msg?.success && msg.obj) {
        setAccessLogEnable(!!msg.obj.accessLogEnable);
        setDevChannelEnable(!!msg.obj.devChannelEnable);
      }
    });
    HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo').then((msg) => {
      if (msg?.success && msg.obj) setPanelUpdateInfo(msg.obj);
    });
  }, []);

  const displayVersion = useMemo(
    () => window.X_UI_CUR_VER || panelUpdateInfo.currentVersion || '?',
    [panelUpdateInfo.currentVersion],
  );

  const setBusy = useCallback(
    ({ busy, tip }: { busy: boolean; tip?: string }) => {
      setLoading(busy);
      if (tip) setLoadingTip(tip);
    },
    [],
  );

  const stopXray = useCallback(async () => {
    await HttpUtil.post('/panel/api/server/stopXrayService');
    await refresh();
  }, [refresh]);

  const restartXray = useCallback(async () => {
    await HttpUtil.post('/panel/api/server/restartXrayService');
    await refresh();
  }, [refresh]);

  function openPanelVersion() {
    setPanelUpdateOpen(true);
  }

  async function handleChannelChange(dev: boolean) {
    const res = await HttpUtil.post('/panel/api/server/setUpdateChannel', { dev });
    if (!res?.success) return;
    setDevChannelEnable(dev);
    const msg = await HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo');
    if (msg?.success && msg.obj) setPanelUpdateInfo(msg.obj);
  }

  function openTelegram() {
    window.open('https://t.me/XrayUI', '_blank', 'noopener,noreferrer');
  }

  async function openConfig() {
    setLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/api/server/getConfigJson');
      if (!msg?.success) return;
      setConfigText(JSON.stringify(msg.obj, null, 2));
      setConfigTextOpen(true);
    } finally {
      setLoading(false);
    }
  }

  async function copyConfig() {
    const ok = await ClipboardManager.copyText(configText || '');
    if (ok) messageApi.success('Copied');
  }

  function downloadConfig() {
    FileManager.downloadTextFile(configText, 'config.json');
  }

  const pageClass = `index-page ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`.trim();

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content className="content-area">
            <header className="dashboard-command-header">
              <div>
                <span className="dashboard-kicker">NOVARUO / CONTROL CENTER</span>
                <h1>系统状态</h1>
              </div>
              <div className="dashboard-live-meta">
                <time dateTime={clock.toISOString()}>
                  {clock.toLocaleString('zh-CN', { hour12: false })}
                </time>
                <span className={`xray-live-state ${status.xray.state}`}>
                  <i />Xray {status.xray.state === 'running' ? '运行中' : '状态异常'}
                </span>
              </div>
            </header>
            <Spin
              spinning={loading || !fetched}
              delay={200}
              description={loading ? loadingTip : t('loading')}
              size="large"
            >
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={refresh}>{t('refresh')}</Button>}
                />
              ) : (
                <Row className="dashboard-grid" gutter={[isMobile ? 8 : 16, 12]}>
                  <Col span={24}>
                    <StatusCard status={status} isMobile={isMobile} />
                  </Col>

                  <Col span={24}>
                    <GlobalNetworkMap />
                  </Col>

                  <Col xs={24} lg={12}>
                    <XrayStatusCard
                      status={status}
                      isMobile={isMobile}
                      accessLogEnable={accessLogEnable}
                      onStopXray={stopXray}
                      onRestartXray={restartXray}
                      onOpenXrayLogs={() => setXrayLogsOpen(true)}
                      onOpenLogs={() => setLogsOpen(true)}
                      onOpenVersionSwitch={() => setVersionOpen(true)}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={t('menu.link')}
                      hoverable
                      actions={[
                        <Space className="action" key="logs" role="button" tabIndex={0} aria-label={t('pages.index.logs')} onClick={() => setLogsOpen(true)} onKeyDown={activateOnKey(() => setLogsOpen(true))}>
                          <BarsOutlined />
                          {!isMobile && <span>{t('pages.index.logs')}</span>}
                        </Space>,
                        <Space className="action" key="config" role="button" tabIndex={0} aria-label={t('pages.index.config')} onClick={openConfig} onKeyDown={activateOnKey(openConfig)}>
                          <ControlOutlined />
                          {!isMobile && <span>{t('pages.index.config')}</span>}
                        </Space>,
                        <Space className="action" key="backup" role="button" tabIndex={0} aria-label={t('pages.index.backupTitle')} onClick={() => setBackupOpen(true)} onKeyDown={activateOnKey(() => setBackupOpen(true))}>
                          <CloudServerOutlined />
                          {!isMobile && <span>{t('pages.index.backupTitle')}</span>}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={
                        <Space>
                          <span>NovaRuo</span>
                          {isMobile && displayVersion && (
                            <Tag color={panelUpdateInfo.updateAvailable ? 'orange' : 'green'}>
                              {panelUpdateInfo.updateAvailable
                                ? formatPanelVersion(panelUpdateInfo.latestVersion)
                                : formatPanelVersion(displayVersion)}
                            </Tag>
                          )}
                        </Space>
                      }
                      hoverable
                      actions={[
                        <Space className="action" key="tg" role="button" tabIndex={0} aria-label="@XrayUI" onClick={openTelegram} onKeyDown={activateOnKey(openTelegram)}>
                          <TelegramFilled aria-hidden="true" />
                          {!isMobile && <span>@XrayUI</span>}
                        </Space>,
                        <Space
                          key="panel-version"
                          className={`action ${panelUpdateInfo.updateAvailable ? 'action-update' : ''}`}
                          role="button"
                          tabIndex={0}
                          aria-label={t('pages.index.updatePanel')}
                          onClick={openPanelVersion}
                          onKeyDown={activateOnKey(openPanelVersion)}
                        >
                          <CloudDownloadOutlined />
                          {!isMobile && (
                            <span>
                              {panelUpdateInfo.updateAvailable
                                ? `${t('update')} ${formatPanelVersion(panelUpdateInfo.latestVersion)}`
                                : formatPanelVersion(displayVersion)}
                            </span>
                          )}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col xs={24} lg={12}>
                    <Card
                      title={t('pages.index.charts')}
                      hoverable
                      actions={[
                        <Space
                          className="action"
                          key="sys-history"
                          role="button"
                          tabIndex={0}
                          aria-label={t('pages.index.systemHistoryTitle')}
                          onClick={() => setSysHistoryOpen(true)}
                          onKeyDown={activateOnKey(() => setSysHistoryOpen(true))}
                        >
                          <AreaChartOutlined />
                          {!isMobile && <span>{t('pages.index.systemHistoryTitle')}</span>}
                        </Space>,
                        <Space
                          className="action"
                          key="xray-metrics"
                          role="button"
                          tabIndex={0}
                          aria-label={t('pages.index.xrayMetricsTitle')}
                          onClick={() => setXrayMetricsOpen(true)}
                          onKeyDown={activateOnKey(() => setXrayMetricsOpen(true))}
                        >
                          <AreaChartOutlined />
                          {!isMobile && <span>{t('pages.index.xrayMetricsTitle')}</span>}
                        </Space>,
                      ]}
                    />
                  </Col>

                  <Col span={24}>
                    <SystemSummaryStrip status={status} />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <LazyMount when={panelUpdateOpen}>
          <PanelUpdateModal
            open={panelUpdateOpen}
            info={panelUpdateInfo}
            devChannelEnable={devChannelEnable}
            onChannelChange={handleChannelChange}
            onClose={() => setPanelUpdateOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>
        <LazyMount when={logsOpen}>
          <LogModal open={logsOpen} onClose={() => setLogsOpen(false)} />
        </LazyMount>
        <LazyMount when={backupOpen}>
          <BackupModal
            open={backupOpen}
            basePath={basePath}
            onClose={() => setBackupOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>
        <LazyMount when={sysHistoryOpen}>
          <SystemHistoryModal
            open={sysHistoryOpen}
            status={status}
            onClose={() => setSysHistoryOpen(false)}
          />
        </LazyMount>
        <LazyMount when={xrayMetricsOpen}>
          <XrayMetricsModal open={xrayMetricsOpen} onClose={() => setXrayMetricsOpen(false)} />
        </LazyMount>
        <LazyMount when={xrayLogsOpen}>
          <XrayLogModal open={xrayLogsOpen} onClose={() => setXrayLogsOpen(false)} />
        </LazyMount>
        <LazyMount when={versionOpen}>
          <VersionModal
            open={versionOpen}
            status={status}
            onClose={() => setVersionOpen(false)}
            onBusy={setBusy}
          />
        </LazyMount>

        <LazyMount when={configTextOpen}>
          <Modal
            open={configTextOpen}
            title={t('pages.index.config')}
            width={isMobile ? '100%' : 900}
            style={isMobile
              ? { top: 20, maxWidth: 'calc(100vw - 16px)' }
              : { top: 20 }}
            onCancel={() => setConfigTextOpen(false)}
            footer={[
              <Button
                key="download"
                onClick={downloadConfig}
                size={isMobile ? 'small' : 'middle'}
                icon={<CloudDownloadOutlined />}
              >
                {isMobile ? 'Download' : 'config.json'}
              </Button>,
              <Button
                key="copy"
                type="primary"
                onClick={copyConfig}
                size={isMobile ? 'small' : 'middle'}
                icon={<CopyOutlined />}
              >
                Copy
              </Button>,
            ]}
          >
            <JsonEditor
              value={configText}
              onChange={setConfigText}
              minHeight={isMobile ? '300px' : 'calc(100vh - 220px)'}
              maxHeight={isMobile ? '70vh' : 'calc(100vh - 220px)'}
              readOnly
            />
          </Modal>
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}

export default function IndexPage() {
  const { account } = useAccount();
  if (account?.role === 'customer') return <CustomerDashboard />;
  return <AdminIndexPage />;
}
