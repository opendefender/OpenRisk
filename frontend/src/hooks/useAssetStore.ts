// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';
import { api } from '../lib/api';
import type { Risk } from './useRiskStore';

export interface Asset {
  id: string;
  name: string;
  type: string;
  criticality: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  owner: string;
  risks?: Risk[];
  source: string;
}

interface AssetStore {
  assets: Asset[];
  isLoading: boolean;
  fetchAssets: () => Promise<void>;
  createAsset: (asset: Partial<Asset>) => Promise<void>;
}

export const useAssetStore = create<AssetStore>((set, get) => ({
  assets: [],
  isLoading: false,
  fetchAssets: async () => {
    set({ isLoading: true });
    try {
      const { data } = await api.get('/assets');
      set({ assets: data });
    } catch (e) { console.error(e); } 
    finally { set({ isLoading: false }); }
  },
  createAsset: async (newAsset) => {
      await api.post('/assets', newAsset);
      get().fetchAssets();
  }
}));