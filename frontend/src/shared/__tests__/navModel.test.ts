// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #270 / UX-16 — the sidebar is grouped by intention, at most seven top-level
// entries, and no entry lives in two groups.
//
// The merge 91df861 left a second `g_pilot` group carrying risks,
// vulnerabilities, mitigations, incidents and automation, which were already in
// Évaluer, Identifier and Traiter: seven groups, five entries rendered twice,
// and nothing failed. These tests are what fails now.

import { describe, it, expect } from 'vitest';

import { NAV_GROUPS, pinnedItems, visibleNavGroups } from '../navModel';

const target = (i: { href?: string; path: string }) => i.href ?? i.path;

describe('navigation model (UX-16)', () => {
  it('follows the ratified intentions, in the order of the work', () => {
    expect(NAV_GROUPS.map((g) => g.groupKey)).toEqual([
      'g_pilot',
      'g_identify',
      'g_evaluate',
      'g_treat',
      'g_prove',
      'g_admin',
    ]);
  });

  it('has at most seven top-level entries, pinned entries included', () => {
    expect(pinnedItems(NAV_GROUPS).length + NAV_GROUPS.length).toBeLessThanOrEqual(7);
  });

  it('never puts an entry in two groups', () => {
    const keys = NAV_GROUPS.flatMap((g) => g.items.map((i) => i.key));
    const links = NAV_GROUPS.flatMap((g) => g.items.map(target));
    expect(keys.filter((k, i) => keys.indexOf(k) !== i)).toEqual([]);
    expect(links.filter((l, i) => links.indexOf(l) !== i)).toEqual([]);
  });

  it('keeps each core entry in its ratified intention', () => {
    const groupOf = (key: string) =>
      NAV_GROUPS.find((g) => g.items.some((i) => i.key === key))?.groupKey;
    expect(groupOf('dashboard')).toBe('g_pilot');
    expect(groupOf('vulnerabilities')).toBe('g_identify');
    expect(groupOf('vendors')).toBe('g_identify');
    expect(groupOf('risks')).toBe('g_evaluate');
    expect(groupOf('mitigations')).toBe('g_treat');
    expect(groupOf('incidents')).toBe('g_treat');
    expect(groupOf('automation')).toBe('g_treat');
    expect(groupOf('compliance')).toBe('g_prove');
  });

  it('drops a group a restricted role cannot use, without duplicating the rest', () => {
    // An auditor-like role: compliance only. Évaluer and Traiter have nothing
    // left for them and must disappear rather than render as empty headings.
    const can = (perm: string) => perm === 'compliance:read';
    const groups = visibleNavGroups(can, false).map((g) => g.groupKey);
    expect(groups).not.toContain('g_evaluate');
    expect(groups).not.toContain('g_treat');
    expect(groups).toContain('g_prove');
    expect(new Set(groups).size).toBe(groups.length);
  });
});
