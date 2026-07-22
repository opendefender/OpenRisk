// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for vulnerability-integration configuration:
//   - scanner connectors (encrypted API credentials, live-pull, inbound webhook,
//     auto-risk / auto-ticket toggles) — /vulnerabilities/integrations
//   - tenant ITSM/ticketing config (Jira/ServiceNow) — /vulnerabilities/ticketing
// Credentials are write-only: the API never returns them (only has_credentials).

import { api } from '../../lib/api';
import type { VulnSource } from './vulnerabilityService';

export interface VulnIntegration {
  id: string;
  tenant_id: string;
  source: VulnSource;
  name: string;
  enabled: boolean;
  base_url: string;
  live_pull_enabled: boolean;
  schedule_minutes: number;
  last_pull_at?: string | null;
  last_pull_status: string; // never | ok | error
  last_pull_error?: string;
  last_pull_count: number;
  webhook_enabled: boolean;
  webhook_token?: string;
  auto_create_risk: boolean;
  auto_create_ticket: boolean;
  has_credentials: boolean;
}

export interface SaveIntegrationInput {
  source: VulnSource;
  name?: string;
  enabled?: boolean;
  base_url?: string;
  credentials?: Record<string, string>;
  clear_credentials?: boolean;
  live_pull_enabled?: boolean;
  schedule_minutes?: number;
  webhook_enabled?: boolean;
  regenerate_webhook_token?: boolean;
  auto_create_risk?: boolean;
  auto_create_ticket?: boolean;
}

export interface LivePullResult {
  integration_id: string;
  source: VulnSource;
  supported: boolean;
  received: number;
  created: number;
  updated: number;
  skipped: number;
}

export type VulnTicketProvider = '' | 'jira' | 'servicenow';

export interface VulnTicketingConfig {
  id?: string;
  tenant_id?: string;
  provider: VulnTicketProvider;
  enabled: boolean;
  base_url: string;
  project_or_table: string;
  default_issue_type: string;
  has_credentials: boolean;
}

export interface SaveTicketingInput {
  provider: VulnTicketProvider;
  enabled: boolean;
  base_url?: string;
  project_or_table?: string;
  default_issue_type?: string;
  credentials?: Record<string, string>;
  clear_credentials?: boolean;
}

export const vulnIntegrationsService = {
  list: async (): Promise<VulnIntegration[]> => {
    const res = await api.get<{ items: VulnIntegration[] }>('/vulnerabilities/integrations');
    return res.data.items ?? [];
  },
  save: async (input: SaveIntegrationInput): Promise<VulnIntegration> => {
    const res = await api.post<VulnIntegration>('/vulnerabilities/integrations', input);
    return res.data;
  },
  remove: async (id: string): Promise<void> => {
    await api.delete(`/vulnerabilities/integrations/${id}`);
  },
  pull: async (id: string): Promise<LivePullResult> => {
    const res = await api.post<LivePullResult>(`/vulnerabilities/integrations/${id}/pull`, {});
    return res.data;
  },
  getTicketing: async (): Promise<VulnTicketingConfig> => {
    const res = await api.get<VulnTicketingConfig>('/vulnerabilities/ticketing');
    return res.data;
  },
  saveTicketing: async (input: SaveTicketingInput): Promise<VulnTicketingConfig> => {
    const res = await api.put<VulnTicketingConfig>('/vulnerabilities/ticketing', input);
    return res.data;
  },
  deleteTicketing: async (): Promise<void> => {
    await api.delete('/vulnerabilities/ticketing');
  },
};
