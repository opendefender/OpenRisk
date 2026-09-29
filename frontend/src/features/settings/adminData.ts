// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Data hooks for the admin features consolidated into Settings. API Tokens and
// Custom Fields are live; Members use /organization/members (#807). Roles /
// Organizations / Audit-log endpoints currently 500 (their tables aren't
// migrated in this schema) so their hooks surface an error the UI degrades on
// gracefully.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';

/* ---------------- API Tokens (/tokens) ---------------- */
export interface ApiToken {
  id: string;
  name: string;
  token_prefix?: string;
  expires_at?: string;
  created_at: string;
  last_used_at?: string;
  revoked?: boolean;
}

export function useTokens() {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: ['admin', 'tokens'],
    queryFn: async () => (await api.get<{ tokens: ApiToken[] }>('/tokens')).data?.tokens ?? [],
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ['admin', 'tokens'] });
  const create = useMutation({
    mutationFn: (name: string) => api.post<{ token?: string }>('/tokens', { name }),
    onSuccess: invalidate,
  });
  const revoke = useMutation({
    mutationFn: (id: string) => api.post(`/tokens/${id}/revoke`),
    onSuccess: invalidate,
  });
  return {
    tokens: query.data ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
    create,
    revoke,
  };
}

/* ---------------- Custom Fields (/custom-fields) ---------------- */
export interface CustomField {
  id: string;
  name?: string;
  label?: string;
  field_type?: string;
  entity_type?: string;
  required?: boolean;
}

export function useCustomFields() {
  const query = useQuery({
    queryKey: ['admin', 'custom-fields'],
    queryFn: async () => (await api.get<CustomField[]>('/custom-fields')).data ?? [],
  });
  return { fields: query.data ?? [], isLoading: query.isLoading, isError: query.isError };
}

/* ---------------- Roles / Organizations / Audit ---------------- */
export interface RbacRole {
  id: string;
  name: string;
  description?: string;
  level: number;
  is_predefined?: boolean;
  is_active?: boolean;
}
export function useRoles() {
  const query = useQuery({
    queryKey: ['admin', 'roles'],
    queryFn: async () => (await api.get<{ roles?: RbacRole[] }>('/rbac/roles')).data?.roles ?? [],
    retry: false,
  });
  return { roles: query.data ?? [], isLoading: query.isLoading, isError: query.isError };
}

export interface AuditEntry {
  id?: string;
  action?: string;
  actor?: string;
  user_email?: string;
  resource?: string;
  created_at?: string;
  timestamp?: string;
}
export function useAuditLogs() {
  const query = useQuery({
    queryKey: ['admin', 'audit'],
    queryFn: async () => {
      const d = (
        await api.get<
          AuditEntry[] | { items?: AuditEntry[]; logs?: AuditEntry[]; data?: AuditEntry[] }
        >('/audit-logs')
      ).data;
      return Array.isArray(d) ? d : (d.items ?? d.logs ?? d.data ?? []);
    },
    retry: false,
  });
  return { logs: query.data ?? [], isLoading: query.isLoading, isError: query.isError };
}

export interface Org {
  id?: string;
  name?: string;
  slug?: string;
  created_at?: string;
}
export function useTenants() {
  const query = useQuery({
    queryKey: ['admin', 'tenants'],
    queryFn: async () => {
      const d = (await api.get<Org[] | { items?: Org[]; tenants?: Org[] }>('/rbac/tenants')).data;
      return Array.isArray(d) ? d : (d.items ?? d.tenants ?? []);
    },
    retry: false,
  });
  return { tenants: query.data ?? [], isLoading: query.isLoading, isError: query.isError };
}
