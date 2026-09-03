// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Theme = 'dark' | 'light';
/**
 * What the user chose. `system` follows the OS setting and keeps following it
 * as that setting changes, which is why the preference and the resolved value
 * have to be stored separately: collapsing them would freeze `system` to
 * whatever the OS happened to be at the moment of the choice.
 */
export type ThemeMode = 'dark' | 'light' | 'system';

/** The OS preference, or 'dark' where it cannot be read (SSR, old browsers). */
export function systemTheme(): Theme {
  if (typeof window === 'undefined' || !window.matchMedia) return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/** Resolves a mode to the theme actually applied to <html>. */
export function resolveTheme(mode: ThemeMode): Theme {
  return mode === 'system' ? systemTheme() : mode;
}
export type Variant = 'azure' | 'iris';
export type Lang = 'fr' | 'en';
/** UI density (docs/UI_ELEVATION_PROPOSAL §1.2). Confort is the ratified default. */
export type Density = 'comfort' | 'compact' | 'spacious';

interface UIState {
  /** Resolved theme currently on <html>. Read this to render. */
  theme: Theme;
  /** The user's choice, including 'system'. Read this to render the picker. */
  themeMode: ThemeMode;
  variant: Variant;
  lang: Lang;
  density: Density;
  sidebarCollapsed: boolean;
  /** ⌘K command palette open state (not persisted). */
  cmdkOpen: boolean;

  setTheme: (t: Theme) => void;
  setThemeMode: (m: ThemeMode) => void;
  toggleTheme: () => void;
  setVariant: (v: Variant) => void;
  setLang: (l: Lang) => void;
  toggleLang: () => void;
  setDensity: (d: Density) => void;
  cycleDensity: () => void;
  toggleSidebar: () => void;
  setSidebarCollapsed: (v: boolean) => void;
  setCmdkOpen: (v: boolean) => void;
  toggleCmdk: () => void;
}

/** Reflect the theme/accent onto <html> so the CSS-variable token system swaps. */
function applyDom(theme: Theme, variant: Variant, lang: Lang) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.setAttribute('data-theme', theme);
  root.setAttribute('data-variant', variant);
  root.setAttribute('lang', lang);
}

/** Reflect density onto <html> (drives --den-* tokens). Comfort clears the attr. */
function applyDensity(density: Density) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  if (density === 'comfort') root.removeAttribute('data-density');
  else root.setAttribute('data-density', density);
}

const DENSITY_CYCLE: Density[] = ['comfort', 'compact', 'spacious'];

// Legacy i18n key used by the pre-existing useI18n hook; default to FR per design.
const legacyLocale =
  (typeof localStorage !== 'undefined' && (localStorage.getItem('locale') as Lang)) || 'fr';

export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      theme: resolveTheme('system'),
      themeMode: 'system',
      variant: 'azure',
      lang: legacyLocale,
      density: 'comfort',
      sidebarCollapsed: false,
      cmdkOpen: false,

      // Choosing an explicit theme is also choosing to stop following the OS.
      setTheme: (theme) => get().setThemeMode(theme),

      setThemeMode: (themeMode) => {
        const theme = resolveTheme(themeMode);
        applyDom(theme, get().variant, get().lang);
        set({ theme, themeMode });
      },

      toggleTheme: () => {
        // Toggling from 'system' commits to the opposite of what is on screen,
        // which is what the user is asking for by flipping a visible switch.
        const theme: Theme = get().theme === 'dark' ? 'light' : 'dark';
        get().setThemeMode(theme);
      },
      setVariant: (variant) => {
        applyDom(get().theme, variant, get().lang);
        set({ variant });
      },
      setLang: (lang) => {
        if (typeof localStorage !== 'undefined') localStorage.setItem('locale', lang);
        applyDom(get().theme, get().variant, lang);
        set({ lang });
        // Keep any consumer listening on the legacy event in sync.
        window.dispatchEvent(new CustomEvent('locale-change', { detail: { locale: lang } }));
      },
      toggleLang: () => get().setLang(get().lang === 'fr' ? 'en' : 'fr'),
      setDensity: (density) => {
        applyDensity(density);
        set({ density });
      },
      cycleDensity: () => {
        const next =
          DENSITY_CYCLE[(DENSITY_CYCLE.indexOf(get().density) + 1) % DENSITY_CYCLE.length];
        applyDensity(next);
        set({ density: next });
      },
      toggleSidebar: () => set({ sidebarCollapsed: !get().sidebarCollapsed }),
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
      setCmdkOpen: (cmdkOpen) => set({ cmdkOpen }),
      toggleCmdk: () => set({ cmdkOpen: !get().cmdkOpen }),
    }),
    {
      name: 'openrisk-ui',
      partialize: (s) => ({
        themeMode: s.themeMode,
        variant: s.variant,
        lang: s.lang,
        density: s.density,
        sidebarCollapsed: s.sidebarCollapsed,
      }),
      onRehydrateStorage: () => (state) => {
        // Once persisted prefs are loaded, reflect them onto <html>.
        if (state) {
          // Re-resolve rather than trusting a stored resolved value: under
          // 'system' the OS may have changed since the last visit.
          state.theme = resolveTheme(state.themeMode ?? 'system');
          applyDom(state.theme, state.variant, state.lang);
          applyDensity(state.density);
        }
      },
    },
  ),
);

// Apply immediately on module load for the very first paint (before rehydrate runs).
applyDom(useUIStore.getState().theme, useUIStore.getState().variant, useUIStore.getState().lang);
applyDensity(useUIStore.getState().density);

// Keep following the OS while the user is on 'system'. Without this the theme
// would only track the OS at load time, so a machine switching to dark at
// sunset would leave the app light until the next reload.
if (typeof window !== 'undefined' && window.matchMedia) {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    const { themeMode, variant, lang } = useUIStore.getState();
    if (themeMode !== 'system') return;
    const theme = systemTheme();
    applyDom(theme, variant, lang);
    useUIStore.setState({ theme });
  });
}
