// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Static presentation metadata for the scan engine: provider identities,
// per-provider credential fields, status colors and small formatting helpers.

import { Cloud, Server, Network, type LucideIcon } from 'lucide-react';
import type { AgentStatus, AssetCriticality, ScanJobStatus, ScannerProvider } from './scannerService';

export interface ProviderMeta {
  short: string;
  color: string;
  icon: LucideIcon;
  cloud: boolean;
}

export const PROVIDERS: Record<ScannerProvider, ProviderMeta> = {
  aws: { short: 'AWS', color: '#ff9f0a', icon: Cloud, cloud: true },
  azure: { short: 'Azure', color: '#0a84ff', icon: Cloud, cloud: true },
  gcp: { short: 'GCP', color: '#30d158', icon: Cloud, cloud: true },
  nmap: { short: 'On-Premise', color: '#7c6cff', icon: Network, cloud: false },
  agent: { short: 'Agent', color: '#64d2ff', icon: Server, cloud: false },
};

export interface CredField {
  key: string;
  label: string;
  placeholder: string;
  kind: 'text' | 'password' | 'textarea';
}

// Fields required per cloud provider — mirrors the `required` slices in
// internal/scanner/cloud.go (NewAWSScanner / NewAzureScanner / NewGCPScanner).
export const CLOUD_CRED_FIELDS: Record<'aws' | 'azure' | 'gcp', CredField[]> = {
  aws: [
    { key: 'access_key_id', label: 'Access Key ID', placeholder: 'AKIA…', kind: 'text' },
    { key: 'secret_access_key', label: 'Secret Access Key', placeholder: '••••••••', kind: 'password' },
    { key: 'session_token', label: 'Session Token (optional)', placeholder: '', kind: 'password' },
  ],
  azure: [
    { key: 'tenant_id', label: 'Tenant ID', placeholder: '00000000-0000-…', kind: 'text' },
    { key: 'client_id', label: 'Client ID', placeholder: '00000000-0000-…', kind: 'text' },
    { key: 'client_secret', label: 'Client Secret', placeholder: '••••••••', kind: 'password' },
    { key: 'subscription_id', label: 'Subscription ID', placeholder: '00000000-0000-…', kind: 'text' },
  ],
  gcp: [
    { key: 'service_account_json', label: 'Service Account JSON', placeholder: '{ "type": "service_account", … }', kind: 'textarea' },
    { key: 'project_id', label: 'Project ID (optional)', placeholder: 'my-project-123', kind: 'text' },
  ],
};

export function jobStatusColor(s: ScanJobStatus): string {
  switch (s) {
    case 'completed': return 'var(--low)';
    case 'running':
    case 'claimed': return 'var(--info)';
    case 'queued': return 'var(--medium)';
    case 'failed':
    case 'timeout': return 'var(--critical)';
    default: return 'var(--text-muted)';
  }
}

export function agentStatusColor(s: AgentStatus): string {
  switch (s) {
    case 'online': return 'var(--low)';
    case 'scanning': return 'var(--info)';
    case 'error': return 'var(--critical)';
    case 'revoked': return 'var(--text-muted)';
    default: return 'var(--text-muted)'; // offline
  }
}

export function severityColor(sev: string): string {
  switch (sev.toLowerCase()) {
    case 'critical': return 'var(--critical)';
    case 'high': return 'var(--high)';
    case 'medium': return 'var(--medium)';
    case 'low': return 'var(--low)';
    default: return 'var(--info)';
  }
}

// criticalityFromFactor mirrors internal/scanner/normalize.go CriticalityLabel.
export function criticalityFromFactor(f: number): AssetCriticality {
  if (f >= 2.75) return 'CRITICAL';
  if (f >= 2.0) return 'HIGH';
  if (f >= 1.0) return 'MEDIUM';
  return 'LOW';
}

export function scheduleLabel(minutes: number, lang: 'fr' | 'en'): string {
  switch (minutes) {
    case 60: return lang === 'fr' ? 'Horaire' : 'Hourly';
    case 1440: return lang === 'fr' ? 'Quotidien' : 'Daily';
    case 10080: return lang === 'fr' ? 'Hebdo' : 'Weekly';
    case 0: return lang === 'fr' ? 'Manuel' : 'Manual';
    default: return lang === 'fr' ? `${minutes} min` : `${minutes} min`;
  }
}

export function timeAgo(iso: string | null | undefined, lang: 'fr' | 'en'): string {
  if (!iso) return '—';
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return '—';
  const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
  const ago = lang === 'fr' ? 'il y a' : '';
  const suffix = lang === 'fr' ? '' : 'ago';
  const fmt = (n: number, u: string) => (lang === 'fr' ? `${ago} ${n}${u}` : `${n}${u} ${suffix}`).trim();
  if (secs < 60) return lang === 'fr' ? "à l'instant" : 'just now';
  if (secs < 3600) return fmt(Math.floor(secs / 60), 'm');
  if (secs < 86400) return fmt(Math.floor(secs / 3600), 'h');
  return fmt(Math.floor(secs / 86400), 'd');
}
