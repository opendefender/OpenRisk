// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
