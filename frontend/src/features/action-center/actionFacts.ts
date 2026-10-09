// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The grey line under an action's title (#902): "Vulnérabilité · KEV · SLA
// dépassé · CVE-2025-55241 · ad-dc-01". Built from the facts the server sends
// in `meta` and from the due date; the server never words it.

import type { ActionItem } from './actionCenterService';
import { daysUntil } from './dueDate';
import { kindOf } from './actionKinds';
import { PRIORITY_CODE } from '../incidents/incidentMeta';
import type { IncidentSeverity } from '../incidents/incidentService';

type T = (key: string, params?: Record<string, string | number>) => string;

export function actionFacts(item: ActionItem, t: T, now = new Date()): string {
  const meta = item.meta ?? {};
  const parts: string[] = [
    kindOf(item.type) ? t(`actionCenter.kind.${item.type}`) : t('actionCenter.types.unknown'),
  ];
  const late = item.due_at ? daysUntil(item.due_at, now) < 0 : false;

  switch (item.type) {
    case 'vulnerability_sla':
      if (meta.kev === 'true') parts.push(t('actionCenter.fact.kev'));
      parts.push(late ? t('actionCenter.fact.slaBreached') : t('actionCenter.fact.slaDue'));
      if (meta.cve_id) parts.push(meta.cve_id);
      if (meta.asset_name) parts.push(meta.asset_name);
      break;
    case 'overdue_mitigation':
      parts.push(t('actionCenter.fact.overdue'));
      break;
    case 'mitigation_review':
      parts.push(t('actionCenter.fact.inReview'));
      break;
    case 'critical_risk':
      parts.push(t('actionCenter.fact.noPlan'));
      break;
    case 'open_incident': {
      const code = PRIORITY_CODE[meta.severity as IncidentSeverity];
      if (code) parts.push(code);
      break;
    }
    case 'expiring_evidence':
      parts.push(late ? t('actionCenter.fact.expired') : t('actionCenter.fact.expiring'));
      break;
    case 'vendor_followup':
      if (meta.sent_at) {
        parts.push(t('actionCenter.fact.unanswered', { count: Math.max(0, -daysUntil(meta.sent_at, now)) }));
      }
      break;
    default:
      break;
  }
  return parts.join(' · ');
}

/** Which due group an item falls in: overdue, within 7 days, later, or none. */
export type DueGroup = 'late' | 'week' | 'later' | 'none';

export function dueGroup(item: ActionItem, now = new Date()): DueGroup {
  if (!item.due_at) return 'none';
  const n = daysUntil(item.due_at, now);
  if (n < 0) return 'late';
  if (n <= 7) return 'week';
  return 'later';
}
