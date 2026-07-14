// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Grouped navigation model (OpenRisk.dc.html §5). Single source of truth shared
// by the Sidebar and the ⌘K command palette. `labelKey`/`groupKey` index into
// uiStrings so labels stay FR/EN reactive. Screens without a backend yet route to
// the ComingSoon placeholder so the shell never dead-ends.

import {
  LayoutDashboard, TrendingUp, Trophy, ShieldAlert, ShieldCheck, Siren, Server,
  ClipboardCheck, Globe, Cpu, Database, Atom, FileText, Sparkles, Settings,
  type LucideIcon,
} from 'lucide-react';
import type { UIStrings } from './uiStrings';

export interface NavItem {
  key: string;
  labelKey: keyof UIStrings;
  icon: LucideIcon;
  path: string;
  badge?: { text: string; color?: string };
  /** Placeholder screen (no backend yet). */
  soon?: boolean;
}

export interface NavGroup {
  groupKey: keyof UIStrings;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    groupKey: 'g_overview',
    items: [
      { key: 'dashboard', labelKey: 'n_dashboard', icon: LayoutDashboard, path: '/' },
      { key: 'analytics', labelKey: 'n_analytics', icon: TrendingUp, path: '/analytics' },
      { key: 'leaderboard', labelKey: 'n_leaderboard', icon: Trophy, path: '/leaderboard', soon: true },
    ],
  },
  {
    groupKey: 'g_security',
    items: [
      { key: 'risks', labelKey: 'n_risks', icon: ShieldAlert, path: '/risks', badge: { text: '12' } },
      { key: 'mitigations', labelKey: 'n_mitigations', icon: ShieldCheck, path: '/mitigations', badge: { text: '3', color: 'var(--high)' } },
      { key: 'incidents', labelKey: 'n_incidents', icon: Siren, path: '/incidents' },
      { key: 'infrastructure', labelKey: 'n_infra', icon: Server, path: '/infrastructure', soon: true },
    ],
  },
  {
    groupKey: 'g_intel',
    items: [
      { key: 'compliance', labelKey: 'n_compliance', icon: ClipboardCheck, path: '/compliance' },
      { key: 'cti', labelKey: 'n_cti', icon: Globe, path: '/threat-map' },
      { key: 'simulations', labelKey: 'n_simulations', icon: Cpu, path: '/simulations', soon: true },
    ],
  },
  {
    groupKey: 'g_assets',
    items: [
      { key: 'assets', labelKey: 'n_assets', icon: Database, path: '/assets' },
      { key: 'universe', labelKey: 'n_universe', icon: Atom, path: '/assets/universe', soon: true },
    ],
  },
  {
    groupKey: 'g_report',
    items: [
      { key: 'reports', labelKey: 'n_reports', icon: FileText, path: '/reports' },
      { key: 'ai', labelKey: 'n_ai', icon: Sparkles, path: '/recommendations' },
    ],
  },
  {
    groupKey: 'g_admin',
    items: [{ key: 'settings', labelKey: 'n_settings', icon: Settings, path: '/settings' }],
  },
];

/** Flat list of all nav items, for the command palette. */
export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);
