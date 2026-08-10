import { describe, expect, it } from 'vitest';

import { totalNodeOnlineCount } from '@/api/queries/useNetworkMapQuery';

describe('customer dashboard active connections', () => {
  it('uses node online counts instead of geolocation records', () => {
    expect(totalNodeOnlineCount([
      { onlineCount: 2 },
      { onlineCount: 1 },
    ])).toBe(3);
  });

  it('shows zero only when none of the customer nodes has an online client', () => {
    expect(totalNodeOnlineCount([{ onlineCount: 0 }])).toBe(0);
  });
});
