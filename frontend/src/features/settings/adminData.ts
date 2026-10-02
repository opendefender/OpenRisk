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

/* ---------------- Members (/users) ---------------- */
export interface AdminUser {
  id: string;
  email: string;
  username: string;
  full_name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  last_login?: string;
}

export function useUsers() {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: async () => (await api.get<AdminUser[]>('/users')).data ?? [],
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ['admin', 'users'] });
  const setStatus = useMutation({
    mutationFn: ({ id, is_active }: { id: string; is_active: boolean }) =>
      api.patch(`/users/${id}/status`, { is_active }),
    onSuccess: invalidate,
  });
  const setRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) =>
      api.patch(`/users/${id}/role`, { role }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.delete(`/users/${id}`),
    onSuccess: invalidate,
  });
  return {
    users: query.data ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    setStatus,
    setRole,
    remove,
  };
}

/* ---------------- API Tokens (/auth/pat) ---------------- */
// Personal access tokens (#782). Stored hashed in PostgreSQL, bound to the
// organization they were created in, and accepted by the PAT middleware on
// every /api/v1 route as `Authorization: Bearer orsk_…`.
export interface ApiToken {
  id: string;
  name: string;
  token_prefix: string;
  expires_at?: string | null;
  created_at: string;
  last_used_at?: string | null;
}

/** POST /auth/pat answer. `token` is the secret, returned this once only. */
export interface CreatedApiToken {
  id: string;
  name: string;
  token_prefix: string;
  created_at: string;
  token: string;
}

const TOKENS_KEY = ['admin', 'tokens'] as const;

export function useTokens() {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: TOKENS_KEY,
    queryFn: async () =>
      (await api.get<{ tokens: ApiToken[] | null }>('/auth/pat')).data?.tokens ?? [],
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: TOKENS_KEY });
  const create = useMutation({
    // "*" = everything the creator holds in this organization, never more: the
    // server intersects it with the creator's own permissions on every request.
    mutationFn: async (name: string) =>
      (await api.post<CreatedApiToken>('/auth/pat', { name, scopes: ['*'] })).data,
    onSuccess: invalidate,
  });
  const revoke = useMutation({
    mutationFn: (id: string) => api.delete(`/auth/pat/${id}`),
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: TOKENS_KEY });
      const previous = qc.getQueryData<ApiToken[]>(TOKENS_KEY);
      qc.setQueryData<ApiToken[]>(TOKENS_KEY, (rows) => (rows ?? []).filter((t) => t.id !== id));
      return { previous };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.previous) qc.setQueryData(TOKENS_KEY, ctx.previous);
    },
    onSettled: invalidate,
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
