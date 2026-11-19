// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client of the register endpoints added by #913 (GET /risks/summary and the
// `trend` field of GET /risks items). Types come from the generated OpenAPI file.

import { api } from '../../lib/api';
import type { Risk } from '../../hooks/useRiskStore';
import type { components } from '../../types/openapi.generated';
import type { RiskView } from './riskViews';

/** Direction of the score against its baseline 30 days ago; null = no snapshot. */
export type RiskTrend = components['schemas']['RiskTrend'];

/** GET /risks/summary — views, band counts and the active exposure. */
export type RiskSummary = components['schemas']['RiskSummary'];

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

/** The API's page cap for GET /risks (risk_handler: limit <= 200). */
export const LIST_PAGE_LIMIT = 200;

export const riskRegisterService = {
  /**
   * Every risk matching `params`, read page by page (200 at a time) until the
   * server's `total` is reached. Used by the export, which must write the whole
   * view and not the loaded page. Stops on an empty page, so a total that
   * shrinks under us cannot loop.
   */
  async listAll(
    params: Record<string, string | number>,
    onProgress?: (loaded: number, total: number) => void,
    signal?: AbortSignal,
  ): Promise<Risk[]> {
    const all: Risk[] = [];
    for (let page = 1; ; page += 1) {
      const { data } = await api.get<{ items: Risk[]; total: number }>('/risks', {
        params: { ...params, page, limit: LIST_PAGE_LIMIT },
        signal,
      });
      const items = data.items ?? [];
      all.push(...items);
      onProgress?.(all.length, data.total ?? all.length);
      if (items.length === 0 || all.length >= (data.total ?? 0)) return all;
    }
  },

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
