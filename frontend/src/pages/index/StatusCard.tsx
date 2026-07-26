import { useTranslation } from 'react-i18next';
import { Card, Col, Row, Tooltip } from 'antd';
import { AreaChartOutlined } from '@ant-design/icons';

import { CPUFormatter, SizeFormatter } from '@/utils';
import { useRollingSeries } from '@/hooks/useRollingSeries';
import { Sparkline } from '@/components/viz';
import type { Status } from '@/models/status';
import './StatusCard.css';

interface StatusCardProps {
  status: Status;
  isMobile: boolean;
}

export default function StatusCard({ status, isMobile }: StatusCardProps) {
  const { t } = useTranslation();
  const cpuHistory = useRollingSeries(status.cpu.percent);
  const memHistory = useRollingSeries(status.mem.percent);
  const swapHistory = useRollingSeries(status.swap.percent);
  const diskHistory = useRollingSeries(status.disk.percent);

  const metrics = [
    {
      key: 'cpu',
      label: t('pages.index.cpu'),
      percent: status.cpu.percent,
      detail: CPUFormatter.cpuCoreFormat(status.cpuCores),
      history: cpuHistory,
      tooltip: (
        <>
          <div><b>{t('pages.index.logicalProcessors')}:</b> {status.logicalPro}</div>
          <div><b>{t('pages.index.frequency')}:</b> {CPUFormatter.cpuSpeedFormat(status.cpuSpeedMhz)}</div>
        </>
      ),
    },
    {
      key: 'memory',
      label: t('pages.index.memory'),
      percent: status.mem.percent,
      detail: `${SizeFormatter.sizeFormat(status.mem.current)} / ${SizeFormatter.sizeFormat(status.mem.total)}`,
      history: memHistory,
    },
    {
      key: 'swap',
      label: t('pages.index.swap'),
      percent: status.swap.percent,
      detail: `${SizeFormatter.sizeFormat(status.swap.current)} / ${SizeFormatter.sizeFormat(status.swap.total)}`,
      history: swapHistory,
    },
    {
      key: 'storage',
      label: t('pages.index.storage'),
      percent: status.disk.percent,
      detail: `${SizeFormatter.sizeFormat(status.disk.current)} / ${SizeFormatter.sizeFormat(status.disk.total)}`,
      history: diskHistory,
    },
  ];

  return (
    <Card className="status-card telemetry-card">
      <Row gutter={[12, isMobile ? 12 : 0]}>
        {metrics.map((metric) => (
          <Col xs={24} sm={12} xl={6} key={metric.key}>
            <section className="telemetry-tile" aria-label={`${metric.label} ${metric.percent}%`}>
              <div className="telemetry-heading">
                <div>
                  <span className="telemetry-label">{metric.label}</span>
                  <strong className="telemetry-value">{metric.percent.toFixed(2)}<small>%</small></strong>
                </div>
                {metric.tooltip && (
                  <Tooltip title={metric.tooltip}>
                    <AreaChartOutlined className="telemetry-info" />
                  </Tooltip>
                )}
              </div>
              <div className="telemetry-chart">
                <Sparkline
                  data={metric.history}
                  height={78}
                  stroke="#55e6d2"
                  strokeWidth={1.8}
                  fillOpacity={0.2}
                  showGrid
                  showMarker
                  valueMin={0}
                  valueMax={100}
                />
              </div>
              <div className="telemetry-footer">
                <span>{metric.detail}</span>
                <span className="live-indicator"><i />LIVE</span>
              </div>
            </section>
          </Col>
        ))}
      </Row>
    </Card>
  );
}
