// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { api } from '../lib/api';
import type {
  ComplianceFramework,
  ComplianceControl,
  ControlEvidence,
  ComplianceProgress,
  CreateFrameworkInput,
  CreateControlInput,
  UpdateControlInput,
  ComplianceCatalogSummary,
  ImportCatalogInput,
  ImportCatalogResult,
} from '../types/compliance';

export const complianceService = {
  listCatalogs: async (): Promise<ComplianceCatalogSummary[]> => {
    const response = await api.get<ComplianceCatalogSummary[]>('/compliance/catalogs');
    return response.data;
  },

  importCatalog: async (frameworkId: string, payload: ImportCatalogInput): Promise<ImportCatalogResult> => {
    const response = await api.post<ImportCatalogResult>(`/compliance/frameworks/${frameworkId}/import-catalog`, payload);
    return response.data;
  },

  listFrameworks: async (): Promise<ComplianceFramework[]> => {
    const response = await api.get<ComplianceFramework[]>('/compliance/frameworks');
    return response.data;
  },

  createFramework: async (payload: CreateFrameworkInput): Promise<ComplianceFramework> => {
    const response = await api.post<ComplianceFramework>('/compliance/frameworks', payload);
    return response.data;
  },

  getFramework: async (frameworkId: string): Promise<ComplianceFramework> => {
    const response = await api.get<ComplianceFramework>(`/compliance/frameworks/${frameworkId}`);
    return response.data;
  },

  getProgress: async (frameworkId: string): Promise<ComplianceProgress> => {
    const response = await api.get<ComplianceProgress>(`/compliance/frameworks/${frameworkId}/progress`);
    return response.data;
  },

  listControls: async (frameworkId: string): Promise<ComplianceControl[]> => {
    const response = await api.get<ComplianceControl[]>(`/compliance/frameworks/${frameworkId}/controls`);
    return response.data;
  },

  createControl: async (frameworkId: string, payload: CreateControlInput): Promise<ComplianceControl> => {
    const response = await api.post<ComplianceControl>(`/compliance/frameworks/${frameworkId}/controls`, payload);
    return response.data;
  },

  getControl: async (controlId: string): Promise<ComplianceControl> => {
    const response = await api.get<ComplianceControl>(`/compliance/controls/${controlId}`);
    return response.data;
  },

  updateControl: async (controlId: string, payload: UpdateControlInput): Promise<ComplianceControl> => {
    const response = await api.patch<ComplianceControl>(`/compliance/controls/${controlId}`, payload);
    return response.data;
  },

  deleteControl: async (controlId: string): Promise<void> => {
    await api.delete(`/compliance/controls/${controlId}`);
  },

  listEvidences: async (controlId: string): Promise<ControlEvidence[]> => {
    const response = await api.get<ControlEvidence[]>(`/compliance/controls/${controlId}/evidences`);
    return response.data;
  },

  createEvidence: async (controlId: string, file: File, description?: string): Promise<ControlEvidence> => {
    const formData = new FormData();
    formData.append('file', file);
    if (description) formData.append('description', description);
    const response = await api.post<ControlEvidence>(`/compliance/controls/${controlId}/evidences`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },

  downloadEvidence: async (evidenceId: string, filename: string): Promise<void> => {
    const response = await api.get(`/compliance/evidences/${evidenceId}/download`, { responseType: 'blob' });
    const url = URL.createObjectURL(response.data as Blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  },

  deleteEvidence: async (evidenceId: string): Promise<void> => {
    await api.delete(`/compliance/evidences/${evidenceId}`);
  },
};
