// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';
import { api } from '../lib/api';

interface DashboardStats {
  total_risks: number;
  global_risk_score: number;
  high_risks: number;
  mitigated_risks: number;
  risks_by_severity: Record<string, number>;
}

interface DashboardStore {
  stats: DashboardStats | null;
  isLoading: boolean;
  fetchStats: () => Promise<void>;
}

export const useDashboardStore = create<DashboardStore>((set) => ({
  stats: null,
  isLoading: false,
  fetchStats: async () => {
    set({ isLoading: true });
    try {
      const response = await api.get('/stats');
      set({ stats: response.data });
    } catch (error) {
      console.error('Failed to fetch dashboard stats', error);
    } finally {
      set({ isLoading: false });
    }
  },
}));