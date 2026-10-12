// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// How one journal entry reads (#905): "Fatou Ndiaye a créé la mitigation
// Bastion d'administration (PAM)". Pure helpers so the wording is tested
// without a browser.

import type { TimelineEvent } from '../entity-drawer/types';

/** The filter pills, in the design's order. '' is "Tout". */
export const JOURNAL_FILTERS = [
  '',
  'risk',
  'incident',
  'vulnerability',
  'mitigation',
  'evidence',
  'compliance',
  'report',
] as const;
export type JournalFilter = (typeof JOURNAL_FILTERS)[number];

const VERBS = new Set([
  'create',
  'update',
  'delete',
  'approve',
  'reject',
  'defer',
  'submit',
  'cancel',
  'simulate',
]);
const DOMAINS = new Set([
  'risk',
  'incident',
  'vulnerability',
  'mitigation',
  'evidence',
  'compliance',
  'report',
  'asset',
  'governance',
]);

/** The locale keys of the verb phrase and the noun that follows it. */
export function phraseKeys(e: TimelineEvent): { verb: string; noun: string | null } {
  // Governance objects name themselves ("Budget du PRA…"): no noun before them.
  const domain = e.domain && DOMAINS.has(e.domain) && e.domain !== 'governance' ? e.domain : null;
  if (e.kind === 'update' && e.changes?.some((c) => c.field === 'status')) {
    return {
      verb: 'activity.verb.status',
      noun: domain ? `activity.nounAfterDe.${domain}` : null,
    };
  }
  return {
    verb: VERBS.has(e.kind) ? `activity.verb.${e.kind}` : 'activity.verb.fallback',
    noun: domain ? `activity.noun.${domain}` : null,
  };
}

/** Local calendar day of an instant, "2026-10-09". */
export function dayKey(iso: string): string {
  const d = new Date(iso);
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${m}-${day}`;
}

/** 'today' | 'yesterday' | null for any other day, relative to `now`. */
export function relativeDay(iso: string, now: Date): 'today' | 'yesterday' | null {
  const k = dayKey(iso);
  if (k === dayKey(now.toISOString())) return 'today';
  const y = new Date(now);
  y.setDate(y.getDate() - 1);
  if (k === dayKey(y.toISOString())) return 'yesterday';
  return null;
}

/** Events grouped by local day, newest day first, order kept inside a day. */
export function groupByDay(events: TimelineEvent[]): { day: string; events: TimelineEvent[] }[] {
  const out: { day: string; events: TimelineEvent[] }[] = [];
  for (const e of events) {
    const k = dayKey(e.occurred_at);
    const last = out[out.length - 1];
    if (last && last.day === k) last.events.push(e);
    else out.push({ day: k, events: [e] });
  }
  return out;
}

/** "Fatou Ndiaye" → "FN"; "admin@opendefender.io" → "AD". */
export function initialsOf(label: string): string {
  const base = label.includes('@') ? label.split('@')[0] : label;
  const words = base.split(/[\s._-]+/).filter(Boolean);
  if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase();
  return (words[0] ?? '?').slice(0, 2).toUpperCase();
}
