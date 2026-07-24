// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the universal search endpoint (UX-1) that powers the ⌘K
// palette. One call returns cross-entity hits (risk/asset/vulnerability, …) the
// caller may read, tenant-scoped and RBAC-gated by the backend.

import { api } from '../lib/api';

export type SearchResultType =
  | 'risk'
  | 'asset'
  | 'vulnerability'
  | 'control'
  | 'audit'
  | 'report'
  | 'cve'
  | 'user';

export interface SearchResult {
  type: SearchResultType;
  id: string;
  title: string;
  subtitle?: string;
  /** Severity/criticality token (critical|high|medium|low|info) for a colored chip. */
  badge?: string;
  /** Frontend deep-link to the entity. */
  url: string;
  score?: number;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
}

/**
 * Run a universal search. Pass an AbortSignal so an in-flight request can be
 * cancelled when the user keeps typing. Returns [] on abort/failure — search is a
 * best-effort convenience, never a hard error in the UI.
 */
export async function universalSearch(q: string, signal?: AbortSignal): Promise<SearchResult[]> {
  try {
    const { data } = await api.get<SearchResponse>('/search', { params: { q }, signal });
    return data.results ?? [];
  } catch {
    return [];
  }
}
