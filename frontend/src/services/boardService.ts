// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';
import type {
  BoardReport,
  GenerateBoardReportInput,
  UpdateBoardReportInput,
} from '../types/board';

export const boardService = {
  list: async (): Promise<BoardReport[]> => {
    const response = await api.get<BoardReport[]>('/reports/board');
    return response.data;
  },

  get: async (id: string): Promise<BoardReport> => {
    const response = await api.get<BoardReport>(`/reports/board/${id}`);
    return response.data;
  },

  // generate aggregates the tenant's posture and drafts a report (AI or template
  // narrative). It can take several seconds when the Claude advisor is enabled.
  generate: async (payload: GenerateBoardReportInput): Promise<BoardReport> => {
    const response = await api.post<BoardReport>('/reports/board', payload);
    return response.data;
  },

  update: async (id: string, payload: UpdateBoardReportInput): Promise<BoardReport> => {
    const response = await api.patch<BoardReport>(`/reports/board/${id}`, payload);
    return response.data;
  },

  approve: async (id: string): Promise<BoardReport> => {
    const response = await api.post<BoardReport>(`/reports/board/${id}/approve`, {});
    return response.data;
  },

  remove: async (id: string): Promise<void> => {
    await api.delete(`/reports/board/${id}`);
  },

  // downloadPDF fetches the rendered PDF and triggers a browser download,
  // honoring the server's Content-Disposition filename.
  downloadPDF: async (id: string): Promise<void> => {
    const response = await api.get(`/reports/board/${id}/pdf`, { responseType: 'blob' });

    let filename = 'board-report.pdf';
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
};
