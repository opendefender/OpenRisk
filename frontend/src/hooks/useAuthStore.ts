import { create } from 'zustand';
import { api } from '../lib/api';

interface User {
  id: string;
  email: string;
  username: string;
  full_name: string;
  role: string; // role name for quick access
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
    const { data } = await api.post('/auth/login', { email, password });
    
    localStorage.setItem('auth_token', data.token);
    localStorage.setItem('auth_user', JSON.stringify(data.user));
    localStorage.setItem('auth_expires_in', data.expires_in.toString());
    
    set({ 
      token: data.token, 
      user: data.user,
      expiresIn: data.expires_in,
      isAuthenticated: true
    });
  },

  logout: () => {
    localStorage.removeItem('auth_token');
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
      const { data } = await api.post('/auth/refresh', {});
      
      localStorage.setItem('auth_token', data.token);
      localStorage.setItem('auth_user', JSON.stringify(data.user));
      localStorage.setItem('auth_expires_in', data.expires_in.toString());
      
      set({ 
        token: data.token, 
        user: data.user,
        expiresIn: data.expires_in
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
      admin: [''],
      analyst: ['risk:read', 'risk:create', 'risk:update', 'mitigation:read', 'mitigation:create', 'mitigation:update', 'asset:read'],
      viewer: ['risk:read', 'mitigation:read', 'asset:read']
    };
    
    const permissions = rolePermissions[user.role.toLowerCase()] || [];
    
    // Check for exact match or admin wildcard
    if (permissions.includes('') || permissions.includes(permission)) {
      return true;
    }
    
    // Check for resource-level wildcard (e.g., "risk:")
    const [resource] = permission.split(':');
    return permissions.includes(${resource}:);
  },

  hasRole: (roleName: string) => {
    const { user } = get();
    return user?.role.toLowerCase() === roleName.toLowerCase();
  }
}));