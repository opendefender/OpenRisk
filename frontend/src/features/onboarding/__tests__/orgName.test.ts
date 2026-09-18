// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #716: the tunnel pre-filled the organisation step with the name sign-up
// invented ("Alex Dembele — espace"), most people kept it, and the danger zone
// then asked them to type it. A placeholder opens empty; a real name still shows.

import { describe, expect, it } from 'vitest';

import { initialOrgName, isProvisionalOrgName } from '../wizard/orgName';

describe('initialOrgName', () => {
  it('opens empty when the organisation carries the person’s own name', () => {
    expect(initialOrgName('Alex Dembele', 'Alex Dembele')).toBe('');
  });

  it('ignores case and stray spaces when comparing with the person’s name', () => {
    expect(initialOrgName('  alex   DEMBELE ', 'Alex Dembele')).toBe('');
  });

  it('opens empty for an organisation that still carries the old suffix, in either language', () => {
    expect(initialOrgName('Alex Dembele — espace', 'Alex Dembele')).toBe('');
    expect(initialOrgName('Alex Dembele — workspace', 'Alex Dembele')).toBe('');
    // The suffix alone marks it, even once the person has changed their name.
    expect(initialOrgName('Alex Dembele — espace', 'Alexandre Dembélé')).toBe('');
  });

  it('keeps a real company name', () => {
    expect(initialOrgName('Banque Atlantique', 'Alex Dembele')).toBe('Banque Atlantique');
  });

  it('keeps a real name when the person’s name is not known yet', () => {
    expect(initialOrgName('Banque Atlantique', undefined)).toBe('Banque Atlantique');
  });

  it('opens empty when there is no organisation name at all', () => {
    expect(initialOrgName(undefined, 'Alex Dembele')).toBe('');
    expect(initialOrgName('   ', 'Alex Dembele')).toBe('');
  });
});

describe('isProvisionalOrgName', () => {
  it('does not mistake a company that merely contains the word "espace"', () => {
    expect(isProvisionalOrgName('Espace Santé Douala', 'Alex Dembele')).toBe(false);
  });

  it('does not treat an empty person’s name as matching everything', () => {
    expect(isProvisionalOrgName('Banque Atlantique', '')).toBe(false);
  });
});
