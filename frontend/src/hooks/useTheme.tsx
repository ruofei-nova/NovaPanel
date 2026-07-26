import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function applyDom(isDark: boolean, isUltra: boolean) {
  document.body.setAttribute('class', isDark ? 'dark' : 'light');
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  const msg = document.getElementById('message');
  if (msg) msg.className = isDark ? 'dark' : 'light';
}

// module load so the document is in the right theme before React mounts.
const initialDark = readBool(STORAGE_DARK, true);
const initialUltra = readBool(STORAGE_ULTRA, false);
applyDom(initialDark, initialUltra);

const DARK_TOKENS = {
  colorPrimary: '#55e6d2',
  colorPrimaryHover: '#73f2df',
  colorPrimaryActive: '#2bbda9',
  colorInfo: '#55e6d2',
  colorSuccess: '#4cdda8',
  colorWarning: '#ffb248',
  colorError: '#ff5f5b',
  colorBgBase: '#061116',
  colorBgLayout: '#061116',
  colorBgContainer: '#0b1b21',
  colorBgElevated: '#10262d',
  colorBorder: 'rgba(106, 231, 215, 0.2)',
  colorBorderSecondary: 'rgba(106, 231, 215, 0.12)',
  colorText: 'rgba(235, 248, 247, 0.92)',
  colorTextSecondary: 'rgba(216, 236, 234, 0.68)',
  colorTextTertiary: 'rgba(196, 219, 217, 0.48)',
  borderRadius: 9,
  fontFamily: '"Inter", "Noto Sans SC", "Microsoft YaHei UI", system-ui, sans-serif',
};
const ULTRA_DARK_TOKENS = {
  ...DARK_TOKENS,
  colorBgBase: '#03090c',
  colorBgLayout: '#03090c',
  colorBgContainer: '#08161b',
  colorBgElevated: '#0d2228',
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: '#061116',
  headerBg: '#071419',
  headerColor: '#ffffff',
  footerBg: '#061116',
  siderBg: '#061217',
  triggerBg: '#0b1b21',
  triggerColor: '#ffffff',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#03090c',
  headerBg: '#030b0e',
  headerColor: '#ffffff',
  footerBg: '#03090c',
  siderBg: '#030b0e',
  triggerBg: '#08161b',
  triggerColor: '#ffffff',
};
const DARK_MENU_TOKENS = {
  darkItemBg: '#061217',
  darkSubMenuItemBg: '#07161b',
  darkPopupBg: '#0b1b21',
  darkItemSelectedBg: 'rgba(85, 230, 210, 0.13)',
  darkItemSelectedColor: '#73f2df',
  darkItemHoverBg: 'rgba(85, 230, 210, 0.08)',
};
const ULTRA_DARK_MENU_TOKENS = {
  ...DARK_MENU_TOKENS,
  darkItemBg: '#030b0e',
  darkSubMenuItemBg: '#041014',
  darkPopupBg: '#08161b',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(85, 230, 210, 0.12)',
  headerBg: 'transparent',
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(85, 230, 210, 0.1)',
  headerBg: 'transparent',
};
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: 'rgba(0, 0, 0, 0.58)',
  colorTextTertiary: 'rgba(0, 0, 0, 0.58)',
  colorTextPlaceholder: '#767676',
  colorError: '#cf1322',
  colorErrorText: '#cf1322',
  colorSuccessText: '#237804',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: '#0958d9',
  colorPrimaryHover: '#2468e5',
  colorPrimaryActive: '#073ea8',
};

export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  if (!isDark) {
    return {
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_CONTRAST_TOKENS,
      components: {
        Statistic: STATISTIC_TOKENS,
        Button: LIGHT_BUTTON_TOKENS,
      },
    };
  }
  return {
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Statistic: STATISTIC_TOKENS,
      Table: {
        headerBg: isUltra ? '#08161b' : '#0b1b21',
        headerColor: 'rgba(235, 248, 247, 0.78)',
        rowHoverBg: 'rgba(85, 230, 210, 0.055)',
        borderColor: 'rgba(85, 230, 210, 0.11)',
      },
      Button: {
        primaryShadow: '0 0 20px rgba(85, 230, 210, 0.16)',
      },
      Input: {
        activeBorderColor: '#55e6d2',
        hoverBorderColor: 'rgba(85, 230, 210, 0.55)',
        activeShadow: '0 0 0 2px rgba(85, 230, 210, 0.1)',
      },
    },
  };
}

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

interface ThemeContextValue {
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);

  useEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [isDark, isUltra]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(isDark, isUltra), [isDark, isUltra]);

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
