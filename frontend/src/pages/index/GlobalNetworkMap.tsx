import { useEffect, useMemo, useRef } from 'react';
import { Card, Tag } from 'antd';

import { useNodesQuery, type NodeRecord } from '@/api/queries/useNodesQuery';
import worldMap from '@/assets/nova-world-map.png';
import './GlobalNetworkMap.css';

interface Point {
  x: number;
  y: number;
  status: 'online' | 'slow' | 'offline';
  label: string;
}

function hash(input: string): number {
  let value = 2166136261;
  for (let i = 0; i < input.length; i += 1) {
    value ^= input.charCodeAt(i);
    value = Math.imul(value, 16777619);
  }
  return value >>> 0;
}

function pointFor(node: NodeRecord, index: number): Point {
  const seed = hash(`${node.guid || node.id}-${node.address || node.name || index}`);
  const x = 0.08 + ((seed % 8200) / 10000);
  const y = 0.22 + (((seed >>> 8) % 5200) / 10000);
  const status = node.status !== 'online'
    ? 'offline'
    : (node.latencyMs || 0) >= 180 ? 'slow' : 'online';
  return {
    x,
    y,
    status,
    label: node.name || node.remark || `Node ${index + 1}`,
  };
}

export default function GlobalNetworkMap() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const { nodes, totals } = useNodesQuery();
  const points = useMemo(() => nodes.filter((node) => node.enable).map(pointFor), [nodes]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;
    const ctx = canvas.getContext('2d');
    if (!ctx) return undefined;

    let frame = 0;
    let animationId = 0;
    const resize = () => {
      const rect = canvas.getBoundingClientRect();
      const ratio = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.round(rect.width * ratio));
      canvas.height = Math.max(1, Math.round(rect.height * ratio));
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    };

    const draw = () => {
      const width = canvas.clientWidth;
      const height = canvas.clientHeight;
      ctx.clearRect(0, 0, width, height);
      const hub = points[0] || { x: 0.64, y: 0.47, status: 'online' as const, label: 'NovaPanel' };
      const pulse = (Math.sin(frame / 28) + 1) / 2;

      for (const point of points.slice(1)) {
        const startX = hub.x * width;
        const startY = hub.y * height;
        const endX = point.x * width;
        const endY = point.y * height;
        const midX = (startX + endX) / 2;
        const bend = Math.max(18, Math.abs(endX - startX) * 0.15);
        ctx.beginPath();
        ctx.moveTo(startX, startY);
        ctx.quadraticCurveTo(midX, Math.min(startY, endY) - bend, endX, endY);
        ctx.strokeStyle = point.status === 'offline'
          ? 'rgba(255, 95, 91, 0.34)'
          : point.status === 'slow'
            ? 'rgba(255, 178, 72, 0.36)'
            : 'rgba(85, 230, 210, 0.3)';
        ctx.lineWidth = 1;
        ctx.stroke();
      }

      const visiblePoints = points.length > 0 ? points : [hub];
      for (const point of visiblePoints) {
        const x = point.x * width;
        const y = point.y * height;
        const color = point.status === 'offline'
          ? '#ff5f5b'
          : point.status === 'slow' ? '#ffb248' : '#55e6d2';
        ctx.beginPath();
        ctx.arc(x, y, 8 + pulse * 5, 0, Math.PI * 2);
        ctx.fillStyle = `${color}18`;
        ctx.fill();
        ctx.beginPath();
        ctx.arc(x, y, 3.1, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.shadowColor = color;
        ctx.shadowBlur = 14;
        ctx.fill();
        ctx.shadowBlur = 0;
      }

      frame += 1;
      animationId = window.requestAnimationFrame(draw);
    };

    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    draw();
    return () => {
      observer.disconnect();
      window.cancelAnimationFrame(animationId);
    };
  }, [points]);

  return (
    <Card
      className="network-map-card"
      title={
        <span className="network-title">
          <span>全球网络状态</span>
          <Tag color={totals.offline > 0 ? 'warning' : 'success'}>
            {totals.total} 个节点 · {totals.online} 在线
          </Tag>
        </span>
      }
    >
      <div className="network-map-stage">
        <img src={worldMap} alt="" aria-hidden="true" />
        <canvas ref={canvasRef} aria-label={`全球网络状态，${totals.total} 个节点，${totals.online} 在线`} />
        {points.length === 0 && <span className="network-empty">等待节点接入</span>}
      </div>
      <div className="network-legend" aria-hidden="true">
        <span><i className="online" />在线</span>
        <span><i className="slow" />高延迟</span>
        <span><i className="offline" />离线</span>
        {totals.avgLatency > 0 && <b>平均延迟 {totals.avgLatency} ms</b>}
      </div>
    </Card>
  );
}
