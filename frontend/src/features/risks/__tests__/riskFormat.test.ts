// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// ONE display format for a risk's figures (#913, D-069 A). The table, the
// matrix, the drawer, the modal and the CSV all print through these four
// functions, so these assertions are the contract of every screen.

import { describe, expect, it } from 'vitest';
import { formatImpact, formatPI, formatProbability, formatScore } from '../riskFormat';

describe('formatScore — one decimal, no scale suffix', () => {
  it('prints one decimal in each language', () => {
    expect(formatScore('fr', 7.2)).toBe('7,2');
    expect(formatScore('en', 7.2)).toBe('7.2');
  });

  it('rounds to one decimal, never more', () => {
    expect(formatScore('en', 6.96)).toBe('7.0');
    expect(formatScore('en', 4)).toBe('4.0');
    expect(formatScore('fr', 0)).toBe('0,0');
  });

  it('says "no value" with a dash, never a zero', () => {
    expect(formatScore('fr', null)).toBe('—');
    expect(formatScore('fr', undefined)).toBe('—');
    expect(formatScore('en', Number.NaN)).toBe('—');
  });

  it('never carries a "/25" or "/100" scale: the formula is P × I × criticality', () => {
    expect(formatScore('fr', 12.5)).not.toMatch(/\//);
  });
});

describe('formatProbability and formatImpact', () => {
  it('prints the probability with two decimals', () => {
    expect(formatProbability('fr', 0.62)).toBe('0,62');
    expect(formatProbability('en', 0.5)).toBe('0.50');
    expect(formatProbability('fr', null)).toBe('—');
  });

  it('prints the impact with one decimal', () => {
    expect(formatImpact('fr', 8)).toBe('8,0');
    expect(formatImpact('en', 7.5)).toBe('7.5');
    expect(formatImpact('en', undefined)).toBe('—');
  });
});

describe('formatPI', () => {
  it('writes the two inputs beside a score the way the table shows them', () => {
    expect(formatPI('fr', 0.62, 8)).toBe('P 0,62 · I 8,0');
    expect(formatPI('en', 0.62, 8)).toBe('P 0.62 · I 8.0');
  });

  it('keeps the shape when a term is missing', () => {
    expect(formatPI('fr', null, 8)).toBe('P — · I 8,0');
  });
});
