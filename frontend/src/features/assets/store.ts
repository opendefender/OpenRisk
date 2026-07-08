// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { create } from 'zustand';

// UI state only — server data (assets, history) lives in React Query via
// useAssets.ts, never duplicated here.
interface AssetUIStore {
  isCreateModalOpen: boolean;
  editingAssetId: string | null;
  historyAssetId: string | null;
  view: 'table' | 'card';

  openCreateModal: () => void;
  closeCreateModal: () => void;
  openEditModal: (assetId: string) => void;
  closeEditModal: () => void;
  openHistoryDrawer: (assetId: string) => void;
  closeHistoryDrawer: () => void;
  setView: (view: 'table' | 'card') => void;
}

export const useAssetUIStore = create<AssetUIStore>((set) => ({
  isCreateModalOpen: false,
  editingAssetId: null,
  historyAssetId: null,
  view: (localStorage.getItem('assetView') as 'table' | 'card') || 'table',

  openCreateModal: () => set({ isCreateModalOpen: true }),
  closeCreateModal: () => set({ isCreateModalOpen: false }),
  openEditModal: (assetId) => set({ editingAssetId: assetId }),
  closeEditModal: () => set({ editingAssetId: null }),
  openHistoryDrawer: (assetId) => set({ historyAssetId: assetId }),
  closeHistoryDrawer: () => set({ historyAssetId: null }),
  setView: (view) => {
    localStorage.setItem('assetView', view);
    set({ view });
  },
}));
