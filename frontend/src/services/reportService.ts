// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';
import type {
  Report,
  ReportCatalogue,
  ReportLifecycle,
  ReportVerification,
  ReportVersionDiff,
  CreateReportInput,
  ReportType,
} from '../types/report';

export interface ReportListFilter {
  type?: ReportType;
  lifecycle?: ReportLifecycle;
  limit?: number;
  offset?: number;
  /** '-created_at' (default, newest first) or 'created_at'. */
  sort?: string;
}

export const reportService = {
  /**
   * What the configurator offers.
   *
   * Read from the server rather than hard-coded here, so the picker can never
   * offer a format the engine would refuse on submit.
   */
  async catalogue(locale: string): Promise<ReportCatalogue> {
    const { data } = await api.get<ReportCatalogue>(
      `/reports/types?locale=${encodeURIComponent(locale)}`,
    );
    return data;
  },

  async list(filter: ReportListFilter = {}): Promise<{ items: Report[]; total: number }> {
    const params = new URLSearchParams();
    if (filter.type) params.set('type', filter.type);
    if (filter.lifecycle) params.set('lifecycle', filter.lifecycle);
    params.set('limit', String(filter.limit ?? 20));
    if (filter.offset) params.set('offset', String(filter.offset));
    params.set('sort', filter.sort ?? '-created_at');
    const { data } = await api.get<{ items: Report[]; total: number }>(
      `/reports?${params.toString()}`,
    );
    return data;
  },

  async get(id: string): Promise<Report> {
    const { data } = await api.get<Report>(`/reports/${id}`);
    return data;
  },

  /** Queues the report. Returns immediately with an address to watch. */
  async create(input: CreateReportInput): Promise<Report> {
    const { data } = await api.post<Report>('/reports', input);
    return data;
  },

  async transition(id: string, to: ReportLifecycle, comment?: string): Promise<Report> {
    const { data } = await api.post<Report>(`/reports/${id}/transition`, { to, comment });
    return data;
  },

  async comment(id: string, body: string): Promise<void> {
    await api.post(`/reports/${id}/comments`, { body });
  },

  async remove(id: string): Promise<void> {
    await api.delete(`/reports/${id}`);
  },

  async verify(id: string): Promise<ReportVerification> {
    const { data } = await api.get<ReportVerification>(`/reports/${id}/verify`);
    return data;
  },

  async versions(id: string): Promise<Report[]> {
    const { data } = await api.get<{ versions: Report[] }>(`/reports/${id}/versions`);
    return data.versions ?? [];
  },

  async compare(id: string, withId: string): Promise<ReportVersionDiff> {
    const { data } = await api.get<ReportVersionDiff>(
      `/reports/${id}/compare?with=${encodeURIComponent(withId)}`,
    );
    return data;
  },

  /**
   * Downloads the document and hands the browser a save dialog.
   *
   * The hash the server recorded travels in a response header, so the caller can
   * show what it downloaded without a second request.
   */
  async download(id: string, filename?: string): Promise<{ contentHash: string | null }> {
    const response = await api.get<Blob>(`/reports/${id}/download`, { responseType: 'blob' });
    const url = URL.createObjectURL(response.data);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename || 'rapport';
    document.body.appendChild(a);
    a.click();
    a.remove();
    // Revoked on the next tick: revoking synchronously can cancel the download
    // in some browsers before it has read the blob.
    setTimeout(() => URL.revokeObjectURL(url), 0);

    const headers = response.headers as Record<string, string> | undefined;
    return { contentHash: headers?.['x-content-sha256'] ?? null };
  },
};
