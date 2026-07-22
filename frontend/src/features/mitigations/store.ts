// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';
import type {
  Mitigation,
  MitigationFilters,
  MitigationUIState,
} from '../../types/mitigation';

interface MitigationStore extends MitigationUIState {
  mitigations: Mitigation[];
  isLoading: boolean;
  error: string | null;
  
  // View persistence
  setViewMode: (mode: 'kanban' | 'table' | 'gantt') => void;
  
  // Drawer control
  openDrawer: (mitigationId: string) => void;
  closeDrawer: () => void;
  setActiveTab: (tab: MitigationUIState['activeTab']) => void;
  
  // Filters
  setFilters: (filters: Partial<MitigationFilters>) => void;
  clearFilters: () => void;
  
  // Selection
  toggleSelection: (mitigationId: string) => void;
  setSelectedIds: (ids: string[]) => void;
  clearSelection: () => void;
  
  // Data state
  setMitigations: (mitigations: Mitigation[]) => void;
  updateMitigation: (id: string, partial: Partial<Mitigation>) => void;
  setLoading: (isLoading: boolean) => void;
  setError: (error: string | null) => void;
}

const getInitialViewMode = (): 'kanban' | 'table' | 'gantt' => {
  if (typeof window === 'undefined') return 'kanban';
  const saved = localStorage.getItem('mitigation_view_mode');
  return (saved as 'kanban' | 'table' | 'gantt') || 'kanban';
};

export const useMitigationStore = create<MitigationStore>((set, get) => ({
  // Initial UI state
  selectedMitigationId: null,
  isDrawerOpen: false,
  activeTab: 'overview',
  filters: {},
  viewMode: getInitialViewMode(),
  selectedIds: [],
  
  // Data state
  mitigations: [],
  isLoading: false,
  error: null,
  
  // View persistence
  setViewMode: (mode) => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('mitigation_view_mode', mode);
    }
    set({ viewMode: mode });
  },
  
  // Drawer control
  openDrawer: (mitigationId) => {
    set({
      isDrawerOpen: true,
      selectedMitigationId: mitigationId,
      activeTab: 'overview',
    });
  },
  
  closeDrawer: () => {
    set({
      isDrawerOpen: false,
      selectedMitigationId: null,
    });
  },
  
  setActiveTab: (tab) => {
    set({ activeTab: tab });
  },
  
  // Filters
  setFilters: (filters) => {
    set((state) => ({
      filters: { ...state.filters, ...filters },
    }));
  },
  
  clearFilters: () => {
    set({ filters: {} });
  },
  
  // Selection
  toggleSelection: (mitigationId) => {
    set((state) => {
      const next = state.selectedIds.includes(mitigationId)
        ? state.selectedIds.filter((id) => id !== mitigationId)
        : [...state.selectedIds, mitigationId];
      return { selectedIds: next };
    });
  },
  
  setSelectedIds: (ids) => {
    set({ selectedIds: ids });
  },
  
  clearSelection: () => {
    set({ selectedIds: [] });
  },
  
  // Data state
  setMitigations: (mitigations) => {
    set({ mitigations });
  },
  
  updateMitigation: (id, partial) => {
    set((state) => ({
      mitigations: state.mitigations.map((m) =>
        m.id === id ? { ...m, ...partial } : m
      ),
    }));
  },
  
  setLoading: (isLoading) => {
    set({ isLoading });
  },
  
  setError: (error) => {
    set({ error });
  },
}));
