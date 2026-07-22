// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Typed client for the scan engine (/scanner/*). Hand-written (like the
// incidents/mitigations services). Shapes mirror backend/internal/domain/
// scanner.go + internal/scanner/*.go. Credentials are NEVER returned by the API.

import { api } from '../../lib/api';

export type ScannerProvider =
  | 'aws' | 'azure' | 'gcp' | 'nmap' | 'agent'
  // Auto-discovery API providers (spec "6. Découverte automatique des actifs").
  | 'kubernetes' | 'docker' | 'vmware' | 'active_directory' | 'm365' | 'github' | 'gitlab';
export type AgentStatus = 'online' | 'offline' | 'scanning' | 'error' | 'revoked';
export type ScanJobStatus = 'queued' | 'claimed' | 'running' | 'completed' | 'failed' | 'timeout';

export interface ScanConfig {
  id: string;
  tenant_id: string;
  name: string;
  provider: ScannerProvider;
  enabled: boolean;
  regions: string[] | null;
  targets: string[] | null;
  agent_ids: string[] | null;
  schedule_minutes: number; // 0 = manual only
  next_run_at?: string | null;
  last_run_at?: string | null;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ScannerAgent {
  id: string;
  tenant_id: string;
  name: string;
  version: string;
  status: AgentStatus;
  last_heartbeat: string;
  ip: string;
  hostname: string;
  os: string;
  registered_at: string;
  registration_config_id?: string | null;
  last_scan_job_id?: string | null;
  token_rotated_at: string;
  created_at: string;
  updated_at: string;
}

export interface ScanJob {
  id: string;
  tenant_id: string;
  config_id: string;
  provider: ScannerProvider;
  status: ScanJobStatus;
  targets: string[] | null;
  claimed_by_agent?: string | null;
  preview_key?: string;
  assets_found: number;
  findings_found: number;
  error?: string;
  triggered_by: string;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AssetDiscovery {
  external_id: string;
  name: string;
  type: string;
  ip?: string | null;
  hostname?: string | null;
  os?: string | null;
  os_version?: string | null;
  cpe: string[] | null;
  criticality: number;
  environment: string;
  tags: string[] | null;
  location?: string | null;
  raw_metadata?: Record<string, unknown>;
  scan_job_id: string;
  agent_id?: string | null;
}

export interface FindingDiscovery {
  cve?: string | null;
  title: string;
  description: string;
  severity: string;
  affected_cpe: string[] | null;
  evidence: string;
  remediation_hint: string;
  source: string;
  raw_finding?: Record<string, unknown>;
  asset_external_id?: string;
  scan_job_id: string;
  agent_id?: string | null;
}

export interface AutoMitigation {
  asset_external_id: string;
  cve?: string | null;
  title: string;
  severity: string;
  evidence: string;
  detected_at: string;
}

export interface ScanPreview {
  job_id: string;
  config_id: string;
  tenant_id: string;
  provider: ScannerProvider;
  agent_id?: string | null;
  agent_name?: string;
  triggered_by?: string;
  assets: AssetDiscovery[] | null;
  findings: FindingDiscovery[] | null;
  mitigations: AutoMitigation[] | null;
  errors?: string[];
  created_at: string;
  expires_at: string;
}

export interface CreateScanConfigInput {
  name: string;
  provider: ScannerProvider;
  credentials?: Record<string, string>;
  regions?: string[];
  targets?: string[];
  agent_ids?: string[];
  options?: Record<string, unknown>;
  schedule_minutes?: number;
}

export interface RegistrationTokenResponse {
  registration_token: string;
  expires_at: string;
  config_id: string;
  downloads: { windows: string; linux: string; macos: string; docker: string };
}

export type AssetCriticality = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

export interface ImportSelection {
  external_id: string;
  criticality?: AssetCriticality | '';
}

export const scannerService = {
  listConfigs: async (): Promise<ScanConfig[]> => (await api.get<ScanConfig[]>('/scanner/configs')).data ?? [],

  createConfig: async (input: CreateScanConfigInput): Promise<ScanConfig> =>
    (await api.post<ScanConfig>('/scanner/configs', input)).data,

  deleteConfig: async (id: string): Promise<void> => {
    await api.delete(`/scanner/configs/${id}`);
  },

  triggerScan: async (id: string): Promise<ScanJob> =>
    (await api.post<ScanJob>(`/scanner/configs/${id}/scan`)).data,

  registrationToken: async (id: string): Promise<RegistrationTokenResponse> =>
    (await api.post<RegistrationTokenResponse>(`/scanner/configs/${id}/registration-token`)).data,

  listAgents: async (): Promise<ScannerAgent[]> => (await api.get<ScannerAgent[]>('/scanner/agents')).data ?? [],

  revokeAgent: async (id: string): Promise<void> => {
    await api.delete(`/scanner/agents/${id}`);
  },

  listJobs: async (): Promise<ScanJob[]> => (await api.get<ScanJob[]>('/scanner/jobs')).data ?? [],

  getPreview: async (jobId: string): Promise<ScanPreview> =>
    (await api.get<ScanPreview>(`/scanner/jobs/${jobId}/preview`)).data,

  importPreview: async (jobId: string, selections: ImportSelection[]): Promise<{ assets_imported: number }> =>
    (await api.post<{ assets_imported: number }>(`/scanner/jobs/${jobId}/import`, { selections })).data,

  ignorePreview: async (jobId: string): Promise<void> => {
    await api.post(`/scanner/jobs/${jobId}/ignore`);
  },
};
