// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the CTI / Intel Threat engine (/cti/*). Shapes mirror
// backend/pkg/cti/model.go (CTIVulnerability) and the cti_handler responses.

import { api } from '../../lib/api';

export interface CTIVulnerability {
  cve_id: string;
  cvss_v3: number;
  severity: string; // CRITICAL | HIGH | MEDIUM | LOW
  description: string;
  published_at: string;
  cisa_known: boolean;
  cisa_due_date?: string | null;
  mitre_tactics: string[] | null;
  mitre_techniques: string[] | null;
  affected_cpe: string[] | null;
  remediation: string;
  last_updated_at: string;
}

export interface CTIStats {
  total: number;
  new_24h: number;
  critical: number;
  cisa_known: number;
  cti_risks: number;
}

export interface CTIListParams {
  query?: string;
  severity?: string;
  cisa_known?: boolean;
  limit?: number;
  offset?: number;
}

export const ctiService = {
  async list(params: CTIListParams = {}): Promise<CTIVulnerability[]> {
    const { data } = await api.get('/cti/vulnerabilities', { params });
    return data.vulnerabilities ?? [];
  },

  async stats(): Promise<CTIStats> {
    const { data } = await api.get('/cti/stats');
    return data;
  },

  async get(cve: string): Promise<CTIVulnerability> {
    const { data } = await api.get(`/cti/vulnerabilities/${encodeURIComponent(cve)}`);
    return data;
  },

  async sync(): Promise<{ message: string; total_vulnerabilities: number }> {
    const { data } = await api.post('/cti/sync');
    return data;
  },

  async match(): Promise<{ message: string; risks_created: number }> {
    const { data } = await api.post('/cti/match');
    return data;
  },
};
