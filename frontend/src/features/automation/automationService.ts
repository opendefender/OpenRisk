// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Typed client for the Security Automation / SOAR module (/automation/*).
// Shapes mirror backend/internal/domain/automation.go. Rules bind a trigger to
// an ordered action chain (scan → create risk → assign → ticket → notify → start
// SLA); the engine records executions and the SLA monitor escalates + auto-closes.

import { api } from '../../lib/api';

export type AutomationTrigger =
  | 'vulnerability_detected'
  | 'risk_created'
  | 'risk_score_updated'
  | 'incident_created'
  | 'manual';

export type AutomationActionType =
  | 'scan_asset'
  | 'create_risk'
  | 'assign_owner'
  | 'create_ticket'
  | 'notify'
  | 'start_sla'
  | 'resolve_risk'
  | 'close_ticket';

export type NotifyChannel = 'in_app' | 'email' | 'slack' | 'teams';

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
  action: string;
  status: string; // success|failed|skipped
  detail: string;
  at: string;
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
  started_at: string;
  finished_at?: string | null;
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
  has_slack: boolean;
  has_teams: boolean;
}

export interface ChannelInput {
  slack_enabled: boolean;
  slack_webhook_url?: string;
  teams_enabled: boolean;
  teams_webhook_url?: string;
  email_enabled: boolean;
  default_email: string;
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
  testRule: async (id: string, input: TestInput): Promise<AutomationExecution> => {
    const res = await api.post<AutomationExecution>(`/automation/rules/${id}/test`, input);
    return res.data;
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
