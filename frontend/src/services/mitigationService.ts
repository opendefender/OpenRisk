// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { api } from '../lib/api';
import type {
  Mitigation,
  MitigationListResponse,
  MitigationQueryParams,
  CreateMitigationInput,
  UpdateMitigationInput,
  CreateSubActionInput,
  UpdateSubActionInput,
  RevertSubActionInput,
  BulkMitigationActionInput,
  RescanInput,
  Evidence,
  TimelineEvent,
  SubAction,
} from '../types/mitigation';

export const mitigationService = {
  /**
   * List mitigations with filters and pagination
   */
  listMitigations: async (params: MitigationQueryParams): Promise<MitigationListResponse> => {
    const response = await api.get<MitigationListResponse>('/mitigations', { params });
    return response.data;
  },

  /**
   * Get a single mitigation by ID
   */
  getMitigation: async (id: string): Promise<Mitigation> => {
    const response = await api.get<Mitigation>(`/mitigations/${id}`);
    return response.data;
  },

  /**
   * Create a new mitigation
   */
  createMitigation: async (payload: CreateMitigationInput): Promise<Mitigation> => {
    const response = await api.post<Mitigation>('/mitigations', payload);
    return response.data;
  },

  /**
   * Update a mitigation
   */
  updateMitigation: async (id: string, payload: UpdateMitigationInput): Promise<Mitigation> => {
    const response = await api.patch<Mitigation>(`/mitigations/${id}`, payload);
    return response.data;
  },

  /**
   * Delete a mitigation
   */
  deleteMitigation: async (id: string): Promise<void> => {
    await api.delete(`/mitigations/${id}`);
  },

  /**
   * Bulk operations on mitigations
   */
  bulkAction: async (payload: BulkMitigationActionInput): Promise<void> => {
    await api.post('/mitigations/bulk', payload);
  },

  /**
   * Get all sub-actions for a mitigation
   */
  getSubActions: async (mitigationId: string): Promise<SubAction[]> => {
    const response = await api.get<SubAction[]>(`/mitigations/${mitigationId}/sub-actions`);
    return response.data;
  },

  /**
   * Create a sub-action
   */
  createSubAction: async (payload: CreateSubActionInput): Promise<SubAction> => {
    const response = await api.post<SubAction>(
      `/mitigations/${payload.mitigation_id}/sub-actions`,
      {
        title: payload.title,
        description: payload.description,
        depends_on: payload.depends_on,
      }
    );
    return response.data;
  },

  /**
   * Update a sub-action
   */
  updateSubAction: async (
    mitigationId: string,
    subActionId: string,
    payload: UpdateSubActionInput
  ): Promise<SubAction> => {
    const response = await api.patch<SubAction>(
      `/mitigations/${mitigationId}/sub-actions/${subActionId}`,
      payload
    );
    return response.data;
  },

  /**
   * Delete a sub-action
   */
  deleteSubAction: async (mitigationId: string, subActionId: string): Promise<void> => {
    await api.delete(`/mitigations/${mitigationId}/sub-actions/${subActionId}`);
  },

  /**
   * Reorder sub-actions
   */
  reorderSubActions: async (mitigationId: string, subActionIds: string[]): Promise<void> => {
    await api.post(`/mitigations/${mitigationId}/sub-actions/reorder`, {
      order: subActionIds,
    });
  },

  /**
   * Revert an auto-detected sub-action
   */
  revertSubAction: async (mitigationId: string, subActionId: string): Promise<void> => {
    await api.post(`/mitigations/${mitigationId}/sub-actions/${subActionId}/revert`);
  },

  /**
   * Trigger a rescan for an asset
   */
  rescan: async (mitigationId: string, payload: RescanInput): Promise<{ scan_job_id: string }> => {
    const response = await api.post<{ scan_job_id: string }>(
      `/mitigations/${mitigationId}/sub-actions/${payload.sub_action_id}/rescan`,
      { asset_id: payload.asset_id }
    );
    return response.data;
  },

  /**
   * Get timeline events for a mitigation
   */
  getTimeline: async (mitigationId: string): Promise<TimelineEvent[]> => {
    const response = await api.get<TimelineEvent[]>(`/mitigations/${mitigationId}/timeline`);
    return response.data;
  },

  /**
   * Get evidence items for a mitigation
   */
  getEvidence: async (mitigationId: string): Promise<Evidence[]> => {
    const response = await api.get<Evidence[]>(`/mitigations/${mitigationId}/evidence`);
    return response.data;
  },

  /**
   * Add evidence to a sub-action
   */
  addEvidence: async (
    mitigationId: string,
    subActionId: string,
    payload: FormData
  ): Promise<Evidence> => {
    const response = await api.post<Evidence>(
      `/mitigations/${mitigationId}/sub-actions/${subActionId}/evidence`,
      payload,
      {
        headers: { 'Content-Type': 'multipart/form-data' },
      }
    );
    return response.data;
  },

  /**
   * Export mitigations
   */
  exportMitigations: async (
    params: MitigationQueryParams,
    format: 'csv' | 'json' | 'xlsx' = 'csv'
  ): Promise<Blob> => {
    const response = await api.get<Blob>('/mitigations/export', {
      params: { ...params, format },
      responseType: 'blob',
    });
    return response.data;
  },
};
