// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the Governance module (/governance/*, spec §15). Shapes mirror
// backend/internal/domain/governance.go: the immutable audit trail, time-boxed
// delegations, and the Maker-Checker approval engine (workflows + requests).

import { api } from '../../lib/api';

// ---------------------------------------------------------------------------
// Audit trail
// ---------------------------------------------------------------------------

export type AuditAction =
  | 'create' | 'update' | 'delete' | 'submit' | 'approve' | 'reject'
  | 'delegate' | 'revoke' | 'login' | 'export';

export interface AuditEvent {
  id: string;
  tenant_id: string;
  actor_id?: string | null;
  actor_email?: string;
  action: AuditAction;
  entity_type: string;
  entity_id: string;
  summary: string;
  before?: Record<string, unknown> | null;
  after?: Record<string, unknown> | null;
  changed_fields?: string[] | null;
  ip_address?: string;
  user_agent?: string;
  request_id?: string;
  created_at: string;
}

export interface AuditEventsResult {
  events: AuditEvent[];
  total: number;
  limit: number;
  offset: number;
}

export interface AuditFilter {
  entity_type?: string;
  entity_id?: string;
  action?: string;
  actor_id?: string;
  from?: string;
  to?: string;
  search?: string;
  limit?: number;
  offset?: number;
}

// ---------------------------------------------------------------------------
// Delegations
// ---------------------------------------------------------------------------

export type DelegationStatus = 'active' | 'revoked' | 'expired';

export interface Delegation {
  id: string;
  tenant_id: string;
  delegator_id: string;
  delegator_email?: string;
  delegate_id: string;
  delegate_email?: string;
  reason?: string;
  permissions: string[];
  status: DelegationStatus;
  starts_at: string;
  ends_at: string;
  revoked_at?: string | null;
  created_by: string;
  created_at: string;
}

export interface CreateDelegationInput {
  delegate_id: string;
  reason?: string;
  permissions: string[];
  starts_at?: string;
  ends_at: string;
}

export interface EffectivePermissions {
  delegate_id: string;
  permissions: string[];
}

// ---------------------------------------------------------------------------
// Approval workflows + requests (Maker-Checker)
// ---------------------------------------------------------------------------

export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled';

export interface WorkflowStep {
  order: number;
  name: string;
  approver_role: string;
  min_approvals: number;
}

export interface ApprovalWorkflow {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  entity_type: string;
  action: string;
  enabled: boolean;
  steps: WorkflowStep[];
  created_by: string;
  created_at: string;
}

export interface WorkflowInput {
  name: string;
  description?: string;
  entity_type: string;
  action?: string;
  enabled?: boolean;
  steps: Array<{ name: string; approver_role: string; min_approvals: number }>;
}

export interface ApprovalDecision {
  step_order: number;
  approver_id: string;
  approver_email?: string;
  decision: 'approve' | 'reject';
  comment?: string;
  decided_at: string;
}

export interface ApprovalRequest {
  id: string;
  tenant_id: string;
  workflow_id?: string | null;
  workflow_name?: string;
  entity_type: string;
  entity_id?: string;
  action?: string;
  title: string;
  description?: string;
  payload?: Record<string, unknown> | null;
  status: ApprovalStatus;
  current_step: number;
  steps: WorkflowStep[];
  decisions: ApprovalDecision[];
  requested_by: string;
  requested_by_email?: string;
  resolved_at?: string | null;
  created_at: string;
}

export interface SubmitApprovalInput {
  entity_type: string;
  entity_id?: string;
  action?: string;
  title: string;
  description?: string;
  payload?: Record<string, unknown>;
}

export interface DecideApprovalInput {
  decision: 'approve' | 'reject';
  comment?: string;
}

function qs(filter: AuditFilter): string {
  const p = new URLSearchParams();
  Object.entries(filter).forEach(([k, v]) => {
    if (v !== undefined && v !== '' && v !== null) p.append(k, String(v));
  });
  const s = p.toString();
  return s ? `?${s}` : '';
}

export const governanceService = {
  // Audit trail
  listAuditEvents: (filter: AuditFilter = {}): Promise<AuditEventsResult> =>
    api.get<AuditEventsResult>(`/governance/audit-events${qs(filter)}`).then((r) => r.data),
  auditExportUrl: (filter: AuditFilter = {}): string =>
    `/governance/audit-events/export${qs(filter)}`,
  exportAuditCsv: (filter: AuditFilter = {}): Promise<Blob> =>
    api.get(`/governance/audit-events/export${qs(filter)}`, { responseType: 'blob' }).then((r) => r.data as Blob),

  // Delegations
  listDelegations: (): Promise<Delegation[]> =>
    api.get<Delegation[]>('/governance/delegations').then((r) => r.data),
  createDelegation: (input: CreateDelegationInput): Promise<Delegation> =>
    api.post<Delegation>('/governance/delegations', input).then((r) => r.data),
  revokeDelegation: (id: string): Promise<Delegation> =>
    api.post<Delegation>(`/governance/delegations/${id}/revoke`, {}).then((r) => r.data),
  effectivePermissions: (delegateId?: string): Promise<EffectivePermissions> =>
    api.get<EffectivePermissions>(`/governance/delegations/effective${delegateId ? `?delegate_id=${delegateId}` : ''}`).then((r) => r.data),

  // Approval workflows (config)
  listWorkflows: (): Promise<ApprovalWorkflow[]> =>
    api.get<ApprovalWorkflow[]>('/governance/workflows').then((r) => r.data),
  createWorkflow: (input: WorkflowInput): Promise<ApprovalWorkflow> =>
    api.post<ApprovalWorkflow>('/governance/workflows', input).then((r) => r.data),
  updateWorkflow: (id: string, input: WorkflowInput): Promise<ApprovalWorkflow> =>
    api.put<ApprovalWorkflow>(`/governance/workflows/${id}`, input).then((r) => r.data),
  deleteWorkflow: (id: string): Promise<void> =>
    api.delete(`/governance/workflows/${id}`).then(() => undefined),

  // Approval requests (inbox)
  listApprovals: (params: { status?: string; entity_type?: string; mine?: boolean } = {}): Promise<ApprovalRequest[]> => {
    const p = new URLSearchParams();
    if (params.status) p.append('status', params.status);
    if (params.entity_type) p.append('entity_type', params.entity_type);
    if (params.mine) p.append('mine', 'true');
    const s = p.toString();
    return api.get<ApprovalRequest[]>(`/governance/approvals${s ? `?${s}` : ''}`).then((r) => r.data);
  },
  submitApproval: (input: SubmitApprovalInput): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>('/governance/approvals', input).then((r) => r.data),
  decideApproval: (id: string, input: DecideApprovalInput): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>(`/governance/approvals/${id}/decide`, input).then((r) => r.data),
  cancelApproval: (id: string): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>(`/governance/approvals/${id}/cancel`, {}).then((r) => r.data),
};
