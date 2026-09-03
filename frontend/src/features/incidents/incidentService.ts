// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
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
  risk_ids?: string[] | null;
  asset_ids?: string[] | null;
  stakeholders?: IncidentStakeholder[] | null;
  origin: IncidentOrigin;
  origin_rule_id?: string | null;
  origin_rule_name?: string;
  origin_execution_id?: string | null;
  origin_detail?: string;
  resolution?: string;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
}

export type IncidentOrigin = 'manual' | 'automation' | 'scanner' | 'cti' | 'integration';

export interface IncidentStakeholder {
  user_id?: string;
  email?: string;
  role?: string;
  name?: string;
  channels?: string[];
}

/**
 * One response action on an incident — the War Room's task board.
 *
 * Mirrors domain.IncidentAction (uint ids, like the rest of the incident
 * module). `incident` is present in the wire payload as a zero-value struct
 * because Go's omitempty does not elide structs; it is deliberately not typed
 * here, so nothing can start reading a field that is never populated.
 */
export type IncidentActionStatus = 'pending' | 'in_progress' | 'completed';

export interface IncidentAction {
  id: number;
  incident_id: number;
  title: string;
  description: string;
  assigned_to: string;
  due_date: string;
  status: IncidentActionStatus;
  priority: string;
  created_at: string;
  updated_at: string;
}

export interface CreateIncidentActionInput {
  title: string;
  description?: string;
  assigned_to?: string;
  /** RFC3339. The backend parses a zero time fine when it is omitted. */
  due_date?: string;
}

export interface IncidentOriginInfo {
  key: IncidentOrigin;
  label: string;
  label_en: string;
  description: string;
  automatic: boolean;
  where_to_configure?: string;
}

export interface IncidentOriginCount {
  origin: IncidentOriginInfo;
  count: number;
}

// ---------------------------------------------------------------------------
// Post-mortem
// ---------------------------------------------------------------------------

export interface PostMortemTimelineEntry {
  at: string;
  title: string;
  detail?: string;
  kind?: 'detection' | 'escalation' | 'mitigation' | 'resolution' | 'note';
}

export interface CorrectiveAction {
  id: string;
  title: string;
  description?: string;
  owner_id?: string;
  due_date?: string | null;
  priority?: string;
  status: string;
  mitigation_id?: string;
  risk_id?: string;
}

export interface PostMortem {
  id: string;
  incident_id: number;
  summary: string;
  root_cause: string;
  contributing_factors?: string;
  impact: string;
  detection?: string;
  what_went_well?: string;
  lessons_learned?: string;
  timeline: PostMortemTimelineEntry[] | null;
  corrective_actions: CorrectiveAction[] | null;
  status: 'draft' | 'published';
  author_email?: string;
  published_at?: string | null;
  created_at: string;
}

export interface PostMortemView {
  post_mortem: PostMortem;
  missing: string[] | null;
  required: boolean;
  blocks_closure?: string;
}

export interface PostMortemInput {
  summary: string;
  root_cause: string;
  contributing_factors?: string;
  impact: string;
  detection?: string;
  what_went_well?: string;
  lessons_learned?: string;
  timeline: PostMortemTimelineEntry[];
  corrective_actions: CorrectiveAction[];
}

export interface PublishResult {
  view: PostMortemView;
  mitigations_created: number;
  not_converted?: string[];
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
  risk_ids?: string[];
  asset_ids?: string[];
  stakeholders?: IncidentStakeholder[];
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

  // Where incidents come from — the catalogue plus this tenant's real counts.
  origins: async (): Promise<{ items: IncidentOriginCount[]; total: number }> => {
    const response = await api.get<{ items: IncidentOriginCount[]; total: number }>(
      '/incidents/origins',
    );
    return response.data;
  },

  getPostMortem: async (id: number): Promise<PostMortemView> => {
    const response = await api.get<PostMortemView>(`/incidents/${id}/post-mortem`);
    return response.data;
  },
  savePostMortem: async (id: number, input: PostMortemInput): Promise<PostMortemView> => {
    const response = await api.put<PostMortemView>(`/incidents/${id}/post-mortem`, input);
    return response.data;
  },
  publishPostMortem: async (id: number): Promise<PublishResult> => {
    const response = await api.post<PublishResult>(`/incidents/${id}/post-mortem/publish`, {});
    return response.data;
  },

  // --- response actions (War Room task board) ----------------------------
  //
  // These endpoints have existed since the incident module shipped and are
  // tenant-scoped (the service checks ownsIncident before every read and
  // write). The War Room nevertheless declared its task board unavailable and
  // offered an ephemeral chat instead — honest about the board, but needlessly:
  // there was a real, guarded API the whole time (W0-05 / D7).

  actions: async (id: number): Promise<IncidentAction[]> => {
    const response = await api.get<IncidentAction[] | null>(`/incidents/${id}/actions`);
    return response.data ?? [];
  },

  createAction: async (id: number, input: CreateIncidentActionInput): Promise<IncidentAction> => {
    const response = await api.post<IncidentAction>(`/incidents/${id}/actions`, input);
    return response.data;
  },

  setActionStatus: async (
    id: number,
    actionId: number,
    status: IncidentActionStatus,
  ): Promise<void> => {
    await api.put(`/incidents/${id}/actions/${actionId}`, { status });
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
