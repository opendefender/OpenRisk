// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which sidebar item a location lights up.

import { describe, expect, it } from 'vitest';
import { ALL_NAV_ITEMS, activeNavKey, navGroupFor, navItemFor } from '../navModel';

const key = (path: string, search = '') => activeNavKey(ALL_NAV_ITEMS, path, search);

describe('activeNavKey', () => {
  it('takes the longest matching path', () => {
    expect(key('/assets/topology')).toBe(key('/assets/topology/x'));
    expect(key('/assets/topology')).not.toBe(key('/assets'));
  });

  it('tells Dashboard from Executive by ?view=', () => {
    expect(key('/', '')).not.toBe(key('/', '?view=executive'));
  });

  // #911: the CVE feed moved under Threat Intel and keeps it lit.
  it('lights Threat Intel on its page and on the CVE feed', () => {
    expect(key('/threat-intel')).toBe('cti');
    expect(key('/threat-map')).toBe('cti');
  });

  // #913: the register's ?view=matrix is a display mode, not another item.
  it('keeps the register lit in matrix mode, and the breadcrumb on its group', () => {
    expect(key('/risks', '?view=matrix')).toBe(key('/risks', ''));
    expect(key('/risks', '?view=matrix')).toBe('risks');
    expect(navItemFor('/risks', 'matrix')).toBe(navItemFor('/risks', null));
    expect(navGroupFor('/risks', 'matrix')).toBe(navGroupFor('/risks', null));
    expect(navGroupFor('/risks', 'matrix')).toBeDefined();
  });
});
