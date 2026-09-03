// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../../lib/api';
import type { AssetCategory, AssetTypeSchema, UpdateAssetTypeSchemaInput } from './schemaTypes';

interface SchemaListResponse {
  schemas: AssetTypeSchema[];
  categories: AssetCategory[];
}

export const assetSchemaService = {
  list: async (): Promise<AssetTypeSchema[]> => {
    const { data } = await api.get<SchemaListResponse>('/attack-surface/schemas');
    return data.schemas ?? [];
  },

  get: async (category: AssetCategory): Promise<AssetTypeSchema> => {
    const { data } = await api.get<AssetTypeSchema>(`/attack-surface/schemas/${category}`);
    return data;
  },

  update: async (
    category: AssetCategory,
    payload: UpdateAssetTypeSchemaInput,
  ): Promise<AssetTypeSchema> => {
    const { data } = await api.put<AssetTypeSchema>(`/attack-surface/schemas/${category}`, payload);
    return data;
  },

  reset: async (category: AssetCategory): Promise<AssetTypeSchema> => {
    const { data } = await api.post<AssetTypeSchema>(
      `/attack-surface/schemas/${category}/reset`,
      {},
    );
    return data;
  },
};
