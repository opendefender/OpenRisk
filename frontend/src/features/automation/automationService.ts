// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Typed client for the Security Automation / SOAR module (/automation/*).
// Shapes mirror backend/internal/domain/automation.go. Rules bind a trigger to
// an ordered action chain (scan → create risk → assign → ticket → notify → start
// SLA); the engine records executions and the SLA monitor escalates + auto-closes.

import { api } from '../../lib/api';

export type AutomationTrigger =
  'vulnerability_detected' | 'risk_created' | 'risk_score_updated' | 'incident_created' | 'manual';

export type AutomationActionType =
  | 'scan_asset'
  | 'create_risk'
  | 'assign_owner'
  | 'create_ticket'
  | 'notify'
  | 'start_sla'
  | 'resolve_risk'
  | 'close_ticket';

export type NotifyChannel = 'in_app' | 'email' | 'slack' | 'teams' | 'webhook' | 'sms';

export interface AutomationConditions {
  min_severity?: string;
  min_cvss?: number;
  kev_only?: boolean;
  min_priority_tier?: string;
  asset_tags?: string[];
}

export interface AutomationAction {
  type: AutomationActionType;
  channels?: NotifyChannel[];
  target?: string;
  message?: string;
  ticket_provider?: string;
}

export interface AutomationSLAConfig {
  critical_minutes?: number;
  high_minutes?: number;
  medium_minutes?: number;
  low_minutes?: number;
  escalate_after_minutes?: number;
  escalate_to_role?: string;
  escalate_channels?: NotifyChannel[];
}

export interface AutomationRule {
  id: string;
  tenant_id: string;
  name: string;
  description: string;
  enabled: boolean;
  trigger: AutomationTrigger;
  conditions: AutomationConditions;
  actions: AutomationAction[];
  sla: AutomationSLAConfig;
  priority: number;
  last_triggered_at?: string | null;
  trigger_count: number;
  created_at: string;

  suspended_at?: string | null;
  suspended_by?: string | null;
  suspended_reason?: string;
  last_status?: string;
  last_executed_at?: string | null;
  last_error?: string;
  failure_streak: number;
  template_key?: string;
}

export interface RuleInput {
  name: string;
  description?: string;
  enabled?: boolean;
  trigger: AutomationTrigger;
  conditions: AutomationConditions;
  actions: AutomationAction[];
  sla: AutomationSLAConfig;
  priority?: number;
}

export type ExecutionStatus = 'pending' | 'running' | 'success' | 'partial' | 'failed' | 'skipped';

export interface ExecutionStep {
  index: number;
  action: string;
  status: string; // success|failed|skipped
  detail: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  at: string;
  duration_ms: number;
}

export interface AutomationExecution {
  id: string;
  rule_id: string;
  rule_name: string;
  trigger: AutomationTrigger;
  trigger_ref: string;
  subject: string;
  severity: string;
  status: ExecutionStatus;
  steps: ExecutionStep[] | null;
  error?: string;
  mode: 'live' | 'manual' | 'replay';
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  actor_id?: string | null;
  actor_email?: string;
  replayed_from?: string | null;
  duration_ms: number;
  step_summary?: string;
  started_at: string;
  finished_at?: string | null;
}

// ---------------------------------------------------------------------------
// Dry run — what a rule WOULD do, without doing any of it.
// ---------------------------------------------------------------------------

export type DryRunVerdict =
  'would_run' | 'would_skip' | 'would_fail' | 'not_reached' | 'not_matched';

export interface DryRunStep {
  index: number;
  action: AutomationActionType;
  verdict: DryRunVerdict;
  detail: string;
  capability?: string;
  wired: boolean;
  params?: Record<string, unknown>;
  payload: Record<string, unknown>;
  produces?: Record<string, unknown>;
}

export interface DryRunReport {
  id: string;
  rule_id: string;
  rule_name: string;
  trigger: AutomationTrigger;
  rule_enabled: boolean;
  real_subject: boolean;
  subject_source: string;
  subject: string;
  conditions_matched: boolean;
  conditions_detail: string;
  initial_input: Record<string, unknown>;
  steps: DryRunStep[];
  failed_at_index?: number;
  failed_action?: string;
  failure_reason?: string;
  payload_at_failure?: Record<string, unknown>;
  would_run: number;
  would_skip: number;
  would_fail: number;
  side_effects: boolean;
  status: 'completed' | 'cancelled';
  started_at: string;
  finished_at: string;
  duration_ms: number;
}

export interface DryRunInput {
  subject_type?: 'vulnerability' | 'risk' | 'incident';
  subject_id?: string;
  overrides?: {
    severity?: string;
    cvss?: number;
    kev?: boolean;
    priority_tier?: string;
    asset_tags?: string[];
  };
}

export type RuleHealth = 'ok' | 'degraded' | 'failing' | 'suspended' | 'idle';

export interface RuleState {
  rule_id: string;
  name: string;
  sentence: string;
  trigger: AutomationTrigger;
  enabled: boolean;
  health: RuleHealth;
  health_detail: string;
  last_status?: string;
  last_run_at?: string | null;
  last_error?: string;
  failure_streak: number;
  trigger_count: number;
  suspended_at?: string | null;
  suspended_reason?: string;
  template_key?: string;
}

export interface AutomationState {
  rules: RuleState[];
  active: number;
  suspended: number;
  failing: number;
  degraded: number;
  idle: number;
  observed_at: string;
}

export interface AutomationTemplate {
  key: string;
  name: string;
  name_en: string;
  description: string;
  use_case: string;
  use_case_en: string;
  trigger: AutomationTrigger;
  conditions: AutomationConditions;
  actions: AutomationAction[];
  sla: AutomationSLAConfig;
  priority: number;
  requires_channels: boolean;
  requires_ticketing: boolean;
}

export interface TemplateListItem {
  template: AutomationTemplate;
  sentence: string;
}

export interface ChannelTestResult {
  channel: NotifyChannel;
  configured: boolean;
  delivered: boolean;
  detail: string;
  error?: string;
  recipients?: string;
  duration_ms: number;
  tested_at: string;
}

export interface ChannelCatalogueItem {
  channel: NotifyChannel;
  configured: boolean;
}

export type SLAStatus = 'open' | 'breached' | 'escalated' | 'met' | 'closed';

export interface SLATracker {
  id: string;
  rule_id: string;
  subject_type: string;
  subject_id: string;
  risk_id?: string | null;
  title: string;
  severity: string;
  ticket_ref?: string;
  status: SLAStatus;
  due_at: string;
  escalate_to_role?: string;
  escalation_level: number;
  escalated_at?: string | null;
  created_at: string;
  remaining_minutes: number;
  breached_now: boolean;
}

export interface SLAStats {
  open: number;
  breached: number;
  escalated: number;
  met: number;
  closed: number;
  at_risk: number;
}

export interface ChannelConfig {
  slack_enabled: boolean;
  teams_enabled: boolean;
  email_enabled: boolean;
  default_email: string;
  webhook_enabled: boolean;
  sms_enabled: boolean;
  sms_sender?: string;
  sms_recipients?: string;
  sms_to_field?: string;
  sms_text_field?: string;
  sms_sender_field?: string;
  has_slack: boolean;
  has_teams: boolean;
  has_webhook: boolean;
  has_sms: boolean;
}

export interface ChannelInput {
  slack_enabled: boolean;
  slack_webhook_url?: string;
  teams_enabled: boolean;
  teams_webhook_url?: string;
  email_enabled: boolean;
  default_email: string;
  webhook_enabled: boolean;
  webhook_url?: string;
  webhook_secret?: string;
  sms_enabled: boolean;
  sms_gateway_url?: string;
  sms_api_key?: string;
  sms_sender?: string;
  sms_recipients?: string;
  sms_to_field?: string;
  sms_text_field?: string;
  sms_sender_field?: string;
}

export interface TestInput {
  severity?: string;
  cve_id?: string;
  cvss?: number;
  kev?: boolean;
  priority_tier?: string;
  asset_name?: string;
}

export const automationService = {
  listRules: async (): Promise<AutomationRule[]> => {
    const res = await api.get<{ items: AutomationRule[] }>('/automation/rules');
    return res.data.items ?? [];
  },
  createRule: async (input: RuleInput): Promise<AutomationRule> => {
    const res = await api.post<AutomationRule>('/automation/rules', input);
    return res.data;
  },
  updateRule: async (id: string, input: RuleInput): Promise<AutomationRule> => {
    const res = await api.put<AutomationRule>(`/automation/rules/${id}`, input);
    return res.data;
  },
  deleteRule: async (id: string): Promise<void> => {
    await api.delete(`/automation/rules/${id}`);
  },
  // Dry run: traces the rule against real tenant data and touches nothing.
  dryRun: async (id: string, input: DryRunInput, signal?: AbortSignal): Promise<DryRunReport> => {
    const res = await api.post<DryRunReport>(`/automation/rules/${id}/dry-run`, input, { signal });
    return res.data;
  },
  cancelDryRun: async (id: string): Promise<void> => {
    await api.post(`/automation/dry-runs/${id}/cancel`);
  },
  // Running for real. Deliberately a different verb from dryRun, and it refuses
  // without confirm — this one opens risks, files tickets and pages people.
  runRule: async (id: string, input: TestInput): Promise<AutomationExecution> => {
    const res = await api.post<AutomationExecution>(`/automation/rules/${id}/run`, {
      ...input,
      confirm: true,
    });
    return res.data;
  },
  enableRule: async (id: string): Promise<AutomationRule> => {
    const res = await api.post<AutomationRule>(`/automation/rules/${id}/enable`, {});
    return res.data;
  },
  suspendRule: async (id: string, reason: string): Promise<AutomationRule> => {
    const res = await api.post<AutomationRule>(`/automation/rules/${id}/suspend`, { reason });
    return res.data;
  },
  replayExecution: async (id: string): Promise<AutomationExecution> => {
    const res = await api.post<AutomationExecution>(`/automation/executions/${id}/replay`, {});
    return res.data;
  },
  getState: async (): Promise<AutomationState> => {
    const res = await api.get<AutomationState>('/automation/state', { params: { locale: 'fr' } });
    return res.data;
  },
  listTemplates: async (): Promise<TemplateListItem[]> => {
    const res = await api.get<{ items: TemplateListItem[] }>('/automation/templates', {
      params: { locale: 'fr' },
    });
    return res.data.items ?? [];
  },
  adoptTemplate: async (key: string, name?: string): Promise<AutomationRule> => {
    const res = await api.post<AutomationRule>(`/automation/templates/${key}/adopt`, { name });
    return res.data;
  },
  testChannel: async (channel: NotifyChannel): Promise<ChannelTestResult> => {
    const res = await api.post<ChannelTestResult>('/automation/channels/test', { channel });
    return res.data;
  },
  channelCatalogue: async (): Promise<ChannelCatalogueItem[]> => {
    const res = await api.get<{ items: ChannelCatalogueItem[] }>('/automation/channels/catalogue');
    return res.data.items ?? [];
  },
  listExecutions: async (): Promise<AutomationExecution[]> => {
    const res = await api.get<{ items: AutomationExecution[] }>('/automation/executions', {
      params: { limit: 100 },
    });
    return res.data.items ?? [];
  },
  listSLA: async (): Promise<SLATracker[]> => {
    const res = await api.get<{ items: SLATracker[] }>('/automation/sla');
    return res.data.items ?? [];
  },
  slaStats: async (): Promise<SLAStats> => {
    const res = await api.get<SLAStats>('/automation/sla/stats');
    return res.data;
  },
  getChannels: async (): Promise<ChannelConfig> => {
    const res = await api.get<ChannelConfig>('/automation/channels');
    return res.data;
  },
  saveChannels: async (input: ChannelInput): Promise<ChannelConfig> => {
    const res = await api.put<ChannelConfig>('/automation/channels', input);
    return res.data;
  },
};
