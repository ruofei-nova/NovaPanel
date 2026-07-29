import { describe, expect, it } from 'vitest';

import { globePosition } from '@/pages/index/GlobalNetworkMap';

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
});
