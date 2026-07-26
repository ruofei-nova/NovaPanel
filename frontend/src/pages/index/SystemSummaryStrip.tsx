import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  DesktopOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  GlobalOutlined,
  SwapOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import type { Status } from '@/models/status';
import { SizeFormatter, TimeFormatter } from '@/utils';
import './SystemSummaryStrip.css';

interface SystemSummaryStripProps {
  status: Status;
}

export default function SystemSummaryStrip({ status }: SystemSummaryStripProps) {
  const { t } = useTranslation();
  const [showIp, setShowIp] = useState(false);

  return (
    <section className="system-summary" aria-label="系统实时摘要">
      <div className="summary-section summary-uptime">
        <span className="summary-title">系统运行时间</span>
        <div>
          <span><ThunderboltOutlined /> Xray <b>{TimeFormatter.formatSecond(status.appStats.uptime)}</b></span>
          <span><DesktopOutlined /> OS <b>{TimeFormatter.formatSecond(status.uptime)}</b></span>
        </div>
      </div>

      <div className="summary-section">
        <span className="summary-title">上传 / 下载</span>
        <div>
          <span><ArrowUpOutlined /> <b>{SizeFormatter.sizeFormat(status.netIO.up)}/s</b></span>
          <span><ArrowDownOutlined /> <b>{SizeFormatter.sizeFormat(status.netIO.down)}/s</b></span>
        </div>
      </div>

      <div className="summary-section">
        <span className="summary-title">总数据</span>
        <div>
          <span><CloudUploadOutlined /> <b>{SizeFormatter.sizeFormat(status.netTraffic.sent)}</b></span>
          <span><CloudDownloadOutlined /> <b>{SizeFormatter.sizeFormat(status.netTraffic.recv)}</b></span>
        </div>
      </div>

      <div className="summary-section">
        <span className="summary-title">TCP / UDP</span>
        <div>
          <span><SwapOutlined /> <b>{status.tcpCount}</b></span>
          <span><SwapOutlined /> <b>{status.udpCount}</b></span>
        </div>
      </div>

      <div className={`summary-section summary-ip ${showIp ? 'visible' : 'hidden'}`}>
        <button
          type="button"
          className="summary-title ip-visibility"
          aria-label={t('pages.index.toggleIpVisibility')}
          onClick={() => setShowIp((value) => !value)}
        >
          IP 地址 {showIp ? <EyeOutlined /> : <EyeInvisibleOutlined />}
        </button>
        <div>
          <span><GlobalOutlined /> IPv4 <b>{status.publicIP.ipv4}</b></span>
          <span><GlobalOutlined /> IPv6 <b>{status.publicIP.ipv6}</b></span>
        </div>
      </div>
    </section>
  );
}
