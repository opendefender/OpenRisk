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
  | 'create'
  | 'update'
  | 'delete'
  | 'submit'
  | 'approve'
  | 'reject'
  | 'delegate'
  | 'revoke'
  | 'login'
  | 'export';

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
  method?: string;
  path?: string;
  status_code?: number;
  source?: 'http' | 'gorm' | 'explicit' | 'legacy';
  sequence: number;
  prev_hash: string;
  hash: string;
  created_at: string;
}

// ---------------------------------------------------------------------------
// Tamper evidence
// ---------------------------------------------------------------------------

export interface AuditChainBreak {
  sequence: number;
  event_id?: string;
  kind: 'hash_mismatch' | 'prev_mismatch' | 'sequence_gap' | 'unsealed_head' | 'duplicate_sequence';
  detail: string;
}

export interface AuditChainReport {
  valid: boolean;
  total_events: number;
  verified: number;
  first_sequence: number;
  last_sequence: number;
  head_hash: string;
  seals: number;
  breaks: AuditChainBreak[] | null;
  checked_at: string;
  duration_ms: number;
}

export interface AuditRetentionPolicy {
  retention_days: number;
  last_pruned_at?: string | null;
}

export interface PruneResult {
  pruned: number;
  seal?: { from_sequence: number; to_sequence: number; pruned_count: number } | null;
}

export interface AuditEventsResult {
  events: AuditEvent[];
  total: number;
  limit: number;
  offset: number;
}

export interface AuditFilter {
  format?: 'csv' | 'json';
  entity_type?: string;
  entity_id?: string;
  action?: string;
  actor_id?: string;
  from?: string;
  to?: string;
  search?: string;
  request_id?: string;
  source?: string;
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

export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled' | 'expired';

export interface WorkflowStep {
  order: number;
  name: string;
  approver_role: string;
  min_approvals: number;
  approver_user_ids?: string[];
  quorum_percent?: number;
}

export type ApprovalMode = 'sequential' | 'parallel';

export interface ApprovalRequestType {
  key: string;
  entity_type: string;
  action: string;
  label: string;
  label_en: string;
  description: string;
  linked_to_lifecycle?: string;
}

export interface StepProgress {
  order: number;
  name: string;
  approver_role?: string;
  required_approvals: number;
  approvals: number;
  satisfied: boolean;
  rejected: boolean;
  open: boolean;
  approvers?: string[];
}

export interface EligibilityVerdict {
  eligible: boolean;
  reason: string;
  via_delegation?: string;
}

export interface ApprovalDetail {
  request: ApprovalRequest;
  progress: StepProgress[];
  can_decide: boolean;
  verdict: EligibilityVerdict;
  open_steps?: WorkflowStep[] | null;
  expired: boolean;
  request_type_info?: ApprovalRequestType | null;
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
  request_type?: string;
  mode: ApprovalMode;
  expires_in_hours: number;
  created_by: string;
  created_at: string;
}

export interface WorkflowInput {
  name: string;
  description?: string;
  entity_type: string;
  action?: string;
  enabled?: boolean;
  request_type?: string;
  mode?: ApprovalMode;
  expires_in_hours?: number;
  steps: Array<{
    name: string;
    approver_role: string;
    min_approvals: number;
    approver_user_ids?: string[];
    quorum_percent?: number;
  }>;
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
  mode: ApprovalMode;
  expires_at?: string | null;
  request_type?: string;
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
  /** Targets one branch of a parallel chain. Omitted in sequential mode. */
  step_order?: number;
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
    api
      .get(`/governance/audit-events/export${qs(filter)}`, { responseType: 'blob' })
      .then((r) => r.data as Blob),
  // A signed JSON export carries the entries, the chain verdict at export time
  // and an HMAC over both — so a recipient can re-check it offline.
  exportAuditJson: (filter: AuditFilter = {}): Promise<Blob> =>
    api
      .get(`/governance/audit-events/export${qs({ ...filter, format: 'json' } as AuditFilter)}`, {
        responseType: 'blob',
      })
      .then((r) => r.data as Blob),
  verifyAuditChain: (): Promise<AuditChainReport> =>
    api.get<AuditChainReport>('/governance/audit-events/verify').then((r) => r.data),
  getRetention: (): Promise<AuditRetentionPolicy> =>
    api.get<AuditRetentionPolicy>('/governance/audit-retention').then((r) => r.data),
  setRetention: (days: number): Promise<AuditRetentionPolicy> =>
    api
      .put<AuditRetentionPolicy>('/governance/audit-retention', { retention_days: days })
      .then((r) => r.data),
  applyRetention: (): Promise<PruneResult> =>
    api.post<PruneResult>('/governance/audit-retention/apply', {}).then((r) => r.data),

  // Delegations
  listDelegations: (): Promise<Delegation[]> =>
    api.get<Delegation[]>('/governance/delegations').then((r) => r.data),
  createDelegation: (input: CreateDelegationInput): Promise<Delegation> =>
    api.post<Delegation>('/governance/delegations', input).then((r) => r.data),
  revokeDelegation: (id: string): Promise<Delegation> =>
    api.post<Delegation>(`/governance/delegations/${id}/revoke`, {}).then((r) => r.data),
  effectivePermissions: (delegateId?: string): Promise<EffectivePermissions> =>
    api
      .get<EffectivePermissions>(
        `/governance/delegations/effective${delegateId ? `?delegate_id=${delegateId}` : ''}`,
      )
      .then((r) => r.data),

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
  listApprovals: (
    params: { status?: string; entity_type?: string; mine?: boolean } = {},
  ): Promise<ApprovalRequest[]> => {
    const p = new URLSearchParams();
    if (params.status) p.append('status', params.status);
    if (params.entity_type) p.append('entity_type', params.entity_type);
    if (params.mine) p.append('mine', 'true');
    const s = p.toString();
    return api
      .get<ApprovalRequest[]>(`/governance/approvals${s ? `?${s}` : ''}`)
      .then((r) => r.data);
  },
  // The detail view carries per-step progress and whether the CALLER may sign —
  // so a disabled button can explain itself rather than produce a 403 on click.
  getApproval: (id: string): Promise<ApprovalDetail> =>
    api.get<ApprovalDetail>(`/governance/approvals/${id}`).then((r) => r.data),
  listRequestTypes: (): Promise<ApprovalRequestType[]> =>
    api
      .get<{ items: ApprovalRequestType[] }>('/governance/request-types')
      .then((r) => r.data.items ?? []),
  submitApproval: (input: SubmitApprovalInput): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>('/governance/approvals', input).then((r) => r.data),
  decideApproval: (id: string, input: DecideApprovalInput): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>(`/governance/approvals/${id}/decide`, input).then((r) => r.data),
  cancelApproval: (id: string): Promise<ApprovalRequest> =>
    api.post<ApprovalRequest>(`/governance/approvals/${id}/cancel`, {}).then((r) => r.data),
};
