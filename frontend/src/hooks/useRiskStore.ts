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
  
  source: string; // Important pour l'tape d'intgration (THEHIVE, etc.)
  mitigations?: Mitigation[]; // Important pour le drawer de dtails
  created_at?: string;
  level?: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
}

interface RiskFetchParams {
  q?: string;
  status?: string;
  min_score?: number;
  max_score?: number;
  tag?: string;
  sort_by?: string;
  sort_dir?: 'asc' | 'desc';
}

interface RiskStore {
  risks: Risk[];
  isLoading: boolean;
  // pagination
  total: number;
  page: number;
  pageSize: number;
  setPage: (p: number) => Promise<void>;
  // selected risk for global drawer
  selectedRisk?: Risk | null;
  setSelectedRisk: (r: Risk | null) => void;

  fetchRisks: (params?: RiskFetchParams & { page?: number; limit?: number }) => Promise<void>;
  createRisk: (payload: any) => Promise<void>;
  updateRisk: (id: string, payload: any) => Promise<void>;
  deleteRisk: (id: string) => Promise<void>;
}

// --- STORE ZUSTAND ---

export const useRiskStore = create<RiskStore>((set, get) => ({
  risks: [],
  isLoading: false,
  total: ,
  page: ,
  pageSize: ,
  selectedRisk: null,
  setSelectedRisk: (r: Risk | null) => set({ selectedRisk: r }),

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
        set({ risks: response.data.items, total: response.data.total ||  });
      } else if (Array.isArray(response.data)) {
        set({ risks: response.data, total: response.data.length });
      } else {
        // Fallback
        set({ risks: [], total:  });
      }
    } catch (error) {
      console.error('Failed to fetch risks', error);
      // In production, set an error state or show a toast
      set({ risks: [], total:  });
    } finally {
      set({ isLoading: false });
    }
  },
  createRisk: async (payload) => {
    set({ isLoading: true });
    // optimistic create: add a temporary item to the list
    const tempId = tmp-${Date.now()};
    const optimistic: Risk = {
      id: tempId,
      title: payload.title || 'Nouvel lment',
      description: payload.description || '',
      score: payload.score ?? ,
      impact: payload.impact ?? ,
      probability: payload.probability ?? ,
      status: payload.status || 'DRAFT',
      tags: payload.tags || [],
      assets: undefined,
      source: payload.source || '',
      mitigations: [],
    };

    set((state) => ({ risks: [optimistic, ...state.risks], total: state.total +  }));
    try {
      const response = await api.post('/risks', payload);
      const created = response.data;
      // replace temp item with created item from server
      set((state) => ({ risks: state.risks.map((r) => (r.id === tempId ? created : r)) }));
    } catch (err) {
      // rollback optimistic add
      set((state) => ({ risks: state.risks.filter((r) => r.id !== tempId), total: Math.max(, state.total - ) }));
      console.error('Failed to create risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
  updateRisk: async (id, payload) => {
    set({ isLoading: true });
    const prev = get().risks.find((r) => r.id === id);
    // apply optimistic patch
    set((state) => ({ risks: state.risks.map((r) => (r.id === id ? { ...r, ...payload } : r)) }));
    try {
      const response = await api.patch(/risks/${id}, payload);
      const updated = response.data;
      set((state) => ({ risks: state.risks.map((r) => (r.id === id ? updated : r)) }));
    } catch (err) {
      // rollback
      if (prev) set((state) => ({ risks: state.risks.map((r) => (r.id === id ? prev : r)) }));
      console.error('Failed to update risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
  deleteRisk: async (id) => {
    set({ isLoading: true });
    const prevList = get().risks;
    const prevTotal = get().total;
    // optimistic remove
    set((state) => ({ risks: state.risks.filter((r) => r.id !== id), total: Math.max(, state.total - ) }));
    try {
      await api.delete(/risks/${id});
    } catch (err) {
      // rollback
      set({ risks: prevList, total: prevTotal });
      console.error('Failed to delete risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },
}));