// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query bindings for the entity drawer (W1-02 §32).
//
// CACHE KEYS CARRY THE TENANT. Not because the server would ever serve another
// tenant's row — it refuses — but because the client holds the answer after the
// server has stopped being asked. `logout` and `login` both call
// clearSessionScope(), which drops the whole cache; the tenant in the key is the
// second belt: if an identity ever changes without that clear (a token refresh
// that re-scopes the session, a bug in a future switcher), a key that names the
// old tenant simply cannot be read under the new one. A miss costs one refetch.
// A hit would cost a cross-tenant disclosure.

import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  fetchAudit,
  fetchCatalogue,
  fetchEntity,
  fetchRelations,
  fetchTenantTimeline,
  fetchTimeline,
} from './entityService';
import type { EntityType } from './types';

/** The root of every drawer key, so one call can invalidate the lot. */
export const ENTITY_QUERY_ROOT = 'entity-drawer' as const;

/** The tenant the current session belongs to, or 'anonymous'. */
function useTenantKey(): string {
  return useAuthStore((s) => s.user?.tenant_id) ?? 'anonymous';
}

export function entityKey(tenant: string, type: EntityType, id: string) {
  return [ENTITY_QUERY_ROOT, tenant, type, id] as const;
}

/**
 * The entity head. `enabled` is false until there is something to load, so a
 * closed drawer costs nothing.
 *
 * retry:false — a 403 or a 404 is a final answer about authorisation or
 * existence, and retrying it three times only delays telling the user.
 */
export function useEntity(type: EntityType | null, id: string | null) {
  const tenant = useTenantKey();
  return useQuery({
    queryKey: entityKey(tenant, type ?? 'risk', id ?? ''),
    queryFn: () => fetchEntity(type as EntityType, id as string),
    enabled: !!type && !!id,
    retry: false,
    staleTime: 30_000,
  });
}

/**
 * Relations, loaded separately from the head.
 *
 * Separate on purpose (§27): relations are separately slow and separately
 * allowed to fail, and folding them into the head would mean one failing
 * relation query blanks the whole drawer. `enabled` also waits for the tab to
 * be open, so opening a drawer on Summary never pays for a relation query the
 * user did not ask for.
 */
export function useEntityRelations(type: EntityType | null, id: string | null, enabled: boolean) {
  const tenant = useTenantKey();
  return useQuery({
    queryKey: [...entityKey(tenant, type ?? 'risk', id ?? ''), 'relations'] as const,
    queryFn: () => fetchRelations(type as EntityType, id as string),
    enabled: enabled && !!type && !!id,
    retry: false,
    staleTime: 30_000,
  });
}

/**
 * The entity timeline, cursor-paginated.
 *
 * useInfiniteQuery because the server pages by cursor: "load more" appends,
 * rather than replacing, which is what a history reads like.
 */
export function useEntityTimeline(type: EntityType | null, id: string | null, enabled: boolean) {
  const tenant = useTenantKey();
  return useInfiniteQuery({
    queryKey: [...entityKey(tenant, type ?? 'risk', id ?? ''), 'timeline'] as const,
    queryFn: ({ pageParam }) =>
      fetchTimeline(type as EntityType, id as string, { cursor: pageParam as string | undefined, limit: 25 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: enabled && !!type && !!id,
    retry: false,
    staleTime: 15_000,
  });
}

/** The raw audit trail. Only fetched when the tab is open AND the server offered
 *  the tab — it gates on its own permission. */
export function useEntityAudit(type: EntityType | null, id: string | null, enabled: boolean) {
  const tenant = useTenantKey();
  return useQuery({
    queryKey: [...entityKey(tenant, type ?? 'risk', id ?? ''), 'audit'] as const,
    queryFn: () => fetchAudit(type as EntityType, id as string, { limit: 50 }),
    enabled: enabled && !!type && !!id,
    retry: false,
    staleTime: 15_000,
  });
}

/** The tenant-wide activity feed. */
export function useTenantTimeline(kind?: string) {
  const tenant = useTenantKey();
  return useInfiniteQuery({
    queryKey: [ENTITY_QUERY_ROOT, tenant, 'tenant-timeline', kind ?? 'all'] as const,
    queryFn: ({ pageParam }) =>
      fetchTenantTimeline({ cursor: pageParam as string | undefined, limit: 30, kind }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor || undefined,
    retry: false,
    staleTime: 15_000,
  });
}

/** The type catalogue — which types exist and which the caller may read. */
export function useEntityCatalogue() {
  const tenant = useTenantKey();
  return useQuery({
    queryKey: [ENTITY_QUERY_ROOT, tenant, 'catalogue'] as const,
    queryFn: fetchCatalogue,
    staleTime: 5 * 60_000,
    retry: false,
  });
}
