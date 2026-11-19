// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// `/risks?focus=<id>&tab=<name>` named the OLD drawer's nine tabs. The redesigned
// drawer has four; every old name must still land on what it always named (#913).

import { describe, expect, it } from 'vitest';
import { MORE_SECTIONS, resolveDrawerTarget } from '../riskDrawerTarget';

describe('resolveDrawerTarget', () => {
  it('keeps the timeline redirect working: /risks/:id/timeline → tab=timeline', () => {
    expect(resolveDrawerTarget('timeline').tab).toBe('history');
  });

  it('maps the old tab names onto the new ones', () => {
    expect(resolveDrawerTarget('miti').tab).toBe('miti');
    expect(resolveDrawerTarget('details').tab).toBe('overview');
    for (const s of ['lifecycle', 'score', 'smart', 'financial', 'cti', 'ai'] as const) {
      expect(resolveDrawerTarget(s)).toEqual({ tab: 'more', section: s });
    }
  });

  it('normalizes casing', () => {
    expect(resolveDrawerTarget('CTI')).toEqual({ tab: 'more', section: 'cti' });
    expect(resolveDrawerTarget(' Timeline ').tab).toBe('history');
  });

  it('lands on the overview for an unknown or missing name', () => {
    expect(resolveDrawerTarget('whatever').tab).toBe('overview');
    expect(resolveDrawerTarget(null).tab).toBe('overview');
    expect(resolveDrawerTarget(undefined).tab).toBe('overview');
  });

  it('offers every "Plus" section a deep link can name', () => {
    for (const s of MORE_SECTIONS) {
      expect(resolveDrawerTarget(s).tab).toBe('more');
    }
  });
});
