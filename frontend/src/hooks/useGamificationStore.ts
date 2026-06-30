// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { create } from 'zustand';
import { api } from '../lib/api';

interface Badge {
  id: string;
  name: string;
  description: string;
  icon: string;
  unlocked: boolean;
}

interface UserStats {
  total_xp: number;
  level: number;
  next_level_xp: number;
  progress_percent: number;
  risks_managed: number;
  mitigations_done: number;
  badges: Badge[];
}

interface GamificationStore {
  stats: UserStats | null;
  loading: boolean;
  error: string | null;
  fetchStats: () => Promise<void>;
  reset: () => void;
}

export const useGamificationStore = create<GamificationStore>((set) => ({
  stats: null,
  loading: false,
  error: null,

  fetchStats: async () => {
    try {
      set({ loading: true, error: null });
      const response = await api.get('/gamification/me');
      set({ stats: response.data, loading: false });
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to fetch gamification stats';
      set({ error: errorMessage, loading: false });
      console.error('Failed to fetch gamification stats:', err);
    }
  },

  reset: () => {
    set({ stats: null, loading: false, error: null });
  },
}));
