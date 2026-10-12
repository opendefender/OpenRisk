// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import { Boxes, KeyRound, Laptop, Server } from 'lucide-react';

import type { Asset } from '../../../types/asset';
import { categoryCounts, locationOf, slugOf, typeIconOf } from '../inventoryRow';

const asset = (over: Partial<Asset>): Asset => ({ name: 'x', ...over }) as Asset;

describe('#906 — inventory row', () => {
  it('names an asset technically from what it carries', () => {
    expect(slugOf(asset({ hostnames: ['srv-core-01'] }))).toBe('srv-core-01');
    expect(slugOf(asset({ attributes: { hostname: 'ad-dc-01' } }))).toBe('ad-dc-01');
    expect(slugOf(asset({ attributes: { resource_id: 'm365' } }))).toBe('m365');
    expect(slugOf(asset({ ip_addresses: ['10.20.0.1'] }))).toBe('10.20.0.1');
    expect(slugOf(asset({}))).toBe('');
  });

  it('reads the location attributes, nothing invented', () => {
    expect(locationOf(asset({ attributes: { physical_location: 'DC Dakar' } }))).toBe('DC Dakar');
    expect(locationOf(asset({ attributes: { region: 'eu-west-3' } }))).toBe('eu-west-3');
    expect(locationOf(asset({ attributes: { environment: 'production' } }))).toBe('');
  });

  it('picks the icon by type, then category', () => {
    expect(typeIconOf(asset({ type: 'Serveur' }))).toBe(Server);
    expect(typeIconOf(asset({ type: 'Annuaire' }))).toBe(KeyRound);
    expect(typeIconOf(asset({ type: 'Poste maison', category: 'workstation' }))).toBe(Laptop);
    expect(typeIconOf(asset({ type: 'Inconnu' }))).toBe(Boxes);
  });

  it('counts categories in the schema order, uncategorised last', () => {
    const got = categoryCounts([
      asset({ category: 'cloud' }),
      asset({ category: 'server' }),
      asset({ category: 'cloud' }),
      asset({}),
    ]);
    expect(got).toEqual([
      { id: 'all', n: 4 },
      { id: 'server', n: 1 },
      { id: 'cloud', n: 2 },
      { id: 'none', n: 1 },
    ]);
  });
});
