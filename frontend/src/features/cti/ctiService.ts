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

/** One part of a threat's relevance (backend domain.ThreatFactor). */
export interface ThreatFactor {
  key: 'asset' | 'exploitation' | 'severity' | 'evidence';
  points: number;
  max: number;
  /** A criticality, kev / epss_high / epss_medium, or scan / cpe. */
  value: string;
}

export interface ThreatAsset {
  /** Null for a scanned host that matches no asset of the inventory. */
  id: string | null;
  name: string;
  criticality: string;
  via: ('scan' | 'cpe')[];
}

/** A catalogue entry that touches the tenant's assets (#911). */
export interface Threat {
  cve_id: string;
  /** The scanner's title when one found it, else empty. */
  title: string;
  description: string;
  severity: string;
  cvss: number;
  epss: number;
  kev: boolean;
  kev_due_date?: string | null;
  published_at: string | null;
  techniques: string[];
  tactics: string[];
  assets: ThreatAsset[];
  relevance: number;
  factors: ThreatFactor[];
  /** nvd, cisa_kev, mitre_attack, and the scanners that reported it. */
  sources: string[];
  dismissed_at: string | null;
}

export interface ThreatList {
  threats: Threat[];
  assets_with_cpes: number;
  truncated: boolean;
}

export const ctiService = {
  async threats(): Promise<ThreatList> {
    const { data } = await api.get<ThreatList>('/cti/threats');
    return data;
  },

  async dismiss(cve: string): Promise<void> {
    await api.put(`/cti/threats/${encodeURIComponent(cve)}/dismissal`);
  },

  async restore(cve: string): Promise<void> {
    await api.delete(`/cti/threats/${encodeURIComponent(cve)}/dismissal`);
  },

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
