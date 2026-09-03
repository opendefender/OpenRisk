// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../../lib/api';
import type { AssetTopology, CompromiseChain } from './topologyTypes';

export const topologyService = {
  get: async (limit?: number): Promise<AssetTopology> => {
    const { data } = await api.get<AssetTopology>('/attack-surface/topology', {
      params: limit ? { limit } : undefined,
    });
    return data;
  },

  compromiseChain: async (assetId: string): Promise<CompromiseChain> => {
    const { data } = await api.get<CompromiseChain>(
      `/attack-surface/topology/${assetId}/compromise-chain`,
    );
    return data;
  },
};
