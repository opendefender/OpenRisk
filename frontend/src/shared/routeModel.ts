// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The route tree — one source of truth for the sitemap.
//
// Three things used to disagree about the shape of the app: the <Routes> table,
// the sidebar nav model, and whatever ad-hoc "← back" button each screen chose
// to render. That disagreement is what let a user reach Gap Analysis, Audits or
// Remediation with no way back, and what let Reports and Compliance point at
// each other forever. Declaring the tree once, with parents, makes the trail
// derivable instead of hand-written, so a page cannot be added without one.
//
// Rules encoded here:
//   - every sub-view is a route, never component state (so Back works and links
//     are shareable);
//   - every node at depth >= 2 has a `parent`, which is what the breadcrumb
//     walks and what "back" falls back to when there is no history to pop;
//   - `perm` is the permission the route needs, named in the access-denied
//     screen so a refusal says which grant is missing.

import type { UIStrings } from './uiStrings';

export interface RouteNode {
  /** react-router pattern, e.g. '/compliance/frameworks/:frameworkId/gaps'. */
  path: string;
  /** Label from the i18n table. Mutually exclusive with `label`. */
  labelKey?: keyof UIStrings;
  /** Literal label for nodes with no nav entry. */
  label?: { fr: string; en: string };
  /** Parent pattern. Absent only for depth-1 nodes. */
  parent?: string;
  /** Permission required to view. Named verbatim on the access-denied screen. */
  perm?: string;
  /**
   * Renders the label for a matched instance, e.g. "ISO 27001" instead of
   * "Framework". Falls back to the static label when the resolver returns null.
   */
  dynamic?: boolean;
  /**
   * A destination reachable directly from the sidebar, whose URL merely happens
   * to be nested (/analytics/financial, /ai/emerging-risks). It is not a
   * drill-down, so it needs no parent and is not a dead end without one.
   * Everything else at depth >= 2 must declare a parent — that invariant is
   * what guarantees a way back, and it is enforced by a test.
   */
  topLevel?: boolean;
}

/**
 * The tree. Order does not matter; matching picks the most specific pattern.
 */
export const ROUTES: RouteNode[] = [
  /* ---------------- Dashboard ---------------- */
  { path: '/', labelKey: 'n_dashboard' },

  /* ---------------- Risks ---------------- */
  { path: '/risks', labelKey: 'n_risks', perm: 'risks:read' },
  { path: '/risks/import', label: { fr: 'Importer', en: 'Import' }, parent: '/risks', perm: 'risks:create' },
  { path: '/risks/weighting', label: { fr: 'Pondération', en: 'Weighting' }, parent: '/risks', perm: 'risks:read' },
  { path: '/risks/:riskId/timeline', label: { fr: 'Chronologie', en: 'Timeline' }, parent: '/risks', perm: 'risks:read' },
  // Mitigations live under Risks: a mitigation only exists to reduce a risk, and
  // filing it anywhere else is what made "back" ambiguous from its detail view.
  { path: '/risks/mitigations', labelKey: 'n_mitigations', parent: '/risks', perm: 'mitigations:read' },
  { path: '/risks/mitigations/:mitigationId', label: { fr: 'Plan', en: 'Plan' }, parent: '/risks/mitigations', perm: 'mitigations:read', dynamic: true },

  /* ---------------- Vulnerabilities / threats ---------------- */
  { path: '/vulnerabilities', labelKey: 'n_vulns', perm: 'vulnerabilities:read' },
  { path: '/threat-map', labelKey: 'n_cti', perm: 'risks:read' },
  { path: '/ai/emerging-risks', labelKey: 'n_emerging', perm: 'risks:read', topLevel: true },
  { path: '/simulations', labelKey: 'n_simulations', perm: 'risks:read' },

  /* ---------------- Incidents ---------------- */
  { path: '/incidents', labelKey: 'n_incidents', perm: 'incidents:read' },
  { path: '/incidents/:id/war-room', label: { fr: 'War Room', en: 'War Room' }, parent: '/incidents', perm: 'incidents:read' },

  /* ---------------- Compliance ---------------- */
  { path: '/compliance', labelKey: 'n_compliance', perm: 'compliance:frameworks:read' },
  { path: '/compliance/frameworks/:frameworkId', label: { fr: 'Référentiel', en: 'Framework' }, parent: '/compliance', perm: 'compliance:controls:read', dynamic: true },
  { path: '/compliance/frameworks/:frameworkId/gaps', label: { fr: 'Écarts', en: 'Gaps' }, parent: '/compliance/frameworks/:frameworkId', perm: 'compliance:controls:read' },
  { path: '/compliance/gaps', label: { fr: "Analyse d'écarts", en: 'Gap analysis' }, parent: '/compliance', perm: 'compliance:controls:read' },
  { path: '/compliance/evidence', label: { fr: 'Bibliothèque de preuves', en: 'Evidence library' }, parent: '/compliance', perm: 'compliance:evidences:read' },
  { path: '/compliance/evidence/missing', label: { fr: 'Preuves manquantes', en: 'Missing evidence' }, parent: '/compliance/evidence', perm: 'compliance:evidences:read' },
  { path: '/compliance/audits', label: { fr: 'Audits', en: 'Audits' }, parent: '/compliance', perm: 'compliance:audits:read' },
  { path: '/compliance/audits/:auditId', label: { fr: 'Audit', en: 'Audit' }, parent: '/compliance/audits', perm: 'compliance:audits:read', dynamic: true },
  { path: '/compliance/remediation', label: { fr: 'Plans de remédiation', en: 'Remediation plans' }, parent: '/compliance', perm: 'compliance:remediations:read' },
  { path: '/compliance/remediation/:planId', label: { fr: 'Plan', en: 'Plan' }, parent: '/compliance/remediation', perm: 'compliance:remediations:read', dynamic: true },

  /* ---------------- Assets ---------------- */
  { path: '/assets', labelKey: 'n_assets', perm: 'assets:read' },
  { path: '/assets/topology', labelKey: 'n_universe', parent: '/assets', perm: 'assets:read' },
  { path: '/assets/schemas', labelKey: 'n_assetSchemas', parent: '/assets', perm: 'assets:read' },
  { path: '/infrastructure', labelKey: 'n_infra', perm: 'scanner:read' },
  { path: '/infrastructure/scans/:jobId', label: { fr: 'Aperçu du scan', en: 'Scan preview' }, parent: '/infrastructure', perm: 'scanner:read' },

  /* ---------------- Analytics ---------------- */
  // The executive view is a display mode of the dashboard (?view=executive), not
  // a report. It answers "how are we doing", which is the dashboard's question;
  // filing it under Reports is what made people look for it beside PDFs.
  { path: '/analytics/financial', labelKey: 'n_financial', perm: 'risks:read', topLevel: true },
  { path: '/leaderboard', labelKey: 'n_leaderboard' },

  /* ---------------- Reports ---------------- */
  { path: '/reports', labelKey: 'n_reports', perm: 'compliance:controls:read' },
  { path: '/reports/library', label: { fr: 'Bibliothèque de rapports', en: 'Report library' }, parent: '/reports', perm: 'compliance:controls:read' },
  { path: '/reports/jobs/:jobId', label: { fr: 'Rapport généré', en: 'Generated report' }, parent: '/reports', perm: 'compliance:controls:read', dynamic: true },
  { path: '/reports/board', label: { fr: 'Rapports Conseil', en: 'Board reports' }, parent: '/reports', perm: 'reports:board:read' },
  { path: '/reports/:reportId', label: { fr: 'Rapport', en: 'Report' }, parent: '/reports/board', perm: 'reports:board:read', dynamic: true },
  { path: '/recommendations', labelKey: 'n_ai', perm: 'risks:read' },

  /* ---------------- Automation / governance ---------------- */
  { path: '/automation', labelKey: 'n_automation', perm: 'automation:read' },
  { path: '/governance', labelKey: 'n_governance' },
  { path: '/governance/audit-trail', label: { fr: "Piste d'audit", en: 'Audit trail' }, parent: '/governance' },

  /* ---------------- Settings ---------------- */
  { path: '/settings', labelKey: 'n_settings' },
  // Members owns invitations AND role assignment: they are one job ("give this
  // person the right access"), and splitting them across two screens is why
  // "Invite a member" used to land on Roles & permissions.
  { path: '/settings/members', label: { fr: 'Membres', en: 'Members' }, parent: '/settings' },
];

/* ------------------------------------------------------------------ *
 * Matching
 * ------------------------------------------------------------------ */

/** Segment-wise match of a concrete pathname against a route pattern. */
export function matchPath(pattern: string, pathname: string): Record<string, string> | null {
  const pa = pattern.split('/').filter(Boolean);
  const pb = pathname.split('/').filter(Boolean);
  if (pa.length !== pb.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < pa.length; i++) {
    if (pa[i].startsWith(':')) {
      if (!pb[i]) return null;
      params[pa[i].slice(1)] = decodeURIComponent(pb[i]);
    } else if (pa[i] !== pb[i]) {
      return null;
    }
  }
  return params;
}

/** Specificity: more static segments wins, so '/reports/board' beats '/reports/:reportId'. */
function specificity(pattern: string): number {
  return pattern
    .split('/')
    .filter(Boolean)
    .reduce((n, seg) => n + (seg.startsWith(':') ? 1 : 10), 0);
}

export interface MatchedRoute {
  node: RouteNode;
  params: Record<string, string>;
}

/** Finds the most specific route matching a pathname. */
export function resolveRoute(pathname: string): MatchedRoute | null {
  let best: MatchedRoute | null = null;
  let bestScore = -1;
  for (const node of ROUTES) {
    const params = matchPath(node.path, pathname);
    if (!params) continue;
    const score = specificity(node.path);
    if (score > bestScore) {
      best = { node, params };
      bestScore = score;
    }
  }
  return best;
}

/**
 * The ancestor chain for a pathname, root first, matched node last.
 *
 * Parent patterns are filled from the matched node's own params, so
 * /compliance/frameworks/abc/gaps yields a trail whose Framework link points at
 * /compliance/frameworks/abc rather than at the raw pattern.
 */
export function routeTrail(pathname: string): Array<{ node: RouteNode; href: string; params: Record<string, string> }> {
  const matched = resolveRoute(pathname);
  if (!matched) return [];

  const chain: RouteNode[] = [];
  let cursor: RouteNode | undefined = matched.node;
  const guard = new Set<string>();
  while (cursor && !guard.has(cursor.path)) {
    guard.add(cursor.path);
    chain.unshift(cursor);
    cursor = cursor.parent ? ROUTES.find((r) => r.path === cursor!.parent) : undefined;
  }

  return chain.map((node) => ({
    node,
    params: matched.params,
    href: fillPattern(node.path, matched.params),
  }));
}

/** Substitutes :params into a pattern. */
export function fillPattern(pattern: string, params: Record<string, string>): string {
  return (
    '/' +
    pattern
      .split('/')
      .filter(Boolean)
      .map((seg) => (seg.startsWith(':') ? (params[seg.slice(1)] ?? seg) : seg))
      .join('/')
  );
}

/** The permission a pathname requires, or undefined when it is unguarded. */
export function permissionFor(pathname: string): string | undefined {
  return resolveRoute(pathname)?.node.perm;
}

/**
 * The parent URL for a pathname — where "back" goes when there is no history to
 * pop (a deep link opened in a fresh tab). Every depth >= 2 route has one, which
 * is the structural guarantee that no screen is a dead end.
 */
export function parentHref(pathname: string): string | null {
  const trail = routeTrail(pathname);
  if (trail.length < 2) return null;
  return trail[trail.length - 2].href;
}
