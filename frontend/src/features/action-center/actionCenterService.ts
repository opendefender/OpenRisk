// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for GET /api/v1/action-center (#429 / #430).
//
// The response shape is NOT hand-typed here. It is imported from
// src/types/openapi.generated.ts, which openapi-typescript generates from
// docs/openapi.yaml — the contract the backend actually serves. That indirection
// is the entire point of this file: the notification bell shipped with four
// hardcoded items and a set of localStorage-backed switches precisely because
// the screen was allowed to describe the server rather than read it. If the
// backend changes a field, `tsc --noEmit` fails here instead of the panel
// silently rendering `undefined`.

import { api } from '../../lib/api';
import type { components } from '../../types/openapi.generated';

/** One thing the caller needs to act on. Assembled server-side on read. */
export type ActionItem = components['schemas']['ActionItem'];

/** The paged envelope. `data` is always an array, never null, even when empty. */
export type ActionCenterResponse = components['schemas']['ActionCenterResponse'];

/** The six categories the server can return, in `category_rank` order. */
export type ActionItemType = ActionItem['type'];

export interface ActionCenterQuery {
  /** Page size. The server defaults to 20 and clamps to 100. */
  limit?: number;
  /** Rows to skip. A negative value is rejected server-side with 400. */
  offset?: number;
}

export const actionCenterService = {
  /**
   * The caller's outstanding work, already prioritised by the server.
   *
   * The order is the server's: category rank, then each category's own
   * secondary key, then id. It is returned untouched — see useActionItems for
   * why re-sorting client-side would contradict the server on page two.
   */
  async list(query: ActionCenterQuery = {}): Promise<ActionCenterResponse> {
    const { data } = await api.get<ActionCenterResponse>('/action-center', { params: query });
    return data;
  },
};
