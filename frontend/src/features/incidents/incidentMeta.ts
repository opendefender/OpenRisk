// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Shared incident vocabulary (severity/status labels + colors) so the register,
// the detail drawer and the War Room all render the same badges.

import type { IncidentSeverity, IncidentStatus } from './incidentService';

export const SEV: Record<IncidentSeverity, { color: string; fr: string; en: string }> = {
  critical: { color: 'var(--critical)', fr: 'Critique', en: 'Critical' },
  high: { color: 'var(--high)', fr: 'Élevée', en: 'High' },
  medium: { color: 'var(--medium)', fr: 'Moyenne', en: 'Medium' },
  low: { color: 'var(--low)', fr: 'Faible', en: 'Low' },
};

export const STATUS: Record<IncidentStatus, { color: string; fr: string; en: string }> = {
  open: { color: 'var(--critical)', fr: 'Ouvert', en: 'Open' },
  in_progress: { color: 'var(--high)', fr: 'En cours', en: 'In progress' },
  resolved: { color: 'var(--low)', fr: 'Résolu', en: 'Resolved' },
  closed: { color: 'var(--text-muted)', fr: 'Clos', en: 'Closed' },
};

export const STATUSES: IncidentStatus[] = ['open', 'in_progress', 'resolved', 'closed'];
export const SEVERITIES: IncidentSeverity[] = ['critical', 'high', 'medium', 'low'];
export const TYPES = ['breach', 'attack', 'vulnerability', 'data_loss', 'phishing', 'malware', 'other'];

export const sevMeta = (s: IncidentSeverity) => SEV[s] ?? SEV.medium;
export const statusMeta = (s: IncidentStatus) => STATUS[s] ?? STATUS.open;
