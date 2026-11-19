// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// `/risks?focus=<id>&tab=<name>` named the old drawer's tabs. The redesigned
// drawer keeps every one of them as a visible tab, after the design's three, so
// each old name still lands on the tab it always named (#913).

import { describe, expect, it } from 'vitest';
import { DRAWER_TABS, resolveDrawerTab } from '../riskDrawerTarget';

describe('resolveDrawerTab', () => {
  it('shows the design tabs first, then every older tab', () => {
    expect([...DRAWER_TABS]).toEqual([
      'overview',
      'miti',
      'history',
      'ownership',
      'lifecycle',
      'score',
      'smart',
      'financial',
      'cti',
      'ai',
    ]);
  });

  it('keeps the timeline redirect working: /risks/:id/timeline → tab=timeline', () => {
    expect(resolveDrawerTab('timeline')).toBe('history');
  });

  it('maps the old names onto the tabs that hold their content', () => {
    expect(resolveDrawerTab('miti')).toBe('miti');
    expect(resolveDrawerTab('details')).toBe('overview');
    for (const s of [
      'lifecycle',
      'score',
      'smart',
      'financial',
      'cti',
      'ai',
      'ownership',
    ] as const) {
      expect(resolveDrawerTab(s)).toBe(s);
    }
  });

  it('normalizes casing and falls back to the overview', () => {
    expect(resolveDrawerTab('CTI')).toBe('cti');
    expect(resolveDrawerTab(' Timeline ')).toBe('history');
    expect(resolveDrawerTab('whatever')).toBe('overview');
    expect(resolveDrawerTab(null)).toBe('overview');
    expect(resolveDrawerTab(undefined)).toBe('overview');
  });
});
