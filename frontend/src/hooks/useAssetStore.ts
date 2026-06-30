// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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