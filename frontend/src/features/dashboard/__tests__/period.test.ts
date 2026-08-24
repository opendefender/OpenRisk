// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest';

import {
  DEFAULT_PERIOD,
  PERIOD_PRESETS,
  periodKey,
  periodLabel,
  periodParams,
  periodToSearchParams,
  readPeriod,
  type PeriodSelection,
} from '../period';

const read = (qs: string) => readPeriod(new URLSearchParams(qs));

describe('readPeriod', () => {
  it('defaults to the unbounded window, matching the server', () => {
    // Not 30d: the headline counters are stock quantities, and defaulting them
    // to a window would put the dashboard permanently at odds with the register
    // on first paint with nothing on screen to say a filter was applied.
    expect(read('')).toEqual({ kind: 'preset', preset: DEFAULT_PERIOD });
    expect(DEFAULT_PERIOD).toBe('all');
  });

  it('reads each supported preset', () => {
    for (const preset of PERIOD_PRESETS) {
      expect(read(`period=${preset}`)).toEqual({ kind: 'preset', preset });
    }
  });

  it('falls back to the default rather than sending something the server would reject', () => {
    // The client's contract mirrors the server's: the server 400s a malformed
    // period rather than answering a different question, and the client never
    // *sends* one it could not parse.
    for (const bad of ['6m', 'yesterday', '', 'custom', '30days', 'ALL']) {
      expect(read(`period=${bad}`)).toEqual({ kind: 'preset', preset: DEFAULT_PERIOD });
    }
  });

  it('reads an explicit range, and prefers it over a preset', () => {
    expect(read('from=2026-08-01&to=2026-09-01')).toEqual({
      kind: 'custom',
      from: '2026-08-01',
      to: '2026-09-01',
    });
    expect(read('period=7d&from=2026-08-01&to=2026-09-01')).toEqual({
      kind: 'custom',
      from: '2026-08-01',
      to: '2026-09-01',
    });
  });

  it('rejects an unusable range instead of shipping it', () => {
    const cases = [
      'from=2026-08-01', // no end
      'to=2026-09-01', // no start
      'from=01/08/2026&to=01/09/2026', // not ISO
      'from=2026-09-01&to=2026-08-01', // inverted
      'from=2026-08-01&to=2026-08-01', // empty
    ];
    for (const qs of cases) {
      expect(read(qs)).toEqual({ kind: 'preset', preset: DEFAULT_PERIOD });
    }
  });
});

describe('periodParams', () => {
  it('sends a preset as ?period and a range as ?from/?to', () => {
    expect(periodParams({ kind: 'preset', preset: '30d' })).toEqual({ period: '30d' });
    expect(periodParams({ kind: 'custom', from: '2026-08-01', to: '2026-09-01' })).toEqual({
      from: '2026-08-01',
      to: '2026-09-01',
    });
  });

  it('never sends both, because the server treats that as a contradiction', () => {
    const params = periodParams({ kind: 'custom', from: '2026-08-01', to: '2026-09-01' });
    expect(params).not.toHaveProperty('period');
  });
});

describe('periodKey', () => {
  it('distinguishes every window, so two windows never share a cache entry', () => {
    const keys = [
      periodKey({ kind: 'preset', preset: 'all' }),
      periodKey({ kind: 'preset', preset: '7d' }),
      periodKey({ kind: 'preset', preset: '30d' }),
      periodKey({ kind: 'preset', preset: '90d' }),
      periodKey({ kind: 'custom', from: '2026-08-01', to: '2026-09-01' }),
      periodKey({ kind: 'custom', from: '2026-07-01', to: '2026-08-01' }),
    ];
    expect(new Set(keys).size).toBe(keys.length);
  });

  it('is stable for the same selection', () => {
    const sel: PeriodSelection = { kind: 'custom', from: '2026-08-01', to: '2026-09-01' };
    expect(periodKey(sel)).toBe(periodKey({ ...sel }));
  });
});

describe('URL round trip', () => {
  it('survives serialise then parse for every window', () => {
    const selections: PeriodSelection[] = [
      { kind: 'preset', preset: 'all' },
      { kind: 'preset', preset: '7d' },
      { kind: 'preset', preset: '30d' },
      { kind: 'preset', preset: '90d' },
      { kind: 'custom', from: '2026-08-01', to: '2026-09-01' },
    ];
    for (const sel of selections) {
      const qs = periodToSearchParams(sel).toString();
      expect(readPeriod(new URLSearchParams(qs))).toEqual(sel);
    }
  });

  it('omits the default so a clean dashboard has a clean URL', () => {
    expect(periodToSearchParams({ kind: 'preset', preset: 'all' }).toString()).toBe('');
    expect(periodToSearchParams({ kind: 'preset', preset: '7d' }).toString()).toBe('period=7d');
  });

  it('preserves params it does not own', () => {
    // ?focus= from universal search, ?view=executive, a table's facets — none of
    // them belong to the period control and none may be dropped by it.
    const into = new URLSearchParams('view=executive&f.criticality=critical');
    const out = periodToSearchParams({ kind: 'preset', preset: '30d' }, into);
    expect(out.get('view')).toBe('executive');
    expect(out.get('f.criticality')).toBe('critical');
    expect(out.get('period')).toBe('30d');
  });
});

describe('periodLabel', () => {
  it('labels every preset in both languages, so a new one cannot ship unlabelled', () => {
    for (const preset of PERIOD_PRESETS) {
      for (const lang of ['fr', 'en'] as const) {
        const label = periodLabel({ kind: 'preset', preset }, lang);
        expect(label).toBeTruthy();
        expect(label).not.toBe(preset);
      }
    }
  });

  it('shows a custom range as its dates', () => {
    expect(periodLabel({ kind: 'custom', from: '2026-08-01', to: '2026-09-01' }, 'en'))
      .toBe('2026-08-01 → 2026-09-01');
  });
});
