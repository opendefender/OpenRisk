// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Grouped navigation model (OpenRisk.dc.html §5). Single source of truth shared
// by the Sidebar and the ⌘K command palette. `labelKey`/`groupKey` index into
// uiStrings so labels stay FR/EN reactive.
//
// Information architecture: grouped by the user's GRC intention, in the natural
// order of the work — Piloter → Identifier → Évaluer → Traiter → Prouver — plus a
// utility group (see docs/IA_NAVIGATION_PROPOSAL.md, ratified 2026-07-24). Genuine
// placeholders (leaderboard, simulations) are withheld from the sidebar rather than
// shown as empty promises; their routes still exist for progressive disclosure.

import {
  LayoutDashboard, TrendingUp, ShieldAlert, ShieldCheck, Siren, Server,
  ClipboardCheck, Globe, Database, Atom, FileText, Sparkles, Settings, Bug, Coins,
  Workflow, Scale, Users,
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
  /** Required permission to see this item. Mirrors the route guard on the
   *  backend so the menu never offers a screen the API would 403. Omitted =
   *  visible to any authenticated member. */
  perm?: string;
  /** Only visible to org admins/root (governance, tenant administration). */
  adminOnly?: boolean;
}

export interface NavGroup {
  groupKey: keyof UIStrings;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  // 0 · Piloter — « Où en suis-je ? » (dashboard par rôle, exécutif, financier)
  {
    groupKey: 'g_pilot',
    items: [
      { key: 'dashboard', labelKey: 'n_dashboard', icon: LayoutDashboard, path: '/' },
      { key: 'analytics', labelKey: 'n_analytics', icon: TrendingUp, path: '/analytics', perm: 'risks:read' },
      { key: 'financial', labelKey: 'n_financial', icon: Coins, path: '/analytics/financial', perm: 'risks:read' },
    ],
  },
  // 1 · Identifier — « Qu'est-ce que je possède et qu'est-ce qui me menace ? »
  {
    groupKey: 'g_identify',
    items: [
      { key: 'assets', labelKey: 'n_assets', icon: Database, path: '/assets', perm: 'assets:read' },
      { key: 'universe', labelKey: 'n_universe', icon: Atom, path: '/assets/universe', perm: 'assets:read' },
      { key: 'vulnerabilities', labelKey: 'n_vulns', icon: Bug, path: '/vulnerabilities', perm: 'vulnerabilities:read' },
      { key: 'cti', labelKey: 'n_cti', icon: Globe, path: '/threat-map', perm: 'risks:read' },
      { key: 'infrastructure', labelKey: 'n_infra', icon: Server, path: '/infrastructure', perm: 'scanner:read' },
    ],
  },
  // 2 · Évaluer — « Quel est mon risque, en clair et en argent ? »
  {
    groupKey: 'g_evaluate',
    items: [
      { key: 'risks', labelKey: 'n_risks', icon: ShieldAlert, path: '/risks', perm: 'risks:read' },
    ],
  },
  // 3 · Traiter — « Que fais-je pour réduire ? »
  {
    groupKey: 'g_treat',
    items: [
      { key: 'mitigations', labelKey: 'n_mitigations', icon: ShieldCheck, path: '/mitigations', perm: 'mitigations:read' },
      { key: 'incidents', labelKey: 'n_incidents', icon: Siren, path: '/incidents', perm: 'incidents:read' },
      { key: 'automation', labelKey: 'n_automation', icon: Workflow, path: '/automation', perm: 'automation:read' },
    ],
  },
  // 4 · Prouver — « Comment je le démontre à un régulateur ? »
  {
    groupKey: 'g_prove',
    items: [
      { key: 'compliance', labelKey: 'n_compliance', icon: ClipboardCheck, path: '/compliance', perm: 'compliance:read' },
      { key: 'reports', labelKey: 'n_reports', icon: FileText, path: '/reports', perm: 'reports:board:read' },
      { key: 'ai', labelKey: 'n_ai', icon: Sparkles, path: '/recommendations', perm: 'risks:read' },
      { key: 'emerging', labelKey: 'n_emerging', icon: Sparkles, path: '/ai/emerging-risks', perm: 'risks:read' },
      { key: 'governance', labelKey: 'n_governance', icon: Scale, path: '/governance', adminOnly: true },
    ],
  },
  // Utility — hors des 5 intentions (bas de sidebar)
  {
    groupKey: 'g_admin',
    items: [
      { key: 'roles', labelKey: 'n_roles', icon: Users, path: '/settings/roles', adminOnly: true },
      { key: 'settings', labelKey: 'n_settings', icon: Settings, path: '/settings' },
    ],
  },
];

/** Flat list of all nav items, for the command palette. */
export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);

/**
 * Filter the nav to what a member may actually reach, given a permission check
 * and admin flag. An item is shown when it has no gate, when the user has its
 * `perm`, or (for adminOnly items) when the user is an admin. Empty groups are
 * dropped so the sidebar never renders a header with nothing under it.
 */
export function visibleNavGroups(
  can: (perm: string) => boolean,
  isAdmin: boolean
): NavGroup[] {
  const allow = (it: NavItem): boolean => {
    if (it.adminOnly) return isAdmin;
    if (it.perm) return isAdmin || can(it.perm);
    return true;
  };
  return NAV_GROUPS.map((g) => ({ ...g, items: g.items.filter(allow) })).filter(
    (g) => g.items.length > 0
  );
}

/**
 * Post-login landing route for each GRC business role, mirroring the backend
 * domain.DefaultLandingFor so each profession opens on a relevant screen.
 */
const BUSINESS_ROLE_LANDING: Record<string, string> = {
  rssi: '/',
  dsi: '/assets',
  risk_manager: '/risks',
  auditor: '/compliance',
  compliance_officer: '/compliance',
  internal_control: '/compliance',
  asset_owner: '/assets',
  risk_owner: '/risks',
  security_analyst: '/vulnerabilities',
  executive: '/analytics',
  viewer: '/',
};

/** Landing route for a business role key ('/' for admins / unknown roles). */
export function landingForBusinessRole(businessRole?: string): string {
  if (!businessRole) return '/';
  return BUSINESS_ROLE_LANDING[businessRole] ?? '/';
}
