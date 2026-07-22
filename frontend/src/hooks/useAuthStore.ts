// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';
import { api } from '../lib/api';

interface User {
  id: string;
  email: string;
  username: string;
  full_name: string;
  role: string; // role name for quick access
  role_level?: number; // role level for RBAC
  permissions?: string[]; // array of permission strings
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

export const useAuthStore = create<AuthStore>((set, get) => ({
  user: JSON.parse(localStorage.getItem('auth_user') || 'null'),
  token: localStorage.getItem('auth_token'),
  expiresIn: localStorage.getItem('auth_expires_in') ? parseInt(localStorage.getItem('auth_expires_in')!) : null,
  isAuthenticated: !!localStorage.getItem('auth_token'),

  login: async (email, password) => {
    // Backend shape: { user: domain.User, token_pair: { access_token, refresh_token, expires_in }, organization }
    // domain.User.role is a nested Role object (or absent) — flatten to the role name this store expects.
    const { data } = await api.post('/auth/login', { email, password });
    const user: User = { ...data.user, role: data.user.role?.name ?? '' };

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

      set({
        token: data.token_pair.access_token,
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
    if (!user) return false;
    
    // For now, use simple role-based checks
    // In production, would check actual permission array from role
    const rolePermissions: Record<string, string[]> = {
      admin: ['*'],
      analyst: ['risk:read', 'risk:create', 'risk:update', 'mitigation:read', 'mitigation:create', 'mitigation:update', 'asset:read'],
      viewer: ['risk:read', 'mitigation:read', 'asset:read']
    };
    
    const permissions = rolePermissions[user.role.toLowerCase()] || [];
    
    // Check for exact match or admin wildcard
    if (permissions.includes('*') || permissions.includes(permission)) {
      return true;
    }
    
    // Check for resource-level wildcard (e.g., "risk:*")
    const [resource] = permission.split(':');
    return permissions.includes(`${resource}:*`);
  },

  hasRole: (roleName: string) => {
    const { user } = get();
    return user?.role.toLowerCase() === roleName.toLowerCase();
  }
}));