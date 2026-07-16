// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { api } from '../lib/api';
import type {
  Asset,
  AssetDependency,
  AssetSnapshot,
  CreateAssetDependencyInput,
  CreateAssetInput,
  UpdateAssetInput,
} from '../types/asset';

export const assetService = {
  listAssets: async (): Promise<Asset[]> => {
    const response = await api.get<Asset[]>('/assets');
    return response.data;
  },

  getAsset: async (id: string): Promise<Asset> => {
    const response = await api.get<Asset>(`/assets/${id}`);
    return response.data;
  },

  createAsset: async (payload: CreateAssetInput): Promise<Asset> => {
    const response = await api.post<Asset>('/assets', payload);
    return response.data;
  },

  updateAsset: async (id: string, payload: UpdateAssetInput): Promise<Asset> => {
    const response = await api.patch<Asset>(`/assets/${id}`, payload);
    return response.data;
  },

  deleteAsset: async (id: string): Promise<void> => {
    await api.delete(`/assets/${id}`);
  },

  getAssetHistory: async (id: string): Promise<AssetSnapshot[]> => {
    const response = await api.get<AssetSnapshot[]>(`/assets/${id}/history`);
    return response.data;
  },

  // --- Dependency graph (cartographie des dépendances) ---
  listDependencies: async (): Promise<AssetDependency[]> => {
    const response = await api.get<AssetDependency[]>('/asset-dependencies');
    return response.data;
  },

  createDependency: async (payload: CreateAssetDependencyInput): Promise<AssetDependency> => {
    const response = await api.post<AssetDependency>('/asset-dependencies', payload);
    return response.data;
  },

  deleteDependency: async (id: string): Promise<void> => {
    await api.delete(`/asset-dependencies/${id}`);
  },
};
