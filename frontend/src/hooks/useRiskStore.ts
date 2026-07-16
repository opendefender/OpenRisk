// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

// The six ISO 31000 lifecycle phases surfaced in the register's "Cycle de vie" stepper.
export type RiskPhase = 'identified' | 'analyzed' | 'evaluated' | 'treated' | 'monitored' | 'closed';

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
  frameworks?: string[];
  assigned_to?: string;
  source: string; // Important pour l'étape d'intégration (THEHIVE, etc.)
  mitigations?: Mitigation[]; // Important pour le drawer de détails
  created_at?: string;
  updated_at?: string;
  level?: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
  // ISO 31000 lifecycle phase (orthogonal to status).
  lifecycle_phase?: RiskPhase;
  // Cyber Risk Quantification (CRQ). Inputs in XAF; ALE returned in XAF + USD.
  sle_xaf?: number | null;
  aro?: number | null;
  ale_xaf?: number;
  ale_usd?: number;
  ale_basis?: 'explicit' | 'reference';
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

interface RiskFilters {
  q?: string;
  status?: string;
  framework?: string;
  assignedTo?: string;
  createdBy?: string;
  source?: string;
  tag?: string;
  minScore?: number;
  maxScore?: number;
  dateFrom?: string;
  dateTo?: string;
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

  // filters & selection for bulk actions
  filters: RiskFilters;
  setFilters: (patch: Partial<RiskFilters>) => void;
  clearFilters: () => void;

  selectedIds: string[];
  setSelectedIds: (ids: string[]) => void;
  toggleSelection: (id: string) => void;
  clearSelection: () => void;

  // bulk operations
  bulkDelete: (ids: string[]) => Promise<void>;
  bulkUpdate: (ids: string[], payload: Partial<Risk>) => Promise<void>;

  // import / export helpers
  importRisks: (file: File) => Promise<void>;
  exportRisks: (params?: Record<string, any>) => Promise<Blob>;

  // realtime
  startSSE: (url?: string) => void;
  stopSSE: () => void;

  fetchRisks: (params?: RiskFetchParams & { page?: number; limit?: number }) => Promise<void>;
  createRisk: (payload: any) => Promise<void>;
  updateRisk: (id: string, payload: any) => Promise<void>;
  transitionPhase: (id: string, phase: RiskPhase, note?: string) => Promise<Risk>;
  deleteRisk: (id: string) => Promise<void>;
}

// --- STORE ZUSTAND ---

export const useRiskStore = create<RiskStore>((set, get) => ({
  risks: [],
  isLoading: false,
  total: 0,
  page: 1,
  pageSize: 20,
  selectedRisk: null,
  filters: {},
  selectedIds: [],
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
    // optimistic create: add a temporary item to the list
    const tempId = `tmp-${Date.now()}`;
    const optimistic: Risk = {
      id: tempId,
      title: payload.title || 'Nouvel élément',
      description: payload.description || '',
      score: payload.score ?? 0,
      impact: payload.impact ?? 0,
      probability: payload.probability ?? 0,
      status: payload.status || 'DRAFT',
      tags: payload.tags || [],
      assets: undefined,
      source: payload.source || '',
      mitigations: [],
    };

    set((state) => ({ risks: [optimistic, ...state.risks], total: state.total + 1 }));
    try {
      const response = await api.post('/risks', payload);
      const created = response.data;
      // replace temp item with created item from server
      set((state) => ({ risks: state.risks.map((r) => (r.id === tempId ? created : r)) }));
    } catch (err) {
      // rollback optimistic add
      set((state) => ({ risks: state.risks.filter((r) => r.id !== tempId), total: Math.max(0, state.total - 1) }));
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
      const response = await api.patch(`/risks/${id}`, payload);
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
  transitionPhase: async (id, phase, note) => {
    const prev = get().risks.find((r) => r.id === id);
    // Optimistic: reflect the new phase immediately in the register.
    set((state) => ({ risks: state.risks.map((r) => (r.id === id ? { ...r, lifecycle_phase: phase } : r)) }));
    try {
      const response = await api.post(`/risks/${id}/transition`, { phase, note });
      const updated = response.data as Risk;
      set((state) => ({ risks: state.risks.map((r) => (r.id === id ? updated : r)) }));
      return updated;
    } catch (err) {
      if (prev) set((state) => ({ risks: state.risks.map((r) => (r.id === id ? prev : r)) }));
      console.error('Failed to transition risk phase', err);
      throw err;
    }
  },
  deleteRisk: async (id) => {
    set({ isLoading: true });
    const prevList = get().risks;
    const prevTotal = get().total;
    // optimistic remove
    set((state) => ({ risks: state.risks.filter((r) => r.id !== id), total: Math.max(0, state.total - 1) }));
    try {
      await api.delete(`/risks/${id}`);
    } catch (err) {
      // rollback
      set({ risks: prevList, total: prevTotal });
      console.error('Failed to delete risk', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },

  // filters & selection implementation
  setFilters: (patch) => set((state) => ({ filters: { ...state.filters, ...patch } })),
  clearFilters: () => set({ filters: {} }),

  setSelectedIds: (ids) => set({ selectedIds: ids }),
  toggleSelection: (id) => set((state) => ({ selectedIds: state.selectedIds.includes(id) ? state.selectedIds.filter((x) => x !== id) : [...state.selectedIds, id] })),
  clearSelection: () => set({ selectedIds: [] }),

  // bulk operations
  bulkDelete: async (ids) => {
    set({ isLoading: true });
    const prev = get().risks;
    const prevTotal = get().total;
    set((state) => ({ risks: state.risks.filter((r) => !ids.includes(r.id)), total: Math.max(0, state.total - ids.length) }));
    try {
      // backend may not provide bulk delete endpoint; call per-id
      await Promise.all(ids.map((id) => api.delete(`/risks/${id}`)));
      // clear selection
      set({ selectedIds: [] });
    } catch (err) {
      set({ risks: prev, total: prevTotal });
      console.error('bulkDelete failed', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },

  bulkUpdate: async (ids, payload) => {
    set({ isLoading: true });
    const prev = get().risks;
    // optimistic update
    set((state) => ({ risks: state.risks.map((r) => (ids.includes(r.id) ? { ...r, ...payload } : r)) }));
    try {
      await Promise.all(ids.map((id) => api.patch(`/risks/${id}`, payload)));
    } catch (err) {
      set({ risks: prev });
      console.error('bulkUpdate failed', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },

  // import/export
  importRisks: async (file) => {
    set({ isLoading: true });
    try {
      const fd = new FormData();
      fd.append('file', file);
      await api.post('/risks/import', fd, { headers: { 'Content-Type': 'multipart/form-data' } });
      // refresh
      await get().fetchRisks({ page: get().page, limit: get().pageSize });
    } catch (err) {
      console.error('importRisks failed', err);
      throw err;
    } finally {
      set({ isLoading: false });
    }
  },

  exportRisks: async (params) => {
    try {
      const res = await api.get('/risks/export', { params, responseType: 'blob' });
      return res.data as Blob;
    } catch (err) {
      console.error('exportRisks failed', err);
      throw err;
    }
  },

  // SSE realtime support
  startSSE: (url = '/api/v1/risks/events') => {
    // avoid multiple event sources
    // store eventSource on closure
    // @ts-ignore - keep internal ref
    if ((get() as any)._es) return;
    try {
      const es = new EventSource(url);
      (set as any)((state: unknown) => state); // noop to satisfy linter
      // save reference
      (get() as any)._es = es;
      es.onmessage = (evt) => {
        try {
          const payload = JSON.parse(evt.data);
          const type = payload.type || payload.event || '';
          const data = payload.data || payload;
          if (type === 'risk.created') {
            set((state) => ({ risks: [data, ...state.risks], total: state.total + 1 }));
          } else if (type === 'risk.updated' || type === 'risk.score_updated') {
            set((state) => ({ risks: state.risks.map((r) => (r.id === data.id ? { ...r, ...data } : r)) }));
          } else if (type === 'risk.deleted') {
            set((state) => ({ risks: state.risks.filter((r) => r.id !== data.id), total: Math.max(0, state.total - 1) }));
          }
        } catch (e) {
          // ignore parse errors
        }
      };
      es.onerror = () => {
        // close on error to avoid retry storms
        try { es.close(); } catch {};
        (get() as any)._es = null;
      };
    } catch (err) {
      console.error('startSSE failed', err);
    }
  },

  stopSSE: () => {
    try {
      const ref = (get() as any)._es as EventSource | undefined | null;
      if (ref) {
        ref.close();
        (get() as any)._es = null;
      }
    } catch (err) {
      console.error('stopSSE failed', err);
    }
  },
}));