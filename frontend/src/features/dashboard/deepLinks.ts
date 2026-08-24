// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Where a dashboard tile takes you, and what it takes with it.
//
// THE PROBLEM THIS SOLVES. Every KPI on every persona used to navigate to a bare
// list route. Clicking "Critical — 3" landed on an unfiltered register of
// everything, and the user had to rebuild by hand the filter they had just
// expressed by clicking a tile labelled with it. The destinations were never the
// obstacle: the register, the vulnerability list and the inventory have all read
// their filters from the URL since the DataTable migration
// (`?f.criticality=critical&sort=score:desc`). Only the links were missing.
//
// THE RULE, and it is the reason this is a module and not five inline template
// strings: a filter is only ever appended to a destination that HONOURS it.
// Sending `?period=30d` to a screen with no period control produces a URL that
// describes a view the user is not being shown — a quieter version of exactly the
// deception this wave removes. Each destination therefore declares what it
// supports, once, here, next to the facet keys it actually reads.

import type { PeriodSelection } from './period';
import { periodToSearchParams } from './period';

/**
 * A destination, and what it understands.
 *
 * `facets` are the DataTable facet keys the screen registers — they become
 * `f.<key>=<v1,v2>` in the URL. `period` says whether the screen has a period
 * control at all; today none of the list screens do, which is a fact this table
 * records rather than a gap it papers over.
 */
interface Destination {
  path: string;
  facets: readonly string[];
  period: boolean;
  /** Sort keys the screen accepts, so a link cannot ask for a column it lacks. */
  sorts?: readonly string[];
}

export const DESTINATIONS = {
  risks: {
    path: '/risks',
    facets: ['criticality', 'status', 'phase', 'category_id', 'source'],
    sorts: ['score', 'name', 'criticality', 'status', 'updated_at'],
    period: false,
  },
  vulnerabilities: {
    path: '/vulnerabilities',
    facets: ['tier', 'severity', 'status', 'kev'],
    sorts: ['priority_score', 'cvss_score', 'cve_id', 'severity'],
    period: false,
  },
  assets: {
    path: '/assets',
    facets: ['type', 'criticality'],
    period: false,
  },
  incidents: { path: '/incidents', facets: [], period: false },
  compliance: { path: '/compliance', facets: [], period: false },
  gaps: { path: '/compliance/gaps', facets: [], period: false },
} as const satisfies Record<string, Destination>;

export type DestinationKey = keyof typeof DESTINATIONS;

export interface DeepLinkOptions {
  /** Facet values, `key -> one or more values`. Unknown keys are DROPPED. */
  filters?: Record<string, string | string[] | undefined>;
  /** Sort key + direction. Dropped if the destination does not offer it. */
  sort?: { key: string; dir: 'asc' | 'desc' };
  /** Free-text query, mapped to the destination's `?q=`. */
  q?: string;
  /** Open a row's drawer on arrival — the `?focus=<id>` universal-search contract. */
  focus?: string;
  /**
   * Carry the dashboard's period across, IF the destination has a period
   * control. Passing it to one that does not is a no-op, not a lie in the URL.
   */
  period?: PeriodSelection;
}

/**
 * Build a deep link.
 *
 * Unknown facet keys and unsupported sorts are dropped SILENTLY and on purpose:
 * a caller that asks for a filter the destination cannot apply has made a
 * mistake, and the safe failure is a broader list rather than a URL that claims
 * a narrowing the screen will not perform. The unit tests assert the dropping,
 * so the mistake is caught where it can be fixed rather than shipped.
 */
export function deepLink(to: DestinationKey, opts: DeepLinkOptions = {}): string {
  const dest = DESTINATIONS[to] as Destination;
  const params = new URLSearchParams();

  for (const [key, raw] of Object.entries(opts.filters ?? {})) {
    if (raw == null) continue;
    if (!dest.facets.includes(key)) continue;
    const values = (Array.isArray(raw) ? raw : [raw]).map((v) => v.trim()).filter(Boolean);
    if (values.length) params.set(`f.${key}`, values.join(','));
  }

  if (opts.sort && dest.sorts?.includes(opts.sort.key)) {
    params.set('sort', `${opts.sort.key}:${opts.sort.dir}`);
  }
  if (opts.q?.trim()) params.set('q', opts.q.trim());
  if (opts.focus) params.set('focus', opts.focus);
  if (opts.period && dest.period) periodToSearchParams(opts.period, params);

  const qs = params.toString();
  return qs ? `${dest.path}?${qs}` : dest.path;
}
