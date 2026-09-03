// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for report jobs — the addressable form of "generate a report".

import { api } from '../../lib/api';

export type ReportKind = 'compliance_framework';
export type ReportJobStatus = 'queued' | 'running' | 'succeeded' | 'failed';

export interface ReportJob {
  id: string;
  tenant_id: string;
  kind: ReportKind;
  status: ReportJobStatus;
  params?: Record<string, unknown>;
  title: string;
  filename?: string;
  content_type?: string;
  size_bytes?: number;
  error?: string;
  requested_by: string;
  created_at: string;
  completed_at?: string;
}

export interface CreateReportJobInput {
  kind: ReportKind;
  params: Record<string, unknown>;
}

export const reportJobService = {
  async create(input: CreateReportJobInput): Promise<ReportJob> {
    const { data } = await api.post<ReportJob>('/reports/jobs', input);
    return data;
  },

  async get(id: string): Promise<ReportJob> {
    const { data } = await api.get<ReportJob>(`/reports/jobs/${id}`);
    return data;
  },

  async list(limit = 25): Promise<ReportJob[]> {
    const { data } = await api.get<{ data: ReportJob[] | null }>('/reports/jobs', {
      params: { limit },
    });
    return data.data ?? [];
  },

  /** Downloads the stored artifact — the document as generated, not a re-render. */
  async download(job: ReportJob): Promise<void> {
    const res = await api.get(`/reports/jobs/${job.id}/download`, { responseType: 'blob' });
    const url = URL.createObjectURL(res.data as Blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = job.filename || 'report.pdf';
    a.click();
    URL.revokeObjectURL(url);
  },
};
