// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';
import type {
  Evidence,
  EvidenceFilter,
  EvidenceListResult,
  CreateEvidenceInput,
  UpdateEvidenceInput,
  FrameworkEvidenceCoverage,
} from '../types/evidence';

export const evidenceService = {
  async list(filter: EvidenceFilter = {}): Promise<EvidenceListResult> {
    const params = new URLSearchParams();
    if (filter.type) params.set('type', filter.type);
    if (filter.review) params.set('review', filter.review);
    if (filter.control_id) params.set('control_id', filter.control_id);
    if (filter.framework_id) params.set('framework_id', filter.framework_id);
    if (filter.q) params.set('q', filter.q);
    if (filter.limit) params.set('limit', String(filter.limit));
    if (filter.offset) params.set('offset', String(filter.offset));
    const qs = params.toString();
    const { data } = await api.get<EvidenceListResult>(`/evidence${qs ? `?${qs}` : ''}`);
    return data;
  },

  async get(id: string): Promise<Evidence> {
    const { data } = await api.get<Evidence>(`/evidence/${id}`);
    return data;
  },

  /**
   * Records a new artifact.
   *
   * Multipart when there is a file, JSON otherwise: evidence can legitimately be
   * a link to a system of record or a written statement, and forcing an upload
   * would make people attach a screenshot of a page instead of pointing at it.
   */
  async create(input: CreateEvidenceInput): Promise<Evidence> {
    if (input.file) {
      const form = new FormData();
      form.append('file', input.file);
      form.append('title', input.title);
      if (input.type) form.append('type', input.type);
      if (input.description) form.append('description', input.description);
      if (input.external_url) form.append('external_url', input.external_url);
      if (input.collected_at) form.append('collected_at', input.collected_at);
      if (input.valid_until) form.append('valid_until', input.valid_until);
      if (input.control_ids?.length) form.append('control_ids', input.control_ids.join(','));
      const { data } = await api.post<Evidence>('/evidence', form);
      return data;
    }
    const { data } = await api.post<Evidence>('/evidence', {
      title: input.title,
      type: input.type,
      description: input.description,
      external_url: input.external_url,
      collected_at: input.collected_at,
      valid_until: input.valid_until,
      control_ids: input.control_ids ?? [],
    });
    return data;
  },

  async update(id: string, input: UpdateEvidenceInput): Promise<Evidence> {
    const { data } = await api.patch<Evidence>(`/evidence/${id}`, input);
    return data;
  },

  /** Reuse: attach an artifact the tenant already holds to further controls. */
  async link(id: string, controlIds: string[], note?: string): Promise<Evidence> {
    const { data } = await api.post<Evidence>(`/evidence/${id}/links`, {
      control_ids: controlIds,
      note,
    });
    return data;
  },

  async unlink(id: string, controlId: string): Promise<void> {
    await api.delete(`/evidence/${id}/links/${controlId}`);
  },

  /** A rejection must carry a reason; the server refuses one without. */
  async review(
    id: string,
    review: 'accepted' | 'rejected' | 'pending',
    note?: string,
  ): Promise<Evidence> {
    const { data } = await api.post<Evidence>(`/evidence/${id}/review`, { review, note });
    return data;
  },

  async remove(id: string): Promise<void> {
    await api.delete(`/evidence/${id}`);
  },

  /** The worklist: what proof is missing, and whether it is absent or stale. */
  async missing(frameworkId?: string): Promise<FrameworkEvidenceCoverage[]> {
    const qs = frameworkId ? `?framework_id=${encodeURIComponent(frameworkId)}` : '';
    const { data } = await api.get<{ frameworks: FrameworkEvidenceCoverage[] }>(
      `/evidence/missing${qs}`,
    );
    return data.frameworks ?? [];
  },

  /** The artifact's bytes. Blob, so the caller can save or preview it. */
  async download(id: string): Promise<Blob> {
    const { data } = await api.get<Blob>(`/evidence/${id}/download`, { responseType: 'blob' });
    return data;
  },
};
