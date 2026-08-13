// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';
import type {
  Asset,
  AssetDependency,
  AssetSnapshot,
  CreateAssetDependencyInput,
  CreateAssetInput,
  UpdateAssetInput,
} from '../types/asset';

/**
 * Typed-attribute search. Sent to the server as `?category=&attr.<key>=<value>`
 * so the matching semantics are the server's single implementation
 * (domain.MatchesAttributes) rather than a second copy living in the client.
 */
export interface AssetSearchFilter {
  category?: string;
  /** attribute key → value, AND-ed together. */
  attributes?: Record<string, string>;
}

export const assetService = {
  listAssets: async (filter?: AssetSearchFilter): Promise<Asset[]> => {
    const params: Record<string, string> = {};
    if (filter?.category) params.category = filter.category;
    for (const [k, v] of Object.entries(filter?.attributes ?? {})) {
      if (k.trim() && v.trim()) params[`attr.${k.trim()}`] = v.trim();
    }
    const response = await api.get<Asset[]>('/assets', { params });
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
