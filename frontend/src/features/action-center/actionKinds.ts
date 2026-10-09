// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// How each action-center type is filtered, drawn, worded and completed in the
// October 2026 redesign (#902).
//
// Completion is real (owner decision 2026-10-08): ticking an item does the work
// the item asks for — the mitigation goes to Done, the finding to Remediated,
// the incident to Resolved, the remediation plan to Completed — through the
// same endpoint the record's own screen uses, so the same permission applies.
// Types that need a judgement (approve, review a risk, renew a proof, chase a
// vendor) have no tick.

import {
  Bug,
  CheckCheck,
  ClipboardCheck,
  FolderClock,
  Handshake,
  ListChecks,
  ShieldAlert,
  ShieldCheck,
  Siren,
  type LucideIcon,
} from 'lucide-react';
import { api } from '../../lib/api';
import type { ActionItem, ActionItemType } from './actionCenterService';

/** The redesign's filter chips. `incident` shows only when there is one. */
export type ActionFilter = 'all' | 'approval' | 'vuln' | 'plans' | 'incident' | 'compliance' | 'vendor';

export const FILTER_ORDER: ActionFilter[] = [
  'all',
  'approval',
  'vuln',
  'plans',
  'incident',
  'compliance',
  'vendor',
];

interface KindSpec {
  filter: Exclude<ActionFilter, 'all'>;
  icon: LucideIcon;
  /** Completes the record for real. Absent when the work needs a judgement. */
  complete?: (id: string) => Promise<unknown>;
}

const KINDS: Record<ActionItemType, KindSpec> = {
  pending_approval: { filter: 'approval', icon: CheckCheck },
  vulnerability_sla: {
    filter: 'vuln',
    icon: Bug,
    complete: (id) => api.patch(`/vulnerabilities/${id}/status`, { status: 'remediated' }),
  },
  overdue_mitigation: {
    filter: 'plans',
    icon: ShieldCheck,
    complete: (id) => api.patch(`/mitigations/${id}`, { status: 'DONE' }),
  },
  mitigation_review: {
    filter: 'plans',
    icon: ShieldCheck,
    complete: (id) => api.patch(`/mitigations/${id}`, { status: 'DONE' }),
  },
  critical_risk: { filter: 'plans', icon: ShieldAlert },
  open_incident: {
    filter: 'incident',
    icon: Siren,
    complete: (id) => api.put(`/incidents/${id}`, { status: 'resolved' }),
  },
  expiring_evidence: { filter: 'compliance', icon: FolderClock },
  overdue_remediation: {
    filter: 'compliance',
    icon: ClipboardCheck,
    complete: (id) => api.patch(`/compliance/remediations/${id}`, { status: 'completed' }),
  },
  vendor_followup: { filter: 'vendor', icon: Handshake },
};

export function kindOf(type: string): KindSpec | undefined {
  return KINDS[type as ActionItemType];
}

export function iconFor(type: string): LucideIcon {
  return kindOf(type)?.icon ?? ListChecks;
}

export function filterOf(item: ActionItem): Exclude<ActionFilter, 'all'> | undefined {
  return kindOf(item.type)?.filter;
}

/** The completion call for an item, or undefined when it cannot be ticked. */
export function completionFor(item: ActionItem): (() => Promise<unknown>) | undefined {
  const complete = kindOf(item.type)?.complete;
  if (!complete) return undefined;
  return () => complete(item.subject_resource_id);
}
