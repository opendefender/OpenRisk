// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query binding for the Action Center, following useNotifications.

import { useQuery } from '@tanstack/react-query';
import { useAuthStore } from '../../hooks/useAuthStore';
import { actionCenterService, type ActionItem } from './actionCenterService';

export const ACTION_CENTER_ROOT = 'action-center' as const;

/**
 * Cache key, scoped by tenant — the same shape the entity drawer uses.
 *
 * This list is tenant data, so the tenant belongs in the key rather than being
 * assumed from the fact that a session cannot currently change tenants without
 * a reload. That assumption is true today and is exactly the kind of thing that
 * stops being true quietly.
 */
export function actionCenterKey(tenant: string, limit: number, offset: number) {
  return [ACTION_CENTER_ROOT, tenant, limit, offset] as const;
}

export interface UseActionItemsOptions {
  /** Page size. The server defaults to 20 and clamps to 100. */
  limit?: number;
  /** Rows to skip. Negative values are rejected server-side with 400. */
  offset?: number;
}

export interface UseActionItemsResult {
  /** The server's list, in the server's order. Never re-sorted here. */
  items: ActionItem[];
  /** Everything outstanding for this caller — not the size of this page. */
  total: number;
  /** When the aggregation ran. The list is a live read, so this is its as-of. */
  generatedAt: string | null;
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  refetch: () => void;
}

/** Page size used by the dashboard panel — a preview, not the whole queue. */
export const PANEL_LIMIT = 8;

/** Page size used by the full page at /action-center. */
export const PAGE_LIMIT = 20;

/**
 * The caller's prioritised outstanding work.
 *
 * Two deliberate omissions:
 *
 *   - no `placeholderData`, for the same reason the notification bell has none:
 *     an invented action item is worse than a skeleton, and it would claim work
 *     on a tenant that has none;
 *   - no client-side sort. `category_rank` rides on the wire so a client can
 *     GROUP without re-deriving the priority rule, not so it can re-apply it.
 *     The server has already ordered by rank, then by each category's own
 *     secondary key, then by id; re-sorting here would silently disagree with
 *     the server as soon as there is a second page.
 */
export function useActionItems({ limit = PAGE_LIMIT, offset = 0 }: UseActionItemsOptions = {}): UseActionItemsResult {
  const tenant = useAuthStore((s) => s.user?.tenant_id) ?? 'anonymous';
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: actionCenterKey(tenant, limit, offset),
    queryFn: () => actionCenterService.list({ limit, offset }),
    // The underlying records change as work gets done elsewhere in the app, and
    // an item vanishes the moment its work is finished — there is no dismiss.
    refetchInterval: 60_000,
  });

  return {
    items: data?.data ?? [],
    total: data?.total ?? 0,
    generatedAt: data?.generated_at ?? null,
    isLoading,
    isError,
    error,
    refetch: () => {
      void refetch();
    },
  };
}
