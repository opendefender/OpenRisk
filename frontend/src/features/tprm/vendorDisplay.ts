// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// How the vendor screens name backend values (#673). Every mapping normalises
// its input and has a default: an unexpected casing or a value added server-side
// renders as itself, never as a crash (frontend charter, crash pattern 1).

import axios from 'axios';
import type { BadgeIntent } from '../../shared/ds';

export type VendorTier = 'critical' | 'high' | 'medium' | 'low';
export type AssessmentStatus = 'sent' | 'in_progress' | 'submitted' | 'revoked' | 'expired';

export function knownTier(tier: string | null | undefined): VendorTier | null {
  switch ((tier ?? '').toLowerCase()) {
    case 'critical':
      return 'critical';
    case 'high':
      return 'high';
    case 'medium':
      return 'medium';
    case 'low':
      return 'low';
    default:
      return null;
  }
}

export function tierIntent(tier: string | null | undefined): BadgeIntent {
  switch (knownTier(tier)) {
    case 'critical':
      return 'danger';
    case 'high':
      return 'warning';
    case 'medium':
      return 'info';
    case 'low':
      return 'success';
    default:
      return 'neutral';
  }
}

export function knownStatus(status: string | null | undefined): AssessmentStatus | null {
  switch ((status ?? '').toLowerCase()) {
    case 'sent':
      return 'sent';
    case 'in_progress':
      return 'in_progress';
    case 'submitted':
      return 'submitted';
    case 'revoked':
      return 'revoked';
    case 'expired':
      return 'expired';
    default:
      return null;
  }
}

export function statusIntent(status: string | null | undefined): BadgeIntent {
  switch (knownStatus(status)) {
    case 'sent':
    case 'in_progress':
      return 'info';
    case 'submitted':
      return 'success';
    case 'revoked':
    case 'expired':
    default:
      return 'neutral';
  }
}

/** Revoke and resend apply only to a questionnaire the vendor can still answer. */
export function isOpenAssessment(status: string | null | undefined): boolean {
  const s = knownStatus(status);
  return s === 'sent' || s === 'in_progress';
}

/**
 * The label key for a stored link verb. hosted_on and stores_data_in are the
 * stored aliases of hosted_by and processes_data_of (ADR 0004 D2).
 */
export function verbKey(verb: string | null | undefined): string {
  switch ((verb ?? '').toLowerCase()) {
    case 'managed_by':
      return 'managed_by';
    case 'hosted_by':
    case 'hosted_on':
      return 'hosted_by';
    case 'processes_data_of':
    case 'stores_data_in':
      return 'processes_data_of';
    case 'depends_on':
      return 'depends_on';
    default:
      return 'other';
  }
}

/** The shipped vendor schema's service_criticality values (asset_schema_defaults.go). */
export const SERVICE_CRITICALITIES = ['faible', 'moyenne', 'haute', 'critique'] as const;

export function knownServiceCriticality(v: string | null | undefined): string | null {
  const s = (v ?? '').toLowerCase();
  return (SERVICE_CRITICALITIES as readonly string[]).includes(s) ? s : null;
}

/** A risk's criticality, as the chain returns it. */
export function knownRiskLevel(v: string | null | undefined): VendorTier | null {
  return knownTier(v);
}

export function httpStatus(err: unknown): number | undefined {
  return axios.isAxiosError(err) ? err.response?.status : undefined;
}

export function formatNumber(value: number, locale: string, digits = 1): string {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(value);
}
