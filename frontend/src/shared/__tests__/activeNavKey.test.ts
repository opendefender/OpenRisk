// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which sidebar item a location lights up.

import { describe, expect, it } from 'vitest';
import { ALL_NAV_ITEMS, activeNavKey } from '../navModel';

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
});
