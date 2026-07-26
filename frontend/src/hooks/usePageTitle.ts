import { useEffect } from 'react';
import { useLocation } from 'react-router';
import { useTranslation } from 'react-i18next';

const TITLE_KEYS: Record<string, string> = {
  '/': 'menu.dashboard',
  '/inbounds': 'menu.inbounds',
  '/clients': 'menu.clients',
  '/customers': '客户账号',
  '/groups': 'menu.groups',
  '/nodes': 'menu.nodes',
  '/hosts': 'menu.hosts',
  '/settings': 'menu.settings',
  '/xray': 'menu.xray',
  '/outbound': 'menu.outbounds',
  '/routing': 'menu.routing',
  '/api-docs': 'menu.apiDocs',
};

export function usePageTitle() {
  const { pathname } = useLocation();
  const { t } = useTranslation();

  useEffect(() => {
    const key = TITLE_KEYS[pathname];
    const title = key ? t(key) : '';
    document.title = title ? `Nova Panel · ${title}` : 'Nova Panel';
  }, [pathname, t]);
}
