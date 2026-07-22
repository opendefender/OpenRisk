// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';
import { api } from '../lib/api';
import { decodeAccessToken, permitted } from '../lib/jwt';

interface User {
  id: string;
  email: string;
  username: string;
  full_name: string;
  role: string; // org role: root | admin | user
  role_level?: number; // role level for RBAC
  permissions?: string[]; // resolved permission strings from the JWT (["*"] for admin)
  org_roles?: Record<string, string>; // { orgId: roleName } from the JWT
  business_role?: string; // GRC job-role preset (rssi/dsi/…), "" for admin/root
  tenant_id?: string;
  bio?: string;
  phone?: string;
  department?: string;
  timezone?: string;
}

interface AuthStore {
  user: User | null;
  token: string | null;
  expiresIn: number | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<void>;
  hasPermission: (permission: string) => boolean;
  hasRole: (roleName: string) => boolean;
}

/** Merge the JWT-derived permissions/roles/tenant onto a user object. */
function withTokenClaims(user: User, token: string, businessRole?: string): User {
  const claims = decodeAccessToken(token);
  return {
    ...user,
    permissions: claims.permissions ?? user.permissions ?? [],
    org_roles: claims.org_roles ?? user.org_roles,
    tenant_id: claims.tenant_id ?? user.tenant_id,
    business_role: businessRole ?? user.business_role ?? '',
  };
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  user: JSON.parse(localStorage.getItem('auth_user') || 'null'),
  token: localStorage.getItem('auth_token'),
  expiresIn: localStorage.getItem('auth_expires_in') ? parseInt(localStorage.getItem('auth_expires_in')!) : null,
  isAuthenticated: !!localStorage.getItem('auth_token'),

  login: async (email, password) => {
    // Backend shape: { user: domain.User, token_pair: { access_token, refresh_token, expires_in }, organization }
    // domain.User.role is a nested Role object (or absent) — flatten to the role name this store expects.
    const { data } = await api.post('/auth/login', { email, password });
    // Flatten the nested Role object and fold in the JWT permissions/roles + the
    // business_role the backend returns, so RBAC gating works on the client.
    const base: User = { ...data.user, role: data.user.role?.name ?? '' };
    const user = withTokenClaims(base, data.token_pair.access_token, data.business_role);

    localStorage.setItem('auth_token', data.token_pair.access_token);
    localStorage.setItem('auth_refresh_token', data.token_pair.refresh_token);
    localStorage.setItem('auth_user', JSON.stringify(user));
    localStorage.setItem('auth_expires_in', data.token_pair.expires_in.toString());

    set({
      token: data.token_pair.access_token,
      user,
      expiresIn: data.token_pair.expires_in,
      isAuthenticated: true
    });
  },

  logout: () => {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_refresh_token');
    localStorage.removeItem('auth_user');
    localStorage.removeItem('auth_expires_in');
    set({
      token: null,
      user: null,
      expiresIn: null,
      isAuthenticated: false
    });
  },

  refreshToken: async () => {
    try {
      const refresh_token = localStorage.getItem('auth_refresh_token');
      const { data } = await api.post('/auth/refresh', { refresh_token });

      localStorage.setItem('auth_token', data.token_pair.access_token);
      localStorage.setItem('auth_refresh_token', data.token_pair.refresh_token);
      localStorage.setItem('auth_expires_in', data.token_pair.expires_in.toString());

      // Re-derive permissions from the rotated token (a business-role change or a
      // re-scoped session takes effect here, mirroring the backend SessionResolver).
      const current = get().user;
      const user = current ? withTokenClaims(current, data.token_pair.access_token) : current;
      if (user) localStorage.setItem('auth_user', JSON.stringify(user));

      set({
        token: data.token_pair.access_token,
        user,
        expiresIn: data.token_pair.expires_in
      });
    } catch (err) {
      // Token refresh failed, logout user
      get().logout();
      throw err;
    }
  },

  hasPermission: (permission: string) => {
    const { user } = get();
    // Use the real permission set resolved from the JWT (["*"] for admin/root),
    // with the same wildcard semantics the backend enforces.
    return permitted(user?.permissions, permission);
  },

  hasRole: (roleName: string) => {
    const { user } = get();
    if (!user) return false;
    // Match either the org role (root/admin/user) or the business role preset.
    const target = roleName.toLowerCase();
    return user.role.toLowerCase() === target || (user.business_role ?? '').toLowerCase() === target;
  }
}));