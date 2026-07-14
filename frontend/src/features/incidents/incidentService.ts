// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Typed client for the incident register (/incidents). Hand-written (like the
// mitigations service) — incidents aren't in the generated OpenAPI client. The
// shapes mirror backend/internal/domain/incident.go (uint ids, tenant_id string).

import { api } from '../../lib/api';

export type IncidentStatus = 'open' | 'in_progress' | 'resolved' | 'closed';
export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low';

export interface Incident {
  id: number;
  tenant_id: string;
  title: string;
  description: string;
  incident_type: string;
  severity: IncidentSeverity;
  status: IncidentStatus;
  source: string;
  external_id?: string;
  reported_by: string;
  assigned_to?: string;
  risk_id?: number | null;
  resolution?: string;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface IncidentStats {
  total_incidents: number;
  open_incidents: number;
  resolved_incidents: number;
  critical_incidents: number;
  resolution_rate: number;
}

export interface IncidentListResponse {
  incidents: Incident[] | null;
  total: number;
  limit: number;
  offset: number;
}

export interface IncidentListParams {
  status?: IncidentStatus | '';
  severity?: IncidentSeverity | '';
  type?: string;
  limit?: number;
  offset?: number;
}

export interface CreateIncidentInput {
  title: string;
  description: string;
  incident_type: string;
  severity: IncidentSeverity;
  source: string;
  reported_by: string;
}

export interface UpdateIncidentInput {
  title?: string;
  description?: string;
  status?: IncidentStatus;
  severity?: IncidentSeverity;
  assigned_to?: string;
  resolution?: string;
}

export interface IncidentTimelineEvent {
  id: number;
  incident_id: number;
  event_type: string;
  message: string;
  created_by: string;
  created_at: string;
}

export const incidentService = {
  list: async (params: IncidentListParams = {}): Promise<IncidentListResponse> => {
    const response = await api.get<IncidentListResponse>('/incidents', { params });
    return response.data;
  },

  get: async (id: number): Promise<Incident> => {
    const response = await api.get<Incident>(`/incidents/${id}`);
    return response.data;
  },

  stats: async (): Promise<IncidentStats> => {
    const response = await api.get<IncidentStats>('/incidents/stats');
    return response.data;
  },

  create: async (input: CreateIncidentInput): Promise<Incident> => {
    const response = await api.post<Incident>('/incidents', input);
    return response.data;
  },

  update: async (id: number, input: UpdateIncidentInput): Promise<Incident> => {
    const response = await api.put<Incident>(`/incidents/${id}`, input);
    return response.data;
  },

  remove: async (id: number): Promise<void> => {
    await api.delete(`/incidents/${id}`);
  },

  timeline: async (id: number): Promise<IncidentTimelineEvent[]> => {
    const response = await api.get<IncidentTimelineEvent[]>(`/incidents/${id}/timeline`);
    return response.data;
  },
};

// exportIncidentsCsv fetches the (filtered) incident register and triggers a
// browser CSV download. Built client-side — there is no server CSV endpoint —
// mirroring the risk-register export UX on the Reports screen.
export async function exportIncidentsCsv(params: IncidentListParams = {}): Promise<number> {
  const { incidents } = await incidentService.list({ ...params, limit: 1000 });
  const rows = incidents ?? [];
  const cols: [string, (i: Incident) => string | number | null | undefined][] = [
    ['id', (i) => i.id],
    ['title', (i) => i.title],
    ['type', (i) => i.incident_type],
    ['severity', (i) => i.severity],
    ['status', (i) => i.status],
    ['source', (i) => i.source],
    ['reported_by', (i) => i.reported_by],
    ['assigned_to', (i) => i.assigned_to],
    ['resolution', (i) => i.resolution],
    ['created_at', (i) => i.created_at],
    ['resolved_at', (i) => i.resolved_at],
  ];
  const esc = (v: unknown) => {
    const s = v == null ? '' : String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const header = cols.map((c) => c[0]).join(',');
  const body = rows.map((r) => cols.map((c) => esc(c[1](r))).join(',')).join('\n');
  const csv = `${header}\n${body}`;

  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `incidents-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
  return rows.length;
}
