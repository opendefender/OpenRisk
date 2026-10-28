// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Turns the risk's merged timeline into HistoryTimeline entries (#793).
//
// Every event comes from the audit trail, and the author is the one the trail
// recorded: a person, a token's owner, or a job by name such as the Score Engine.
// That is the answer the score working gives too. A write that carried no
// identity says so rather than being dressed up as "System".

import type { HistoryEntry } from '../../shared/HistoryTimeline';
import type { EntityActor, TimelineEvent } from '../entity-drawer/types';

type Tr = (fr: string, en: string) => string;

export function timelineActor(a: EntityActor | undefined, tr: Tr): string {
  if (!a || a.kind === 'unattributed') return tr('Non attribué', 'Unattributed');
  if (a.kind === 'job') return tr(`tâche ${a.label ?? ''}`, `job ${a.label ?? ''}`);
  const who = a.label || a.email || (a.id ? a.id.slice(0, 8) : '');
  if (!who) return tr('Non attribué', 'Unattributed');
  if (a.kind === 'service_token') return tr(`jeton de ${who}`, `token of ${who}`);
  return who;
}

function titleOf(e: TimelineEvent, tr: Tr): string {
  switch (e.kind) {
    case 'create':
      return tr('Risque créé', 'Risk created');
    case 'mitigate':
      return tr('Mitigation appliquée', 'Mitigation applied');
    case 'delete':
      return tr('Risque supprimé', 'Risk deleted');
    case 'update':
      return tr('Mise à jour', 'Updated');
    default:
      return e.summary;
  }
}

export function toHistoryEntries(events: TimelineEvent[], tr: Tr): HistoryEntry[] {
  return events.map((e) => ({
    id: e.id,
    kind: e.kind,
    title: titleOf(e, tr),
    actor: timelineActor(e.actor, tr),
    at: e.occurred_at,
    fields:
      e.changes && e.changes.length > 0
        ? [{ label: tr('Champs', 'Fields'), value: e.changes.map((c) => c.field).join(', ') }]
        : undefined,
  }));
}
