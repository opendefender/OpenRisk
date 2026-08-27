// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the universal entity drawer (W1-02).
//
// Five endpoints serve all eight types. Errors are NOT swallowed here — unlike
// the search client, where a failure degrades to "no results", a drawer that
// cannot load its entity must say so: 403 and 404 mean different things to the
// user and each has its own state in the UI (§29-§31).

import { api } from '../../lib/api';
import type {
  AuditPage,
  EntityCatalogueEntry,
  EntitySection,
  EntityType,
  EntityView,
  RelationGroup,
  RelationsResponse,
  TimelinePage,
} from './types';

/** What the UI needs to tell 403 from 404 from "the network is down". */
export interface EntityError {
  status: number;
  message: string;
}

export function isEntityError(e: unknown): e is EntityError {
  return typeof e === 'object' && e !== null && 'status' in e && 'message' in e;
}

interface AxiosLikeError {
  response?: { status?: number; data?: { error?: string; message?: string } };
  message?: string;
}

function toEntityError(err: unknown): EntityError {
  const e = err as AxiosLikeError;
  const status = e?.response?.status ?? 0;
  const message = e?.response?.data?.error ?? e?.response?.data?.message ?? e?.message ?? 'Request failed';
  return { status, message };
}

async function get<T>(url: string, params?: Record<string, string | number | undefined>): Promise<T> {
  try {
    const { data } = await api.get<T>(url, { params });
    return data;
  } catch (err) {
    throw toEntityError(err);
  }
}

const base = (type: EntityType, id: string) =>
  `/entities/${encodeURIComponent(type)}/${encodeURIComponent(id)}`;

export function fetchEntity(type: EntityType, id: string): Promise<EntityView> {
  return get<EntityView>(base(type, id));
}

export async function fetchRelations(type: EntityType, id: string): Promise<RelationGroup[]> {
  const data = await get<RelationsResponse>(`${base(type, id)}/relations`);
  return data.groups ?? [];
}

export function fetchTimeline(
  type: EntityType,
  id: string,
  opts: { cursor?: string; limit?: number; kind?: string } = {}
): Promise<TimelinePage> {
  return get<TimelinePage>(`${base(type, id)}/timeline`, {
    cursor: opts.cursor || undefined,
    limit: opts.limit,
    kind: opts.kind || undefined,
  });
}

export function fetchAudit(
  type: EntityType,
  id: string,
  opts: { limit?: number; offset?: number } = {}
): Promise<AuditPage> {
  return get<AuditPage>(`${base(type, id)}/audit`, { limit: opts.limit, offset: opts.offset });
}

/** The tenant-wide activity feed. */
export function fetchTenantTimeline(
  opts: { cursor?: string; limit?: number; kind?: string } = {}
): Promise<TimelinePage> {
  return get<TimelinePage>('/timeline', {
    cursor: opts.cursor || undefined,
    limit: opts.limit,
    kind: opts.kind || undefined,
  });
}

/** Which types this deployment resolves, and which of them the caller may read. */
export async function fetchCatalogue(): Promise<EntityCatalogueEntry[]> {
  const data = await get<{ types: EntityCatalogueEntry[] }>('/entities');
  return data.types ?? [];
}

/** The section a drawer opens on when the URL names none. */
export const DEFAULT_SECTION: EntitySection = 'summary';
