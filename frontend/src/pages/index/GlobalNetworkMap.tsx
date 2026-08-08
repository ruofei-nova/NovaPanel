import { useEffect, useMemo, useRef, useState } from 'react';
import { Card } from 'antd';
import * as THREE from 'three';

import { useNetworkMapQuery } from '@/api/queries/useNetworkMapQuery';
import globeTexture from '@/assets/nova-globe-texture.png';
import './GlobalNetworkMap.css';

interface GlobeNode {
  id: number;
  name: string;
  longitude: number;
  latitude: number;
  status: 'online' | 'slow' | 'offline';
}

interface GlobeTooltip {
  title: string;
  detail: string;
  x: number;
  y: number;
}

export const HONG_KONG_HUB = {
  name: '香港网络中心',
  latitude: 22.3193,
  longitude: 114.1694,
} as const;

function toGlobeNode(node: {
  id: number;
  name: string;
  longitude: number;
  latitude: number;
  status: string;
  latencyMs: number;
}): GlobeNode {
  return {
    id: node.id,
    name: node.name,
    longitude: node.longitude * (Math.PI / 180),
    latitude: node.latitude * (Math.PI / 180),
    status: node.status !== 'online'
      ? 'offline'
      : node.latencyMs >= 180 ? 'slow' : 'online',
  };
}

export function globePosition(latitude: number, longitude: number, radius = 2.025) {
  const latitudeScale = Math.cos(latitude);
  return new THREE.Vector3(
    radius * latitudeScale * Math.cos(longitude),
    radius * Math.sin(latitude),
    -radius * latitudeScale * Math.sin(longitude),
  );
}

function disposeObject(child: THREE.Object3D) {
  if (!(child instanceof THREE.Mesh)) return;
  child.geometry.dispose();
  const material = child.material;
  if (Array.isArray(material)) material.forEach((item) => item.dispose());
  else material.dispose();
}

export default function GlobalNetworkMap() {
  const mountRef = useRef<HTMLDivElement>(null);
  const networkGroupRef = useRef<THREE.Group | null>(null);
  const [tooltip, setTooltip] = useState<GlobeTooltip | null>(null);
  const { data } = useNetworkMapQuery();
  const globeNodes = useMemo(
    () => data.nodes
      .filter((node) => node.latitude !== 0 || node.longitude !== 0)
      .map(toGlobeNode),
    [data.nodes],
  );
  const nodeByID = useMemo(
    () => new Map(globeNodes.map((node) => [node.id, node])),
    [globeNodes],
  );

  useEffect(() => {
    const mount = mountRef.current;
    if (!mount) return undefined;

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(34, 1, 0.1, 100);
    camera.position.set(0, 0.08, 7.55);

    const renderer = new THREE.WebGLRenderer({
      alpha: true,
      antialias: true,
      powerPreference: 'high-performance',
    });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.28;
    renderer.setClearColor(0x000000, 0);
    mount.appendChild(renderer.domElement);

    const globeGroup = new THREE.Group();
    globeGroup.rotation.x = -0.08;
    globeGroup.rotation.z = -0.12;
    scene.add(globeGroup);

    const texture = new THREE.TextureLoader().load(globeTexture);
    texture.colorSpace = THREE.SRGBColorSpace;
    texture.wrapS = THREE.RepeatWrapping;
    texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());

    const geometry = new THREE.SphereGeometry(2, 128, 96);
    const material = new THREE.MeshStandardMaterial({
      map: texture,
      color: 0x82d8cc,
      emissive: 0x021516,
      emissiveIntensity: 0.32,
      metalness: 0.06,
      roughness: 0.82,
    });
    globeGroup.add(new THREE.Mesh(geometry, material));

    const networkGroup = new THREE.Group();
    globeGroup.add(networkGroup);
    networkGroupRef.current = networkGroup;

    scene.add(new THREE.HemisphereLight(0x9ffff3, 0x010608, 1.35));
    const keyLight = new THREE.DirectionalLight(0xb6fff6, 2.35);
    keyLight.position.set(-3.5, 3.2, 4.5);
    scene.add(keyLight);
    const rimLight = new THREE.DirectionalLight(0x1a9b90, 1.8);
    rimLight.position.set(4, -1.5, -3);
    scene.add(rimLight);

    const resize = () => {
      const { width, height } = mount.getBoundingClientRect();
      renderer.setSize(Math.max(1, width), Math.max(1, height), false);
      camera.aspect = Math.max(1, width) / Math.max(1, height);
      camera.updateProjectionMatrix();
    };
    const observer = new ResizeObserver(resize);
    observer.observe(mount);
    resize();

    const raycaster = new THREE.Raycaster();
    const pointer = new THREE.Vector2();
    const handlePointerMove = (event: PointerEvent) => {
      const bounds = renderer.domElement.getBoundingClientRect();
      pointer.x = ((event.clientX - bounds.left) / bounds.width) * 2 - 1;
      pointer.y = -((event.clientY - bounds.top) / bounds.height) * 2 + 1;
      raycaster.setFromCamera(pointer, camera);
      const hit = raycaster.intersectObjects(networkGroup.children, false)
        .find((item) => item.object.userData.tooltip);
      if (!hit) {
        setTooltip(null);
        renderer.domElement.style.cursor = 'default';
        return;
      }
      const info = hit.object.userData.tooltip as Omit<GlobeTooltip, 'x' | 'y'>;
      setTooltip({
        ...info,
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });
      renderer.domElement.style.cursor = 'crosshair';
    };
    const handlePointerLeave = () => setTooltip(null);
    renderer.domElement.addEventListener('pointermove', handlePointerMove);
    renderer.domElement.addEventListener('pointerleave', handlePointerLeave);

    const timer = new THREE.Timer();
    let animationId = 0;
    const animate = () => {
      timer.update();
      const delta = Math.min(timer.getDelta(), 0.05);
      globeGroup.rotation.y += delta * 0.18;
      for (const child of networkGroup.children) {
        if (child.userData.pulse) {
          const scale = 0.84 + Math.sin(timer.getElapsed() * 3.2 + child.userData.phase) * 0.2;
          child.scale.setScalar(scale);
        }
      }
      renderer.render(scene, camera);
      animationId = window.requestAnimationFrame(animate);
    };
    animate();

    return () => {
      window.cancelAnimationFrame(animationId);
      observer.disconnect();
      renderer.domElement.removeEventListener('pointermove', handlePointerMove);
      renderer.domElement.removeEventListener('pointerleave', handlePointerLeave);
      networkGroupRef.current = null;
      for (const child of networkGroup.children) disposeObject(child);
      geometry.dispose();
      material.dispose();
      texture.dispose();
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, []);

  useEffect(() => {
    const group = networkGroupRef.current;
    if (!group) return;

    for (const child of [...group.children]) {
      group.remove(child);
      disposeObject(child);
    }

    for (const node of globeNodes.slice(0, 48)) {
      const marker = new THREE.Mesh(
        new THREE.SphereGeometry(0.038, 12, 12),
        new THREE.MeshBasicMaterial({
          color: node.status === 'offline'
            ? 0xff5f5b
            : node.status === 'slow' ? 0xffb248 : 0x55e6d2,
        }),
      );
      marker.position.copy(globePosition(node.latitude, node.longitude));
      marker.userData.pulse = true;
      marker.userData.phase = node.id;
      marker.userData.tooltip = {
        title: node.name,
        detail: `VPS 节点 · ${THREE.MathUtils.radToDeg(node.latitude).toFixed(6)}, ${THREE.MathUtils.radToDeg(node.longitude).toFixed(6)}`,
      };
      group.add(marker);
    }

    const hubLatitude = HONG_KONG_HUB.latitude * (Math.PI / 180);
    const hubLongitude = HONG_KONG_HUB.longitude * (Math.PI / 180);
    const hubPosition = globePosition(hubLatitude, hubLongitude);
    const hubMarker = new THREE.Mesh(
      new THREE.SphereGeometry(0.064, 16, 16),
      new THREE.MeshBasicMaterial({
        color: 0xffd36a,
        transparent: true,
        opacity: 0.96,
        blending: THREE.AdditiveBlending,
      }),
    );
    hubMarker.position.copy(hubPosition);
    hubMarker.userData.pulse = true;
    hubMarker.userData.phase = 0;
    hubMarker.userData.tooltip = {
      title: HONG_KONG_HUB.name,
      detail: `固定起点 · ${HONG_KONG_HUB.latitude.toFixed(6)}, ${HONG_KONG_HUB.longitude.toFixed(6)}`,
    };
    group.add(hubMarker);

    for (const node of globeNodes.slice(0, 48)) {
      const target = globePosition(node.latitude, node.longitude);
      if (hubPosition.distanceTo(target) < 0.02) continue;
      const midpoint = hubPosition.clone().add(target).multiplyScalar(0.5);
      const distance = hubPosition.distanceTo(target);
      midpoint.normalize().multiplyScalar(2.18 + Math.min(1.05, distance * 0.26));
      const curve = new THREE.QuadraticBezierCurve3(hubPosition, midpoint, target);
      const hubLine = new THREE.Mesh(
        new THREE.TubeGeometry(curve, 40, 0.008, 6, false),
        new THREE.MeshBasicMaterial({
          color: 0x7ef5e3,
          transparent: true,
          opacity: node.status === 'offline' ? 0.18 : 0.46,
          blending: THREE.AdditiveBlending,
        }),
      );
      group.add(hubLine);
    }

    for (const [index, connection] of data.connections.slice(0, 80).entries()) {
      const node = nodeByID.get(connection.nodeId);
      if (!node) continue;
      const source = globePosition(
        connection.latitude * (Math.PI / 180),
        connection.longitude * (Math.PI / 180),
      );
      const target = globePosition(node.latitude, node.longitude);
      const midpoint = source.clone().add(target).multiplyScalar(0.5);
      const distance = source.distanceTo(target);
      midpoint.normalize().multiplyScalar(2.15 + Math.min(0.9, distance * 0.24));
      const curve = new THREE.QuadraticBezierCurve3(source, midpoint, target);
      const line = new THREE.Mesh(
        new THREE.TubeGeometry(curve, 36, 0.009, 6, false),
        new THREE.MeshBasicMaterial({
          color: connection.source === 'gps' ? 0x8cfff0 : 0x53ead7,
          transparent: true,
          opacity: connection.source === 'gps' ? 0.72 : 0.48,
          blending: THREE.AdditiveBlending,
        }),
      );
      group.add(line);

      const sourceMarker = new THREE.Mesh(
        new THREE.SphereGeometry(0.026 + Math.min(connection.activeCount, 8) * 0.003, 10, 10),
        new THREE.MeshBasicMaterial({
          color: connection.source === 'gps' ? 0xd2fff8 : 0x8cfff0,
          transparent: true,
          opacity: 0.92,
          blending: THREE.AdditiveBlending,
        }),
      );
      sourceMarker.position.copy(source);
      sourceMarker.userData.pulse = true;
      sourceMarker.userData.phase = index * 0.7;
      sourceMarker.userData.tooltip = {
        title: connection.source === 'gps' ? '客户 GPS 位置' : '客户网络位置',
        detail: `${connection.source} · ${connection.latitude.toFixed(6)}, ${connection.longitude.toFixed(6)} · ${connection.activeCount} 在线`,
      };
      group.add(sourceMarker);
    }
  }, [data.connections, globeNodes, nodeByID]);

  return (
    <Card className="network-map-card globe-card" title="全球网络状态">
      <div className="globe-stage">
        <div
          ref={mountRef}
          className="globe-webgl"
          aria-label="自动旋转的三维全球网络球体"
        />
        {tooltip && (
          <div
            className="globe-tooltip"
            style={{ left: tooltip.x, top: tooltip.y }}
            role="status"
          >
            <strong>{tooltip.title}</strong>
            <span>{tooltip.detail}</span>
          </div>
        )}
      </div>
      <div className="network-legend" aria-hidden="true">
        <span><i className="online" />在线</span>
        <span><i className="slow" />高延迟</span>
        <span><i className="offline" />离线</span>
        <span><i className="hub" />香港中心</span>
        <b>LIVE NETWORK</b>
      </div>
      <a
        className="geo-attribution"
        href="https://db-ip.com"
        target="_blank"
        rel="noreferrer"
      >
        IP Geolocation by DB-IP
      </a>
    </Card>
  );
}
