// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { create } from 'zustand';

// UI state only — server data (frameworks, controls, evidence) lives in
// React Query via useCompliance.ts, never duplicated here.
interface ComplianceUIStore {
  selectedFrameworkId: string | null;
  isCreateFrameworkModalOpen: boolean;
  isCreateControlModalOpen: boolean;
  isControlDrawerOpen: boolean;
  activeControlId: string | null;
  activeDrawerTab: 'details' | 'evidence';

  selectFramework: (frameworkId: string | null) => void;
  openCreateFrameworkModal: () => void;
  closeCreateFrameworkModal: () => void;
  openCreateControlModal: () => void;
  closeCreateControlModal: () => void;
  openControlDrawer: (controlId: string) => void;
  closeControlDrawer: () => void;
  setActiveDrawerTab: (tab: ComplianceUIStore['activeDrawerTab']) => void;
}

export const useComplianceUIStore = create<ComplianceUIStore>((set) => ({
  selectedFrameworkId: null,
  isCreateFrameworkModalOpen: false,
  isCreateControlModalOpen: false,
  isControlDrawerOpen: false,
  activeControlId: null,
  activeDrawerTab: 'details',

  selectFramework: (frameworkId) => set({ selectedFrameworkId: frameworkId }),
  openCreateFrameworkModal: () => set({ isCreateFrameworkModalOpen: true }),
  closeCreateFrameworkModal: () => set({ isCreateFrameworkModalOpen: false }),
  openCreateControlModal: () => set({ isCreateControlModalOpen: true }),
  closeCreateControlModal: () => set({ isCreateControlModalOpen: false }),
  openControlDrawer: (controlId) =>
    set({ isControlDrawerOpen: true, activeControlId: controlId, activeDrawerTab: 'details' }),
  closeControlDrawer: () => set({ isControlDrawerOpen: false, activeControlId: null }),
  setActiveDrawerTab: (tab) => set({ activeDrawerTab: tab }),
}));
