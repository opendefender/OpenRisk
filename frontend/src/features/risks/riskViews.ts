// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// What the register's URL says, and what the API is asked for (#913).
//
// URL keys (all shareable, see useTableState):
//   f.scope=hot|mine|review_due|accepted   the view tab  (absent = all)
//   f.criticality=critical,high            band pills
//   f.status=open                          status listbox
//   f.owner_id=<uuid>                      owner listbox
//   view=matrix                            Tableau / Matrice switch. This key
//                                          predates the tabs (the dashboard's
//                                          matrix link uses it), so it keeps
//                                          meaning "layout"; the tab is `scope`.
// The API's own `view=` parameter is only ever built here, from `scope`.

import type { TableState } from '../../shared/datatable';
import type { Criticality } from '../../shared/riskColors';
import type { BadgeIntent } from '../../shared/ds';
import type { components } from '../../types/openapi.generated';

export const RISK_VIEWS = [
  'all',
  'mine',
  'hot',
  'review_due',
  'accepted',
] as const satisfies readonly components['schemas']['RiskView'][];
export type RiskView = (typeof RISK_VIEWS)[number];

/** The URL facet that carries the tab. */
export const SCOPE_FACET = 'scope';

export const BANDS: readonly Criticality[] = ['critical', 'high', 'medium', 'low'];

export const RISK_STATUSES = ['open', 'in_progress', 'mitigated', 'accepted', 'closed'] as const;
export type RiskStatusValue = (typeof RISK_STATUSES)[number];

/** Rows fetched in matrix mode: the API's page cap (risk_handler: limit <= 200). */
export const MATRIX_LIMIT = 200;

/** Normalizes casing and refuses an unknown value: it falls back to "all". */
export function parseView(raw: string | null | undefined): RiskView {
  const v = (raw ?? '').trim().toLowerCase();
  return (RISK_VIEWS as readonly string[]).includes(v) ? (v as RiskView) : 'all';
}

export type RegisterLayout = 'table' | 'matrix';

export function parseLayout(raw: string | null | undefined): RegisterLayout {
  return (raw ?? '').trim().toLowerCase() === 'matrix' ? 'matrix' : 'table';
}

export function parseBands(values: readonly string[] | undefined): Criticality[] {
  const wanted = new Set((values ?? []).map((v) => v.trim().toLowerCase()));
  return BANDS.filter((b) => wanted.has(b));
}

export function parseStatus(values: readonly string[] | undefined): RiskStatusValue | '' {
  const v = (values?.[0] ?? '').trim().toLowerCase();
  return (RISK_STATUSES as readonly string[]).includes(v) ? (v as RiskStatusValue) : '';
}

/** Both live status vocabularies (open/in_progress… and DRAFT/ACTIVE…) → 5 values. */
export function toStatusValue(raw: string | null | undefined): RiskStatusValue {
  const v = (raw ?? '').toString().trim().toLowerCase();
  switch (v) {
    case 'in_progress':
    case 'progress':
    case 'active':
      return 'in_progress';
    case 'mitigated':
      return 'mitigated';
    case 'accepted':
      return 'accepted';
    case 'closed':
    case 'done':
      return 'closed';
    default:
      return 'open';
  }
}

export interface RegisterFilters {
  view: RiskView;
  bands: Criticality[];
  status: RiskStatusValue | '';
  ownerId: string;
}

export function readFilters(filters: TableState['filters']): RegisterFilters {
  return {
    view: parseView(filters[SCOPE_FACET]?.[0]),
    bands: parseBands(filters.criticality),
    status: parseStatus(filters.status),
    ownerId: filters.owner_id?.[0] ?? '',
  };
}

type Params = Record<string, string | number>;

/**
 * The query for GET /risks. Table mode pages server-side; matrix mode asks for
 * the API's largest page, since a matrix of one page of fifty would lie about
 * the register.
 */
export function buildRiskParams(state: TableState, layout: RegisterLayout): Params {
  const p: Params =
    layout === 'matrix'
      ? { page: 1, limit: MATRIX_LIMIT }
      : { page: state.page, limit: state.pageSize };
  if (state.q.trim()) p.q = state.q.trim();
  if (layout === 'matrix') {
    p.sort_by = 'score';
    p.sort_dir = 'desc';
  } else if (state.sort) {
    p.sort_by = state.sort.key;
    p.sort_dir = state.sort.dir;
  }
  for (const [key, values] of Object.entries(state.filters)) {
    if (key === SCOPE_FACET || !values.length) continue;
    p[key] = values.join(',');
  }
  const view = parseView(state.filters[SCOPE_FACET]?.[0]);
  if (view !== 'all') p.view = view;
  return p;
}

/** What GET /risks/summary is asked: the tab and the search, nothing else. */
export function buildSummaryParams(state: TableState): { view: RiskView; q: string } {
  return { view: parseView(state.filters[SCOPE_FACET]?.[0]), q: state.q.trim() };
}

export type ReviewState =
  | { kind: 'closed' }
  | { kind: 'none' }
  | { kind: 'overdue'; days: number }
  | { kind: 'soon'; days: number }
  | { kind: 'later'; days: number };

const DAY_MS = 86_400_000;

/**
 * Where a risk stands against its next review. Overdue is red, within 14 days
 * amber, as designed. `days` is whole calendar days from `now` (negative when late).
 */
export function reviewState(
  nextReviewAt: string | null | undefined,
  status: RiskStatusValue,
  now: Date,
): ReviewState {
  if (status === 'closed') return { kind: 'closed' };
  if (!nextReviewAt) return { kind: 'none' };
  const due = new Date(nextReviewAt).getTime();
  if (Number.isNaN(due)) return { kind: 'none' };
  const diff = due - now.getTime();
  // Late is decided on the instant, not on rounded days: a review due an hour
  // ago is overdue, whatever Math.ceil would make of -0.04.
  if (diff < 0) return { kind: 'overdue', days: Math.floor(diff / DAY_MS) };
  const days = Math.ceil(diff / DAY_MS);
  if (days <= 14) return { kind: 'soon', days };
  return { kind: 'later', days };
}

/**
 * The register's status colours, as designed: Ouvert and Clos neutral, En cours
 * blue, Atténué green, Accepté amber. (The ds `riskStatusIntent` colours Ouvert
 * as danger, which the register does not want next to its band pills.)
 */
export const registerStatusIntent: Record<RiskStatusValue, BadgeIntent> = {
  open: 'neutral',
  in_progress: 'info',
  mitigated: 'success',
  accepted: 'warning',
  closed: 'neutral',
};
