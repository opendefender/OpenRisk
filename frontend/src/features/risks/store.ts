// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { create } from 'zustand';

export type RiskStatus = 'open' | 'in_progress' | 'mitigated' | 'accepted' | 'closed';
export type RiskLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';

export interface RiskFilters {
  q?: string;
  status?: RiskStatus;
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

interface RiskUIStore {
  filters: RiskFilters;
  selectedIds: string[];
  isCreateModalOpen: boolean;
  isDrawerOpen: boolean;
  drawerRiskId?: string | null;
  activeDrawerTab: 'details' | 'score' | 'mitigations' | 'timeline' | 'cti' | 'ai' | 'financial';
  showFilterPanel: boolean;
  setFilters: (filters: Partial<RiskFilters>) => void;
  clearFilters: () => void;
  toggleFilter: (key: keyof RiskFilters, value?: string | number) => void;
  toggleSelection: (riskId: string) => void;
  clearSelection: () => void;
  setSelectedIds: (ids: string[]) => void;
  openCreateModal: () => void;
  closeCreateModal: () => void;
  openDrawer: (riskId: string) => void;
  closeDrawer: () => void;
  setActiveDrawerTab: (tab: RiskUIStore['activeDrawerTab']) => void;
  setShowFilterPanel: (value: boolean) => void;
}

export const useRiskUIStore = create<RiskUIStore>((set, get) => ({
  filters: {},
  selectedIds: [],
  isCreateModalOpen: false,
  isDrawerOpen: false,
  drawerRiskId: null,
  activeDrawerTab: 'details',
  showFilterPanel: false,

  setFilters: (filters) => set((state) => ({ filters: { ...state.filters, ...filters } })),
  clearFilters: () => set({ filters: {} }),
  toggleFilter: (key, value) => {
    set((state) => {
      const existing = state.filters[key];
      if (existing === value || value === undefined) {
        const next = { ...state.filters };
        delete next[key];
        return { filters: next };
      }
      return { filters: { ...state.filters, [key]: value } };
    });
  },
  toggleSelection: (riskId) => {
    set((state) => {
      const next = state.selectedIds.includes(riskId)
        ? state.selectedIds.filter((id) => id !== riskId)
        : [...state.selectedIds, riskId];
      return { selectedIds: next };
    });
  },
  clearSelection: () => set({ selectedIds: [] }),
  setSelectedIds: (ids) => set({ selectedIds: ids }),
  openCreateModal: () => set({ isCreateModalOpen: true }),
  closeCreateModal: () => set({ isCreateModalOpen: false }),
  openDrawer: (riskId) => set({ isDrawerOpen: true, drawerRiskId: riskId }),
  closeDrawer: () => set({ isDrawerOpen: false, drawerRiskId: null }),
  setActiveDrawerTab: (tab) => set({ activeDrawerTab: tab }),
  setShowFilterPanel: (value) => set({ showFilterPanel: value }),
}));
