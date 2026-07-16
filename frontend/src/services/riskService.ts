// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { api } from '../lib/api';

export type RiskStatus = 'open' | 'in_progress' | 'mitigated' | 'accepted' | 'closed';
export type RiskLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
// ISO 31000 lifecycle phases (Identifier → Analyser → Évaluer → Traiter → Surveiller → Clôturer).
export type RiskPhase = 'identified' | 'analyzed' | 'evaluated' | 'treated' | 'monitored' | 'closed';

export interface Risk {
  id: string;
  title: string;
  description: string;
  score: number;
  impact: number;
  probability: number;
  status: RiskStatus;
  lifecycle_phase?: RiskPhase;
  level?: RiskLevel;
  tags?: string[];
  frameworks?: string[];
  assets?: Asset[];
  assigned_to?: string;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
  source?: string;
  mitigations?: Mitigation[];
  residual_risk?: number;
  // Cyber Risk Quantification (CRQ). Inputs in XAF; ALE returned in XAF + USD.
  sle_xaf?: number | null;
  aro?: number | null;
  ale_xaf?: number;
  ale_usd?: number;
  ale_basis?: 'explicit' | 'reference';
  // Review cadence.
  review_interval_days?: number;
  next_review_at?: string | null;
  last_reviewed_at?: string | null;
}

export interface Asset {
  id: string;
  name: string;
  type: string;
  criticality: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  owner?: string;
}

export interface Mitigation {
  id: string;
  title: string;
  status: 'PLANNED' | 'IN_PROGRESS' | 'DONE';
  progress: number;
  assignee?: string;
}

export interface RiskListResponse {
  items: Risk[];
  total: number;
}

export interface RiskQueryParams {
  q?: string;
  status?: RiskStatus;
  min_score?: number;
  max_score?: number;
  framework?: string;
  assigned_to?: string;
  created_by?: string;
  source?: string;
  tag?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_dir?: 'asc' | 'desc';
}

export interface CreateRiskInput {
  title: string;
  description: string;
  probability: number;
  impact: number;
  asset_criticality?: number;
  framework?: string;
  tags?: string[];
  asset_ids?: string[];
  source?: string;
  status?: RiskStatus;
}

export interface UpdateRiskInput {
  title?: string;
  description?: string;
  probability?: number;
  impact?: number;
  asset_criticality?: number;
  framework?: string;
  tags?: string[];
  asset_ids?: string[];
  status?: RiskStatus;
  sle_xaf?: number | null;
  aro?: number | null;
  review_interval_days?: number;
}

export interface BulkRiskActionInput {
  action: 'change_status' | 'assign_to' | 'add_tags' | 'delete';
  risk_ids: string[];
  payload?: {
    status?: RiskStatus;
    assignee?: string;
    tags?: string[];
  };
}

export const riskService = {
  listRisks: async (params: RiskQueryParams): Promise<RiskListResponse> => {
    const response = await api.get<RiskListResponse>('/risks', { params });
    return response.data;
  },

  getRisk: async (id: string): Promise<Risk> => {
    const response = await api.get<Risk>(`/risks/${id}`);
    return response.data;
  },

  createRisk: async (payload: CreateRiskInput): Promise<Risk> => {
    const response = await api.post<Risk>('/risks', payload);
    return response.data;
  },

  updateRisk: async (id: string, payload: UpdateRiskInput): Promise<Risk> => {
    const response = await api.patch<Risk>(`/risks/${id}`, payload);
    return response.data;
  },

  deleteRisk: async (id: string): Promise<void> => {
    await api.delete(`/risks/${id}`);
  },

  markReviewed: async (id: string): Promise<Risk> => {
    const response = await api.post<Risk>(`/risks/${id}/review`, {});
    return response.data;
  },

  transitionPhase: async (id: string, phase: RiskPhase, note?: string): Promise<Risk> => {
    const response = await api.post<Risk>(`/risks/${id}/transition`, { phase, note });
    return response.data;
  },

  acceptRisk: async (id: string, justification: string): Promise<Risk> => {
    const response = await api.post<Risk>(`/risks/${id}/accept`, { justification });
    return response.data;
  },

  duplicateRisk: async (id: string): Promise<Risk> => {
    const response = await api.post<Risk>(`/risks/${id}/duplicate`);
    return response.data;
  },

  bulkAction: async (payload: BulkRiskActionInput): Promise<void> => {
    await api.post('/risks/bulk', payload);
  },

  exportRisks: async (params: RiskQueryParams, format: 'csv' | 'json' | 'xlsx' = 'csv') => {
    const response = await api.get<Blob>('/risks/export', {
      params: { ...params, format },
      responseType: 'blob',
    });
    return response.data;
  },

  importRisks: async (formData: FormData) => {
    const response = await api.post('/risks/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  },
};
