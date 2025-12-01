import { create } from 'zustand';
import { api } from '../lib/api';
import type { Asset } from './useAssetStore';

export interface Mitigation {
  id: string;
  title: string;
  status: 'PLANNED' | 'IN_PROGRESS' | 'DONE';
  progress: number;
  assignee?: string;
}

export interface Risk {
  id: string;
  title: string;
  description: string;
  score: number;
  impact: number;
  probability: number;
  status: string;
  tags: string[];
  assets?: Asset[]; // Important pour l'association Risk-Asset
  
  source: string; // Important pour l'étape d'intégration (THEHIVE, etc.)
  mitigations?: Mitigation[]; // Important pour le drawer de détails
}

interface RiskFetchParams {
  q?: string;
  status?: string;
  min_score?: number;
  max_score?: number;
  tag?: string;
}

interface RiskStore {
  risks: Risk[];
  isLoading: boolean;
  // pagination
  total: number;
  page: number;
  pageSize: number;
  setPage: (p: number) => Promise<void>;

  fetchRisks: (params?: RiskFetchParams & { page?: number; limit?: number }) => Promise<void>;
  createRisk: (payload: any) => Promise<void>;
  updateRisk: (id: string, payload: any) => Promise<void>;
  deleteRisk: (id: string) => Promise<void>;
}

// --- STORE ZUSTAND ---

export const useRiskStore = create<RiskStore>((set, get) => ({
  risks: [],
  isLoading: false,
  total: 0,
  page: 1,
  pageSize: 20,

  setPage: async (p: number) => {
    set({ page: p });
    // Re-fetch with current filters (if any)
    await get().fetchRisks({ page: p, limit: get().pageSize });
  },

  fetchRisks: async (params) => {
    set({ isLoading: true });
    try {
      const response = await api.get('/risks', { params });

      // Support new paginated response: { items: [...], total: number }
      if (response.data && response.data.items) {
        set({ risks: response.data.items, total: response.data.total || 0 });
      } else if (Array.isArray(response.data)) {
        set({ risks: response.data, total: response.data.length });
      } else {
        // Fallback
        set({ risks: [], total: 0 });
      }
    } catch (error) {
      console.error('Failed to fetch risks', error);
      // In production, set an error state or show a toast
      set({ risks: [], total: 0 });
    } finally {
      set({ isLoading: false });
    }
  },
  createRisk: async (payload) => {
    set({ isLoading: true });
    try {
      await api.post('/risks', payload);
      // refresh current page
      await get().fetchRisks({ page: get().page, limit: get().pageSize });
    } catch (err) {
      console.error('Failed to create risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
  updateRisk: async (id, payload) => {
    set({ isLoading: true });
    try {
      await api.patch(`/risks/${id}`, payload);
      await get().fetchRisks({ page: get().page, limit: get().pageSize });
    } catch (err) {
      console.error('Failed to update risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
  deleteRisk: async (id) => {
    set({ isLoading: true });
    try {
      await api.delete(`/risks/${id}`);
      // After delete, refetch current page (ensure page bounds)
      const nextPage = Math.max(1, get().page);
      await get().fetchRisks({ page: nextPage, limit: get().pageSize });
    } catch (err) {
      console.error('Failed to delete risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
}));