// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #793: the risk Timeline tab names the author the audit trail recorded, never
// the risk's creator and never a made-up "System".

import { describe, expect, it } from 'vitest';
import { toHistoryEntries, timelineActor } from '../riskTimelineEntries';
import type { TimelineEvent } from '../../entity-drawer/types';

const fr = (f: string) => f;
const en = (_f: string, e: string) => e;

const ev = (over: Partial<TimelineEvent>): TimelineEvent => ({
  id: 'e1',
  kind: 'update',
  occurred_at: '2026-10-28T10:00:00Z',
  target: { type: 'risk', id: 'r1' },
  summary: 'Updated risk',
  source: 'audit',
  ...over,
});

describe('risk timeline entries', () => {
  it('names the user who made the edit', () => {
    const [entry] = toHistoryEntries(
      [
        ev({
          actor: { id: 'b0b0b0b0-0000-0000-0000-000000000000', label: 'Bea Martin', kind: 'user' },
          changes: [{ field: 'impact' }, { field: 'probability' }],
        }),
      ],
      fr,
    );
    expect(entry.actor).toBe('Bea Martin');
    expect(entry.title).toBe('Mise à jour');
    expect(entry.fields).toEqual([{ label: 'Champs', value: 'impact, probability' }]);
  });

  it('names the Score Engine as a job, as the score working does', () => {
    const actor = { label: 'score-engine', kind: 'job' as const };
    expect(timelineActor(actor, fr)).toBe('tâche score-engine');
    expect(timelineActor(actor, en)).toBe('job score-engine');
  });

  it('names a token by its owner', () => {
    expect(
      timelineActor(
        { id: 'a1', email: 'ops@example.com', label: 'ops@example.com', kind: 'service_token' },
        en,
      ),
    ).toBe('token of ops@example.com');
  });

  it('falls back to the email, then the short id', () => {
    expect(timelineActor({ email: 'a@example.com' }, en)).toBe('a@example.com');
    expect(timelineActor({ id: '12345678-aaaa-bbbb-cccc-dddddddddddd' }, en)).toBe('12345678');
  });

  it('says a write without identity is unattributed, never "System"', () => {
    expect(timelineActor({ kind: 'unattributed' }, fr)).toBe('Non attribué');
    expect(timelineActor(undefined, en)).toBe('Unattributed');
    expect(timelineActor({}, en)).toBe('Unattributed');
  });
});
