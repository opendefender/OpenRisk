// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Per-source connector metadata for the integration config screen: which
// credential fields each scanner needs, whether it exposes a real live-pull, and
// the base-URL hint. Mirrors internal/vulnscan/livepull + the credential keys the
// backend pullers read.

import type { VulnSource } from './vulnerabilityService';

export interface CredField {
  key: string;
  label: [string, string]; // [fr, en]
  secret?: boolean;
}

export interface SourceMeta {
  label: string;
  category: 'network_scanner' | 'edr' | 'cloud';
  livePull: boolean; // real REST live-pull wired (vs webhook/import only)
  baseUrl?: { label: [string, string]; placeholder: string; required?: boolean };
  creds: CredField[];
}

export const INTEGRATION_META: Record<Exclude<VulnSource, 'scanner' | 'manual'>, SourceMeta> = {
  nessus: {
    label: 'Tenable Nessus / Tenable.io',
    category: 'network_scanner',
    livePull: true,
    baseUrl: { label: ['URL API', 'API URL'], placeholder: 'https://cloud.tenable.com' },
    creds: [
      { key: 'access_key', label: ['Access key', 'Access key'], secret: true },
      { key: 'secret_key', label: ['Secret key', 'Secret key'], secret: true },
    ],
  },
  openvas: {
    label: 'OpenVAS / Greenbone',
    category: 'network_scanner',
    livePull: false,
    baseUrl: { label: ['Hôte GMP', 'GMP host'], placeholder: 'gvm.example.com:9390' },
    creds: [
      { key: 'username', label: ['Utilisateur', 'Username'] },
      { key: 'password', label: ['Mot de passe', 'Password'], secret: true },
    ],
  },
  qualys: {
    label: 'Qualys VMDR',
    category: 'network_scanner',
    livePull: true,
    baseUrl: { label: ['URL du pod', 'Pod URL'], placeholder: 'https://qualysapi.qg3.apps.qualys.com', required: true },
    creds: [
      { key: 'username', label: ['Utilisateur', 'Username'] },
      { key: 'password', label: ['Mot de passe', 'Password'], secret: true },
    ],
  },
  ms_defender: {
    label: 'Microsoft Defender for Endpoint',
    category: 'edr',
    livePull: true,
    creds: [
      { key: 'tenant_id', label: ['Tenant ID', 'Tenant ID'] },
      { key: 'client_id', label: ['Client ID', 'Client ID'] },
      { key: 'client_secret', label: ['Client secret', 'Client secret'], secret: true },
    ],
  },
  crowdstrike: {
    label: 'CrowdStrike Falcon Spotlight',
    category: 'edr',
    livePull: true,
    baseUrl: { label: ['URL API', 'API URL'], placeholder: 'https://api.crowdstrike.com' },
    creds: [
      { key: 'client_id', label: ['Client ID', 'Client ID'] },
      { key: 'client_secret', label: ['Client secret', 'Client secret'], secret: true },
    ],
  },
  aws_inspector: {
    label: 'AWS Inspector',
    category: 'cloud',
    livePull: false,
    creds: [
      { key: 'access_key_id', label: ['Access key ID', 'Access key ID'] },
      { key: 'secret_access_key', label: ['Secret access key', 'Secret access key'], secret: true },
      { key: 'region', label: ['Région', 'Region'] },
    ],
  },
  azure_defender: {
    label: 'Microsoft Defender for Cloud',
    category: 'cloud',
    livePull: true,
    creds: [
      { key: 'tenant_id', label: ['Tenant ID', 'Tenant ID'] },
      { key: 'client_id', label: ['Client ID', 'Client ID'] },
      { key: 'client_secret', label: ['Client secret', 'Client secret'], secret: true },
      { key: 'subscription_id', label: ['Subscription ID', 'Subscription ID'] },
    ],
  },
};

// Ticketing credential schemas.
export const TICKETING_META = {
  jira: {
    label: 'Jira Cloud',
    baseUrl: { label: ['URL Jira', 'Jira URL'], placeholder: 'https://acme.atlassian.net' },
    projectLabel: ['Clé de projet', 'Project key'] as [string, string],
    projectPlaceholder: 'SEC',
    creds: [
      { key: 'email', label: ['Email du compte', 'Account email'] },
      { key: 'api_token', label: ['Jeton API', 'API token'], secret: true },
    ] as CredField[],
  },
  servicenow: {
    label: 'ServiceNow',
    baseUrl: { label: ['URL instance', 'Instance URL'], placeholder: 'https://acme.service-now.com' },
    projectLabel: ['Table', 'Table'] as [string, string],
    projectPlaceholder: 'incident',
    creds: [
      { key: 'username', label: ['Utilisateur', 'Username'] },
      { key: 'password', label: ['Mot de passe', 'Password'], secret: true },
    ] as CredField[],
  },
} as const;
