// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Pure helpers of the vulnerability queue (#909): where a finding stands
// against its remediation deadline, how EPSS reads, which statuses count as
// still to handle, and the signals behind its priority. No React, so the rules
// are tested on their own.

import type { BadgeIntent } from '../../shared/ds';
import type { Vulnerability, VulnStatus } from './vulnerabilityService';

/** Waiting for a decision or a fix. Accepted is a decision, so it is not here. */
export const UNRESOLVED: readonly VulnStatus[] = ['open', 'triaged', 'in_remediation'];

/** What the queue lists unless "Inclure corrigées" is on: fixed and false
 *  positives drop out, accepted findings stay in sight. */
export const QUEUE_STATUSES: readonly VulnStatus[] = [...UNRESOLVED, 'accepted'];

export const isUnresolved = (s: VulnStatus) => UNRESOLVED.includes(s);

export type SlaState =
  | { kind: 'none' }
  | { kind: 'today' }
  | { kind: 'left'; days: number }
  | { kind: 'late'; days: number };

const DAY = 24 * 60 * 60 * 1000;

/**
 * Where a finding stands against its deadline, in whole calendar days. Only an
 * unresolved finding has one: once fixed, accepted or dismissed, the clock no
 * longer applies.
 */
export function slaState(v: Pick<Vulnerability, 'status' | 'sla_due_at'>, now: Date): SlaState {
  if (!isUnresolved(v.status) || !v.sla_due_at) return { kind: 'none' };
  const due = new Date(v.sla_due_at);
  if (Number.isNaN(due.getTime())) return { kind: 'none' };
  const startOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((startOf(due) - startOf(now)) / DAY);
  if (days === 0) return { kind: 'today' };
  return days > 0 ? { kind: 'left', days } : { kind: 'late', days: -days };
}

/** EPSS as a percentage string, or null when the feed gave nothing. */
export function epssPercent(epss: number): string | null {
  if (!epss || epss <= 0) return null;
  const pct = epss * 100;
  if (pct < 1) return '<1';
  return String(Math.round(pct));
}

export const statusIntent: Record<VulnStatus, BadgeIntent> = {
  open: 'neutral',
  triaged: 'neutral',
  in_remediation: 'accent',
  remediated: 'success',
  accepted: 'warning',
  false_positive: 'unavailable',
};

/** The status changes offered from a finding, in the order they are offered. */
export function nextStatuses(s: VulnStatus): VulnStatus[] {
  switch (s) {
    case 'open':
    case 'triaged':
      return ['in_remediation', 'remediated', 'accepted', 'false_positive'];
    case 'in_remediation':
      return ['remediated', 'accepted', 'false_positive'];
    default:
      return ['open'];
  }
}

export type PrioritySignal =
  | { key: 'cvss'; value: number }
  | { key: 'kev' }
  | { key: 'exploit' }
  | { key: 'epss'; value: string }
  | { key: 'asset'; crit: string }
  | { key: 'affected'; count: number };

/**
 * The drivers of the priority, read from the finding itself — the same inputs
 * pkg/vulnprio weighs (CVSS 40 %, exploitation 30 %, asset 20 %, blast radius
 * 10 %), shown in the reader's language rather than the engine's English line.
 */
export function prioritySignals(v: Vulnerability): PrioritySignal[] {
  const out: PrioritySignal[] = [];
  if (v.cvss_score > 0) out.push({ key: 'cvss', value: v.cvss_score });
  if (v.kev) out.push({ key: 'kev' });
  else if (v.exploit_available) out.push({ key: 'exploit' });
  const epss = epssPercent(v.epss);
  if (epss && !v.kev) out.push({ key: 'epss', value: epss });
  if (v.asset_criticality) out.push({ key: 'asset', crit: v.asset_criticality });
  if (v.affected_assets_count > 1) out.push({ key: 'affected', count: v.affected_assets_count });
  return out;
}
