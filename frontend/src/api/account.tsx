import { createContext, useContext, useEffect, useMemo, useState } from 'react';

import { HttpUtil } from '@/utils';

export interface PanelAccount {
  id: number;
  username: string;
  role: 'admin' | 'customer';
}

interface AccountContextValue {
  account: PanelAccount | null;
  loading: boolean;
}

const AccountContext = createContext<AccountContextValue>({
  account: null,
  loading: true,
});

export function AccountProvider({ children }: { children: React.ReactNode }) {
  const [account, setAccount] = useState<PanelAccount | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    HttpUtil.get<PanelAccount>('/panel/api/account/me')
      .then((msg) => {
        if (active && msg?.success && msg.obj) setAccount(msg.obj);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => { active = false; };
  }, []);

  const value = useMemo(() => ({ account, loading }), [account, loading]);
  return <AccountContext.Provider value={value}>{children}</AccountContext.Provider>;
}

export function useAccount() {
  return useContext(AccountContext);
}
