// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest';

import { DESTINATIONS, deepLink } from '../deepLinks';
import type { PeriodSelection } from '../period';

/** Parse a built link so assertions are about params, not string order. */
function parse(link: string) {
  const [path, qs = ''] = link.split('?');
  return { path, params: new URLSearchParams(qs) };
}

describe('deepLink', () => {
  it('carries a tile filter into the register as a URL facet', () => {
    // This is the whole point: clicking "Critical — 3" used to land on an
    // unfiltered list of everything, leaving the user to rebuild by hand the
    // filter they had just expressed by clicking a tile labelled with it.
    const { path, params } = parse(deepLink('risks', { filters: { criticality: 'critical' } }));
    expect(path).toBe('/risks');
    expect(params.get('f.criticality')).toBe('critical');
  });

  it('joins multiple values the way the table reads them', () => {
    const { params } = parse(
      deepLink('assets', { filters: { criticality: ['critical', 'high'] } }),
    );
    expect(params.get('f.criticality')).toBe('critical,high');
  });

  it('drops a facet the destination does not offer', () => {
    // The safe failure is a broader list, never a URL that claims a narrowing
    // the screen will not perform.
    const { params } = parse(
      deepLink('assets', { filters: { severity: 'critical', criticality: 'high' } }),
    );
    expect(params.has('f.severity')).toBe(false);
    expect(params.get('f.criticality')).toBe('high');
  });

  it('drops a sort the destination does not offer', () => {
    const ok = parse(deepLink('risks', { sort: { key: 'score', dir: 'desc' } }));
    expect(ok.params.get('sort')).toBe('score:desc');

    const nope = parse(deepLink('risks', { sort: { key: 'cvss_score', dir: 'desc' } }));
    expect(nope.params.has('sort')).toBe(false);
  });

  it('passes ?focus= through, so a row link opens that row', () => {
    const { params } = parse(deepLink('risks', { focus: 'abc-123' }));
    expect(params.get('focus')).toBe('abc-123');
  });

  it('returns a bare path when there is nothing to carry', () => {
    expect(deepLink('risks')).toBe('/risks');
    expect(deepLink('incidents')).toBe('/incidents');
  });

  it('does not append a period to a destination that has no period control', () => {
    // A period in the URL of a screen that ignores it describes a view the user
    // is not being shown — a quieter version of the same deception.
    const period: PeriodSelection = { kind: 'preset', preset: '30d' };
    for (const key of Object.keys(DESTINATIONS) as (keyof typeof DESTINATIONS)[]) {
      const { params } = parse(deepLink(key, { period }));
      if (!DESTINATIONS[key].period) {
        expect(params.has('period'), `${key} must not receive a period`).toBe(false);
        expect(params.has('from')).toBe(false);
      }
    }
  });

  it('ignores empty and null filter values rather than emitting f.key=', () => {
    const { params } = parse(
      deepLink('risks', { filters: { criticality: '', status: undefined } }),
    );
    expect(params.has('f.criticality')).toBe(false);
    expect(params.has('f.status')).toBe(false);
  });

  it('trims a free-text query and skips a blank one', () => {
    expect(parse(deepLink('risks', { q: '  log4j ' })).params.get('q')).toBe('log4j');
    expect(parse(deepLink('risks', { q: '   ' })).params.has('q')).toBe(false);
  });
});

describe('DESTINATIONS', () => {
  it('declares facet keys that match what the screens register', () => {
    // These strings are the contract between a dashboard tile and a table's
    // facet panel. Pinning them here is what catches a rename on the table side
    // before it silently turns every deep link into an unfiltered list.
    expect(DESTINATIONS.risks.facets).toContain('criticality');
    expect(DESTINATIONS.risks.facets).toContain('status');
    expect(DESTINATIONS.vulnerabilities.facets).toContain('tier');
    expect(DESTINATIONS.vulnerabilities.facets).toContain('kev');
    expect(DESTINATIONS.assets.facets).toContain('criticality');
  });

  it('has a path for every destination', () => {
    for (const dest of Object.values(DESTINATIONS)) {
      expect(dest.path.startsWith('/')).toBe(true);
    }
  });
});
