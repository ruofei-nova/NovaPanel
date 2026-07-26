import { useCallback, useEffect, useState } from 'react';

import { HttpUtil } from '@/utils';

export interface NetworkMapNode {
  id: number;
  guid: string;
  name: string;
  status: string;
  latencyMs: number;
  latitude: number;
  longitude: number;
  ownerUserId?: number;
}

export interface NetworkMapConnection {
  nodeId: number;
  latitude: number;
  longitude: number;
  lastSeen: number;
  activeCount: number;
}

export interface NetworkMapPayload {
  geoReady: boolean;
  generatedAt: number;
  nodes: NetworkMapNode[];
  connections: NetworkMapConnection[];
}

const emptyPayload: NetworkMapPayload = {
  geoReady: false,
  generatedAt: 0,
  nodes: [],
  connections: [],
};

export function useNetworkMapQuery() {
  const [data, setData] = useState<NetworkMapPayload>(emptyPayload);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const msg = await HttpUtil.get<NetworkMapPayload>('/panel/api/network/map');
    if (msg?.success && msg.obj) setData(msg.obj);
    setLoading(false);
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 10_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  return { data, loading, refresh };
}
