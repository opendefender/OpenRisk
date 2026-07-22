// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { create } from 'zustand';

// UI state only — server data (frameworks, controls, evidence) lives in
// React Query via useCompliance.ts, never duplicated here.
interface ComplianceUIStore {
  selectedFrameworkId: string | null;
  isCreateFrameworkModalOpen: boolean;
  isCreateControlModalOpen: boolean;
  isImportCatalogModalOpen: boolean;
  isControlDrawerOpen: boolean;
  activeControlId: string | null;
  activeDrawerTab: 'details' | 'evidence';

  selectFramework: (frameworkId: string | null) => void;
  openCreateFrameworkModal: () => void;
  closeCreateFrameworkModal: () => void;
  openCreateControlModal: () => void;
  closeCreateControlModal: () => void;
  openImportCatalogModal: () => void;
  closeImportCatalogModal: () => void;
  openControlDrawer: (controlId: string) => void;
  closeControlDrawer: () => void;
  setActiveDrawerTab: (tab: ComplianceUIStore['activeDrawerTab']) => void;
}

export const useComplianceUIStore = create<ComplianceUIStore>((set) => ({
  selectedFrameworkId: null,
  isCreateFrameworkModalOpen: false,
  isCreateControlModalOpen: false,
  isImportCatalogModalOpen: false,
  isControlDrawerOpen: false,
  activeControlId: null,
  activeDrawerTab: 'details',

  selectFramework: (frameworkId) => set({ selectedFrameworkId: frameworkId }),
  openCreateFrameworkModal: () => set({ isCreateFrameworkModalOpen: true }),
  closeCreateFrameworkModal: () => set({ isCreateFrameworkModalOpen: false }),
  openCreateControlModal: () => set({ isCreateControlModalOpen: true }),
  closeCreateControlModal: () => set({ isCreateControlModalOpen: false }),
  openImportCatalogModal: () => set({ isImportCatalogModalOpen: true }),
  closeImportCatalogModal: () => set({ isImportCatalogModalOpen: false }),
  openControlDrawer: (controlId) =>
    set({ isControlDrawerOpen: true, activeControlId: controlId, activeDrawerTab: 'details' }),
  closeControlDrawer: () => set({ isControlDrawerOpen: false, activeControlId: null }),
  setActiveDrawerTab: (tab) => set({ activeDrawerTab: tab }),
}));
