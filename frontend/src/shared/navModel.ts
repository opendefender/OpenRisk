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
  FolderCheck,
  LayoutDashboard, TrendingUp, ShieldAlert, ShieldCheck, Siren, Server,
  ClipboardCheck, Globe, Database, Atom, FileText, Sparkles, Settings, Bug, Coins,
  Workflow, Scale, Users, History, ListChecks,
  type LucideIcon,
} from 'lucide-react';
import type { UIStrings } from './uiStrings';

/** The named live counters a nav item may display. Each maps to a real
 *  tenant-scoped field the Sidebar resolves; there is no free-text option, so
 *  a badge cannot be invented. */
export type NavCount = 'pending_invitations';

export interface NavItem {
  key: string;
  labelKey: keyof UIStrings;
  icon: LucideIcon;
  /** The pathname this item represents, used for active-state matching. */
  path: string;
  /**
   * Where clicking actually goes, when that differs from `path`. The executive
   * view is a display mode of the dashboard (/?view=executive), so it navigates
   * to a query string while still matching on "/" — keeping the query out of
   * `path` is what stops `pathname === path` failing for every such item.
   */
  href?: string;
  /**
   * Disambiguates two items sharing a pathname: Dashboard and Executive both
   * live at "/", and are told apart by ?view=. An item with `view` is active
   * only when the param matches; an item without one is active only when the
   * param is absent.
   */
  view?: string;
  /**
   * A live counter for this item, resolved at render time from a real
   * tenant-scoped source. It names WHICH count it wants, so an item cannot
   * carry a number nobody computes.
   *
   * This replaces two literal strings — '12' on Risks and '3' on Mitigations —
   * that were rendered to every tenant regardless of what they actually held.
   * A counter is a claim about the user's data; there is no honest way to write
   * one as a constant.
   */
  badge?: { count: NavCount; color?: string };
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
  // 0 · Piloter — « Où en suis-je ? » (dashboard par rôle, exécutif, financier)
  {
    groupKey: 'g_pilot',
    items: [
      { key: 'risks', labelKey: 'n_risks', icon: ShieldAlert, path: '/risks', perm: 'risks:read' },
      { key: 'vulnerabilities', labelKey: 'n_vulns', icon: Bug, path: '/vulnerabilities', perm: 'vulnerabilities:read' },
      { key: 'mitigations', labelKey: 'n_mitigations', icon: ShieldCheck, path: '/risks/mitigations', perm: 'mitigations:read' },
      { key: 'incidents', labelKey: 'n_incidents', icon: Siren, path: '/incidents', perm: 'incidents:read' },
      { key: 'automation', labelKey: 'n_automation', icon: Workflow, path: '/automation', perm: 'automation:read' },
    ],
  },
  {
    // Where do I stand? — the high-level read on posture and exposure.
    groupKey: 'g_monitor',
    items: [
      { key: 'dashboard', labelKey: 'n_dashboard', icon: LayoutDashboard, path: '/', pinned: true },
      // No `perm`: the endpoint scopes itself to what the caller's business role
      // can act on, so an empty list is the honest answer for a role with nothing
      // outstanding — not a reason to hide the entry.
      { key: 'action-center', labelKey: 'n_actionCenter', icon: ListChecks, path: '/action-center' },
      { key: 'analytics', labelKey: 'n_analytics', icon: TrendingUp, path: '/', href: '/?view=executive', view: 'executive', perm: 'risks:read' },
      { key: 'financial', labelKey: 'n_financial', icon: Coins, path: '/analytics/financial', perm: 'risks:read' },
      { key: 'activity', labelKey: 'n_activity', icon: History, path: '/activity' },
    ],
  },
  // 1 · Identifier — « Qu'est-ce que je possède et qu'est-ce qui me menace ? »
  {
    groupKey: 'g_identify',
    items: [
      { key: 'assets', labelKey: 'n_assets', icon: Database, path: '/assets', perm: 'assets:read' },
      { key: 'universe', labelKey: 'n_universe', icon: Atom, path: '/assets/topology', perm: 'assets:read' },
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
      { key: 'mitigations', labelKey: 'n_mitigations', icon: ShieldCheck, path: '/risks/mitigations', perm: 'mitigations:read' },
      { key: 'incidents', labelKey: 'n_incidents', icon: Siren, path: '/incidents', perm: 'incidents:read' },
      { key: 'automation', labelKey: 'n_automation', icon: Workflow, path: '/automation', perm: 'automation:read' },
    ],
  },
  // 4 · Prouver — « Comment je le démontre à un régulateur ? »
  {
    groupKey: 'g_prove',
    items: [
      { key: 'compliance', labelKey: 'n_compliance', icon: ClipboardCheck, path: '/compliance', perm: 'compliance:read' },
      // The evidence library is its own destination, not a tab inside a
      // framework: the same artifact answers controls in several frameworks, so
      // filing it under one of them would hide it from the others.
      { key: 'evidence', labelKey: 'n_evidence', icon: FolderCheck, path: '/compliance/evidence', perm: 'compliance:evidences:read' },
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
      // The members badge counts OUTSTANDING INVITATIONS, not members: a number
      // beside a nav item should mean "something is waiting for you", and the
      // headcount is not.
      { key: 'roles', labelKey: 'n_roles', icon: Users, path: '/settings/members', adminOnly: true, badge: { count: 'pending_invitations', color: 'var(--info)' } },
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
  executive: '/?view=executive',
  viewer: '/',
};

/** Landing route for a business role key ('/' for admins / unknown roles). */
export function landingForBusinessRole(businessRole?: string): string {
  if (!businessRole) return '/';
  return BUSINESS_ROLE_LANDING[businessRole] ?? '/';
}
