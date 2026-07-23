// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Grouped navigation model (OpenRisk.dc.html §5). Single source of truth shared
// by the Sidebar and the ⌘K command palette. `labelKey`/`groupKey` index into
// uiStrings so labels stay FR/EN reactive. Screens without a backend yet route to
// the ComingSoon placeholder so the shell never dead-ends.

import {
  LayoutDashboard, TrendingUp, Trophy, ShieldAlert, ShieldCheck, Siren, Server,
  ClipboardCheck, Globe, Cpu, Database, Atom, FileText, Sparkles, Settings, Bug, Coins,
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
  /** Hoisted above the grouped nav as a standalone entry (e.g. Dashboard).
   *  Still lives inside its group for routing/palette/breadcrumb lookups. */
  pinned?: boolean;
}

export interface NavGroup {
  groupKey: keyof UIStrings;
  items: NavItem[];
  /** The product's core intention — visually emphasised so the user always
   *  knows where the primary job (reduce risk) lives. */
  core?: boolean;
}

// Navigation grouped by INTENTION (what the user is trying to accomplish), not by
// technical domain. Order reflects the natural GRC flow toward the core job. The
// core intention — "Maîtriser les risques" (identify → score → treat → prove) — is
// the product's reason to exist, so it leads and is visually emphasised (`core`).
// Dashboard is `pinned`: it stays inside its group for routing/palette lookups but
// the Sidebar hoists it to a standalone entry at the very top.
export const NAV_GROUPS: NavGroup[] = [
  {
    // ⭐ The core loop: what could hurt us, how bad, and what we do about it.
    groupKey: 'g_reduce',
    core: true,
    items: [
      { key: 'risks', labelKey: 'n_risks', icon: ShieldAlert, path: '/risks', badge: { text: '12' }, perm: 'risks:read' },
      { key: 'vulnerabilities', labelKey: 'n_vulns', icon: Bug, path: '/vulnerabilities', perm: 'vulnerabilities:read' },
      { key: 'mitigations', labelKey: 'n_mitigations', icon: ShieldCheck, path: '/mitigations', badge: { text: '3', color: 'var(--high)' }, perm: 'mitigations:read' },
      { key: 'incidents', labelKey: 'n_incidents', icon: Siren, path: '/incidents', perm: 'incidents:read' },
      { key: 'automation', labelKey: 'n_automation', icon: Workflow, path: '/automation', perm: 'automation:read' },
    ],
  },
  {
    // Where do I stand? — the high-level read on posture and exposure.
    groupKey: 'g_monitor',
    items: [
      { key: 'dashboard', labelKey: 'n_dashboard', icon: LayoutDashboard, path: '/', pinned: true },
      { key: 'analytics', labelKey: 'n_analytics', icon: TrendingUp, path: '/analytics', perm: 'risks:read' },
      { key: 'financial', labelKey: 'n_financial', icon: Coins, path: '/analytics/financial', perm: 'risks:read' },
      { key: 'leaderboard', labelKey: 'n_leaderboard', icon: Trophy, path: '/leaderboard', soon: true },
    ],
  },
  {
    // What I must protect — the estate that generates risk.
    groupKey: 'g_estate',
    items: [
      { key: 'assets', labelKey: 'n_assets', icon: Database, path: '/assets', perm: 'assets:read' },
      { key: 'universe', labelKey: 'n_universe', icon: Atom, path: '/assets/universe', soon: true, perm: 'assets:read' },
      { key: 'infrastructure', labelKey: 'n_infra', icon: Server, path: '/infrastructure', soon: true, perm: 'scanner:read' },
    ],
  },
  {
    // What's coming — the threats that feed the risk register.
    groupKey: 'g_threats',
    items: [
      { key: 'cti', labelKey: 'n_cti', icon: Globe, path: '/threat-map', perm: 'risks:read' },
      { key: 'emerging', labelKey: 'n_emerging', icon: Sparkles, path: '/ai/emerging-risks', perm: 'risks:read' },
      { key: 'simulations', labelKey: 'n_simulations', icon: Cpu, path: '/simulations', soon: true },
    ],
  },
  {
    // Show your work — controls, audits, approvals.
    groupKey: 'g_prove',
    items: [
      { key: 'compliance', labelKey: 'n_compliance', icon: ClipboardCheck, path: '/compliance', perm: 'compliance:read' },
      { key: 'governance', labelKey: 'n_governance', icon: Scale, path: '/governance', adminOnly: true },
    ],
  },
  {
    // Communicate & decide.
    groupKey: 'g_decide',
    items: [
      { key: 'reports', labelKey: 'n_reports', icon: FileText, path: '/reports', perm: 'reports:board:read' },
      { key: 'ai', labelKey: 'n_ai', icon: Sparkles, path: '/recommendations', perm: 'risks:read' },
    ],
  },
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

/** Items hoisted above the grouped nav (e.g. Dashboard), in the given (already
 *  permission-filtered) groups. The Sidebar renders these first, then renders the
 *  groups with their pinned items removed so nothing shows twice. */
export function pinnedItems(groups: NavGroup[]): NavItem[] {
  return groups.flatMap((g) => g.items.filter((i) => i.pinned));
}

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
