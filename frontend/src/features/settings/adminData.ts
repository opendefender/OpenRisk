// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Data hooks for the admin features consolidated into Settings. Members, API
// Tokens and Custom Fields are live; Roles / Organizations / Audit-log endpoints
// currently 500 (their tables aren't migrated in this schema) so their hooks
// surface an error the UI degrades on gracefully.

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
    mutationFn: ({ id, is_active }: { id: string; is_active: boolean }) => api.patch(`/users/${id}/status`, { is_active }),
    onSuccess: invalidate,
  });
  const setRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) => api.patch(`/users/${id}/role`, { role }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.delete(`/users/${id}`),
    onSuccess: invalidate,
  });
  return { users: query.data ?? [], isLoading: query.isLoading, isError: query.isError, setStatus, setRole, remove };
}

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
  return { tokens: query.data ?? [], isLoading: query.isLoading, isError: query.isError, create, revoke };
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
      const d = (await api.get<AuditEntry[] | { items?: AuditEntry[]; logs?: AuditEntry[]; data?: AuditEntry[] }>('/audit-logs')).data;
      return Array.isArray(d) ? d : d.items ?? d.logs ?? d.data ?? [];
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
      return Array.isArray(d) ? d : d.items ?? d.tenants ?? [];
    },
    retry: false,
  });
  return { tenants: query.data ?? [], isLoading: query.isLoading, isError: query.isError };
}
