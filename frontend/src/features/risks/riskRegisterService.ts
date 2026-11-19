// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client of the register endpoints added by #913. The OpenAPI types are owned
// by the backend side and regenerated from the spec; until that lands these
// interfaces mirror the contract by hand (GET /risks/summary and the `trend`
// field of GET /risks items).

import { api } from '../../lib/api';
import type { Risk } from '../../hooks/useRiskStore';
import type { Criticality } from '../../shared/riskColors';
import type { RiskView } from './riskViews';

/** Direction of the score against its baseline 30 days ago; null = no snapshot. */
export type RiskTrend = 'up' | 'down' | 'flat';

export interface RiskSummary {
  /** Size of each view over the whole register (ignores `view` and `q`). */
  views: Record<RiskView, number>;
  /** Counts inside `view` + `q`; the other list filters are ignored. */
  by_criticality: Record<Criticality, number>;
  active: {
    /** Risks that are not closed, whole tenant. */
    count: number;
    /** Summed annual loss expectancy in XAF; null without the financial entitlement. */
    ale_xaf: number | null;
  };
}

export interface RiskSummaryQuery {
  view: RiskView;
  q: string;
}

/** The trend as the API sends it, tolerant of casing and of anything unexpected. */
export function toTrend(raw: unknown): RiskTrend | null {
  switch (typeof raw === 'string' ? raw.toLowerCase() : '') {
    case 'up':
      return 'up';
    case 'down':
      return 'down';
    case 'flat':
      return 'flat';
    default:
      return null;
  }
}

export const riskRegisterService = {
  /** One risk, in the register's row shape — for a deep link to a risk that is
   *  not on the loaded page. */
  async get(id: string, signal?: AbortSignal): Promise<Risk> {
    const { data } = await api.get<Risk>(`/risks/${id}`, { signal });
    return data;
  },

  async summary(query: RiskSummaryQuery, signal?: AbortSignal): Promise<RiskSummary> {
    const params: Record<string, string> = {};
    if (query.view !== 'all') params.view = query.view;
    if (query.q) params.q = query.q;
    const { data } = await api.get<RiskSummary>('/risks/summary', { params, signal });
    return data;
  },
};
