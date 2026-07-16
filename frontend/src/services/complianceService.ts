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
  GapAnalysis,
  ComplianceAudit,
  CreateAuditInput,
  UpdateAuditInput,
  RemediationPlan,
  CreateRemediationInput,
  UpdateRemediationInput,
  RemediationFilter,
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

  deleteFramework: async (frameworkId: string): Promise<void> => {
    await api.delete(`/compliance/frameworks/${frameworkId}`);
  },

  getProgress: async (frameworkId: string): Promise<ComplianceProgress> => {
    const response = await api.get<ComplianceProgress>(`/compliance/frameworks/${frameworkId}/progress`);
    return response.data;
  },

  // downloadReport fetches the official compliance report (PDF) for a framework
  // and triggers a browser download. The server sets a descriptive filename via
  // Content-Disposition; we honor it, falling back to a sane default.
  downloadReport: async (frameworkId: string, locale: string): Promise<void> => {
    const response = await api.get(`/compliance/frameworks/${frameworkId}/report`, {
      params: { locale },
      responseType: 'blob',
    });

    let filename = 'compliance-report.pdf';
    const disposition = response.headers?.['content-disposition'] as string | undefined;
    const match = disposition?.match(/filename\*?=(?:UTF-8'')?"?([^";]+)"?/i);
    if (match?.[1]) filename = decodeURIComponent(match[1]);

    const url = URL.createObjectURL(response.data as Blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  },

  // getGapAnalysis returns every unsatisfied control across the tenant's
  // frameworks (or a single framework when frameworkId is provided), with
  // per-framework roll-ups. Backs the "Analyse d'écarts" screen.
  getGapAnalysis: async (frameworkId?: string): Promise<GapAnalysis> => {
    const response = await api.get<GapAnalysis>('/compliance/gap-analysis', {
      params: frameworkId ? { framework_id: frameworkId } : undefined,
    });
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

  // --- Audits ---------------------------------------------------------------
  listAudits: async (): Promise<ComplianceAudit[]> => {
    const response = await api.get<ComplianceAudit[]>('/compliance/audits');
    return response.data;
  },
  createAudit: async (payload: CreateAuditInput): Promise<ComplianceAudit> => {
    const response = await api.post<ComplianceAudit>('/compliance/audits', payload);
    return response.data;
  },
  getAudit: async (id: string): Promise<ComplianceAudit> => {
    const response = await api.get<ComplianceAudit>(`/compliance/audits/${id}`);
    return response.data;
  },
  updateAudit: async (id: string, payload: UpdateAuditInput): Promise<ComplianceAudit> => {
    const response = await api.patch<ComplianceAudit>(`/compliance/audits/${id}`, payload);
    return response.data;
  },
  deleteAudit: async (id: string): Promise<void> => {
    await api.delete(`/compliance/audits/${id}`);
  },

  // --- Remediation plans ----------------------------------------------------
  listRemediations: async (filter?: RemediationFilter): Promise<RemediationPlan[]> => {
    const response = await api.get<RemediationPlan[]>('/compliance/remediations', { params: filter });
    return response.data;
  },
  createRemediation: async (payload: CreateRemediationInput): Promise<RemediationPlan> => {
    const response = await api.post<RemediationPlan>('/compliance/remediations', payload);
    return response.data;
  },
  updateRemediation: async (id: string, payload: UpdateRemediationInput): Promise<RemediationPlan> => {
    const response = await api.patch<RemediationPlan>(`/compliance/remediations/${id}`, payload);
    return response.data;
  },
  deleteRemediation: async (id: string): Promise<void> => {
    await api.delete(`/compliance/remediations/${id}`);
  },
};
