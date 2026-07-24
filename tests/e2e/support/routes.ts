// The complete route inventory, derived from frontend/src/App.tsx and
// frontend/src/shared/navModel.ts. smoke.routes.spec.ts drives every entry;
// docs/UX_AUDIT_2026-07.md mirrors this table.

import fs from 'node:fs';
import { SEED_IDS_FILE } from './env';

export interface RouteDef {
  /** Path to visit (relative — Playwright baseURL applies). */
  path: string;
  /** Human name for the report row. */
  name: string;
  /** navModel key, when this route has a sidebar item. */
  nav?: string;
  /** Marked `soon: true` in navModel (placeholder / partial backend). */
  soon?: boolean;
  /** 'static' | 'param' | 'redirect'. */
  kind: 'static' | 'param' | 'redirect';
  /** For redirects: expected destination path prefix. */
  redirectsTo?: string;
}

export interface SeedIds {
  frameworkId?: string;
  riskId?: string;
  incidentId?: string;
  personas?: Record<string, { usable: boolean; reason?: string; email?: string }>;
  counts?: Record<string, number>;
}

export function readSeedIds(): SeedIds {
  try {
    return JSON.parse(fs.readFileSync(SEED_IDS_FILE, 'utf8')) as SeedIds;
  } catch {
    return {};
  }
}

/** Protected, non-parameterised screens — the backbone of the smoke run. */
export const STATIC_ROUTES: RouteDef[] = [
  { path: '/', name: 'Dashboard', nav: 'dashboard', kind: 'static' },
  { path: '/analytics', name: 'Executive dashboard', nav: 'analytics', kind: 'static' },
  { path: '/analytics/financial', name: 'Financial quantification', nav: 'financial', kind: 'static' },
  { path: '/leaderboard', name: 'Leaderboard', nav: 'leaderboard', soon: true, kind: 'static' },
  { path: '/risks', name: 'Risk register', nav: 'risks', kind: 'static' },
  { path: '/risks/import', name: 'Import risks', kind: 'static' },
  { path: '/risks/weighting', name: 'Smart-risk weighting', kind: 'static' },
  { path: '/vulnerabilities', name: 'Vulnerabilities', nav: 'vulnerabilities', kind: 'static' },
  { path: '/mitigations', name: 'Mitigations board', nav: 'mitigations', kind: 'static' },
  { path: '/incidents', name: 'Incidents register', nav: 'incidents', kind: 'static' },
  { path: '/automation', name: 'Automation / SOAR', nav: 'automation', kind: 'static' },
  { path: '/infrastructure', name: 'Infrastructure scanner', nav: 'infrastructure', soon: true, kind: 'static' },
  { path: '/compliance', name: 'Compliance', nav: 'compliance', kind: 'static' },
  { path: '/compliance/gap-analysis', name: 'Gap analysis', kind: 'static' },
  { path: '/compliance/audits', name: 'Compliance audits', kind: 'static' },
  { path: '/compliance/remediations', name: 'Remediation plans', kind: 'static' },
  { path: '/threat-map', name: 'Threat intel (CTI)', nav: 'cti', kind: 'static' },
  { path: '/simulations', name: 'Simulations', nav: 'simulations', soon: true, kind: 'static' },
  { path: '/assets', name: 'Asset inventory', nav: 'assets', kind: 'static' },
  { path: '/assets/universe', name: 'Asset universe', nav: 'universe', soon: true, kind: 'static' },
  { path: '/reports', name: 'Reports', nav: 'reports', kind: 'static' },
  { path: '/reports/board', name: 'Board report', kind: 'static' },
  { path: '/recommendations', name: 'AI advisor', nav: 'ai', kind: 'static' },
  { path: '/ai/emerging-risks', name: 'Emerging risks (AI)', nav: 'emerging', kind: 'static' },
  { path: '/governance', name: 'Governance', nav: 'governance', kind: 'static' },
  { path: '/settings', name: 'Settings', nav: 'settings', kind: 'static' },
  { path: '/settings/roles', name: 'Roles & access', nav: 'roles', kind: 'static' },
];

/** Parameterised screens, resolved from seeded IDs. */
export function paramRoutes(seed: SeedIds): RouteDef[] {
  const fw = seed.frameworkId;
  const risk = seed.riskId;
  const inc = seed.incidentId;
  const routes: RouteDef[] = [];
  if (fw) routes.push({ path: `/compliance/${fw}`, name: 'Framework detail', kind: 'param' });
  if (risk) routes.push({ path: `/risks/${risk}/timeline`, name: 'Risk timeline', kind: 'param' });
  if (inc) routes.push({ path: `/incidents/${inc}/war-room`, name: 'Incident war room', kind: 'param' });
  // No cheap seedable scan job — a synthetic UUID exercises the empty/error state.
  routes.push({
    path: '/infrastructure/scans/00000000-0000-0000-0000-000000000000',
    name: 'Scan preview (empty)',
    kind: 'param',
  });
  return routes;
}

/** Redirect routes — App.tsx folds legacy paths into Settings / Risks. */
export const REDIRECT_ROUTES: RouteDef[] = [
  { path: '/users', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/roles', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/tenants', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/audit-logs', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/tokens', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/marketplace', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/custom-fields', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/analytics/permissions', name: '→ Settings', kind: 'redirect', redirectsTo: '/settings' },
  { path: '/risk-management', name: '→ Risks', kind: 'redirect', redirectsTo: '/risks' },
  { path: '/bulk-operations', name: '→ Risks', kind: 'redirect', redirectsTo: '/risks' },
  { path: '/this-route-does-not-exist', name: 'Catch-all → Dashboard', kind: 'redirect', redirectsTo: '/' },
];
