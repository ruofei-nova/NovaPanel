import { describe, expect, it } from 'vitest';

import {
  globePosition,
  globeTextureCoordinates,
  HONG_KONG_HUB,
  uniqueNodesById,
} from '@/pages/index/GlobalNetworkMap';

const radians = (degrees: number) => degrees * (Math.PI / 180);

describe('globePosition', () => {
  it('matches the equirectangular globe texture at cardinal longitudes', () => {
    const greenwich = globePosition(0, radians(0), 1);
    expect(greenwich.x).toBeCloseTo(1, 6);
    expect(greenwich.z).toBeCloseTo(0, 6);

    const east90 = globePosition(0, radians(90), 1);
    expect(east90.x).toBeCloseTo(0, 6);
    expect(east90.z).toBeCloseTo(-1, 6);

    const west90 = globePosition(0, radians(-90), 1);
    expect(west90.x).toBeCloseTo(0, 6);
    expect(west90.z).toBeCloseTo(1, 6);
  });

  it('places Los Angeles on the west coast hemisphere instead of the Pacific offset', () => {
    const losAngeles = globePosition(radians(34.0522), radians(-118.2437), 1);
    expect(losAngeles.x).toBeLessThan(0);
    expect(losAngeles.y).toBeGreaterThan(0);
    expect(losAngeles.z).toBeGreaterThan(0);
  });

  it('pins the network hub to Hong Kong', () => {
    expect(HONG_KONG_HUB.latitude).toBeCloseTo(22.3193, 4);
    expect(HONG_KONG_HUB.longitude).toBeCloseTo(114.1694, 4);

    const hongKong = globePosition(
      radians(HONG_KONG_HUB.latitude),
      radians(HONG_KONG_HUB.longitude),
      1,
    );
    expect(hongKong.y).toBeGreaterThan(0);
    expect(hongKong.z).toBeLessThan(0);
  });

  it('maps Hong Kong and Malaysia to their true equirectangular texture positions', () => {
    const hongKong = globeTextureCoordinates(22.3193, 114.1694);
    expect(hongKong.u).toBeCloseTo(0.817137, 6);
    expect(hongKong.v).toBeCloseTo(0.6240, 4);

    const malaysia = globeTextureCoordinates(2.8008619, 101.7094012);
    expect(malaysia.u).toBeCloseTo(0.782526, 6);
    expect(malaysia.v).toBeCloseTo(0.51556, 5);
  });
});

describe('uniqueNodesById', () => {
  it('keeps one rendered route per bound VPS even when payload rows repeat', () => {
    expect(uniqueNodesById([
      { id: 7, name: 'Los Angeles' },
      { id: 7, name: 'Los Angeles duplicate' },
      { id: 9, name: 'Malaysia' },
    ])).toEqual([
      { id: 7, name: 'Los Angeles duplicate' },
      { id: 9, name: 'Malaysia' },
    ]);
  });
});
