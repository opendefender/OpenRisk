// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Security Command Center's data hooks.
//
// Two things changed here from the `useStats` this replaces, and both are about
// the cache rather than the numbers.
//
// 1. THESE ARE QUERIES, not a bare useEffect. The old hook was
//    `useState` + `useEffect(..., [])`: it fetched exactly once per mount, had no
//    key, and therefore could not be invalidated, could not refetch, and could not
//    be shared between the two personas that both called it. Two components
//    mounting it made two requests for the same numbers and could hold two
//    different answers.
//
// 2. THE KEY CARRIES THE PERIOD. Two windows over the same register are two
//    different answers; caching them under one key is the client-side equivalent
//    of ignoring the filter. The server's cache key carries the period for the
//    same reason.
//
// TENANT SCOPE: not in the key, deliberately. The tenant is never a request
// parameter — the server takes it from the authenticated session, and a client
// that could name a tenant in a query key could name someone else's. What keeps
// one organisation's numbers out of another's cache is lib/sessionScope.ts, which
// drops the entire QueryClient at both ends of a session transition. Adding a
// tenant id here would look like isolation while providing none.

import { useQuery } from '@tanstack/react-query';

import { api } from '../../lib/api';
import { periodKey, type PeriodParams, type PeriodSelection, periodParams } from './period';

/** One cell of the 5x5 heatmap. Banded server-side so every consumer bands identically. */
export interface MatrixCell {
  probability: number; // 1-5 band
  impact: number; // 1-5 band
  count: number;
}

/** The window the server actually used, echoed back. */
export interface ResolvedPeriod {
  preset: string;
  /** Absent for an unbounded window — there is no start date to invent. */
  from?: string;
  to: string;
}

export interface RiskTrendPoint {
  /** Inclusive bucket start, `YYYY-MM-DD`. */
  bucket: string;
  /** Risks CREATED in this bucket — a flow. */
  opened: number;
  /** `opened`, split by each risk's current criticality band. */
  opened_by_band: Record<string, number>;
  /** Every risk that existed at this bucket's end — a stock. */
  cumulative_total: number;
}

export interface RiskTrend {
  /** NOT always the requested window: an unbounded period caps its series. */
  from: string;
  to: string;
  granularity: 'day' | 'week';
  points: RiskTrendPoint[];
}

/**
 * Every field is a tenant-wide SQL aggregate. None may be recomputed on the
 * client from a page of results: the register is paginated, so a
 * `.filter().length` reports the current page while reading like a total.
 */
export interface DashboardStats {
  period: ResolvedPeriod;
  /** The fields the window narrowed. Everything else is a point-in-time stock. */
  period_applies_to: string[];
  total_risks: number;
  high_risks: number;
  mitigated_risks: number;
  in_progress_risks: number;
  quantified_risks: number;
  risks_by_severity: Record<string, number>;
  risk_matrix: MatrixCell[];
  opened_in_period: number;
  risk_trend: RiskTrend;
  generated_at: string;
}

export interface AssetStatistics {
  period: ResolvedPeriod;
  period_applies_to: string[];
  total: number;
  by_criticality: Record<string, number>;
  by_category: Record<string, number>;
  uncategorised: number;
  by_type: Record<string, number>;
  untyped: number;
  types_truncated: number;
  distinct_types: number;
  by_source: Record<string, number>;
  added_in_period: number;
  generated_at: string;
}

export const DASHBOARD_STATS_KEY = 'dashboard-stats';
export const ASSET_STATS_KEY = 'asset-statistics';

// Long enough that a dashboard full of widgets makes one request per endpoint,
// short enough that a number is not stale after a mutation elsewhere. Same
// value as useScore, so the hero and the tiles beside it age together.
const STALE = 30_000;

function query<T>(path: string, params: PeriodParams, signal?: AbortSignal) {
  return api.get<T>(path, { params, signal }).then((r) => r.data);
}

/**
 * The register's posture over the selected window.
 *
 * `throwOnError` is left off and the error is returned to the caller instead,
 * because every consumer must render an error STATE. A dashboard that swallows a
 * failed fetch shows zeros it never read — which on a security score reads as
 * "nothing wrong here", the most dangerous thing this screen can say.
 */
export function useDashboardStats(selection: PeriodSelection) {
  return useQuery({
    queryKey: [DASHBOARD_STATS_KEY, periodKey(selection)],
    queryFn: ({ signal }) => query<DashboardStats>('/stats', periodParams(selection), signal),
    staleTime: STALE,
  });
}

/** The inventory's shape, counted in SQL rather than reduced from the collection. */
export function useAssetStatistics(selection: PeriodSelection) {
  return useQuery({
    queryKey: [ASSET_STATS_KEY, periodKey(selection)],
    queryFn: ({ signal }) => query<AssetStatistics>('/assets/statistics', periodParams(selection), signal),
    staleTime: STALE,
  });
}
