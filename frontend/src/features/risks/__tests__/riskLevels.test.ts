// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The 5-level pickers write the MIDPOINT of a bucket of our scales, and read
// the bucket the stored value falls in. The two must round-trip, or a picker
// would show a level other than the one that was clicked (#913).

import { describe, expect, it } from 'vitest';
import {
  LEVELS,
  cellCrit,
  impactForLevel,
  impactLevel,
  probabilityForLevel,
  probabilityLevel,
  toBucket,
} from '../riskLevels';

describe('level → value (midpoint of the bucket)', () => {
  it('writes P = (2k − 1) / 10', () => {
    expect(LEVELS.map(probabilityForLevel)).toEqual([0.1, 0.3, 0.5, 0.7, 0.9]);
  });

  it('writes I = 2k − 1', () => {
    expect(LEVELS.map(impactForLevel)).toEqual([1, 3, 5, 7, 9]);
  });
});

describe('value → level (the existing toBucket rule)', () => {
  it('round-trips every level, so the clicked level is the pressed one', () => {
    for (const k of LEVELS) {
      expect(probabilityLevel(probabilityForLevel(k))).toBe(k);
      expect(impactLevel(impactForLevel(k))).toBe(k);
    }
  });

  it('bins the edges of each bucket upward-inclusive', () => {
    expect(probabilityLevel(0.2)).toBe(1);
    expect(probabilityLevel(0.21)).toBe(2);
    expect(probabilityLevel(1)).toBe(5);
    expect(impactLevel(2)).toBe(1);
    expect(impactLevel(2.1)).toBe(2);
    expect(impactLevel(10)).toBe(5);
  });

  it('clamps a stray zero or overflow instead of leaving the grid', () => {
    expect(probabilityLevel(0)).toBe(1);
    expect(impactLevel(0)).toBe(1);
    expect(toBucket(50, 10)).toBe(5);
    expect(toBucket(-3, 10)).toBe(1);
  });

  it('bins a real risk the way the matrix does (P 0.62, I 8)', () => {
    expect(probabilityLevel(0.62)).toBe(4);
    expect(impactLevel(8)).toBe(4);
  });
});

describe('cellCrit — the grid tints its own cells, it scores nothing', () => {
  it('tints from the cell coordinates', () => {
    expect(cellCrit(5, 5)).toBe('critical');
    expect(cellCrit(3, 5)).toBe('critical');
    expect(cellCrit(2, 4)).toBe('high');
    expect(cellCrit(2, 2)).toBe('medium');
    expect(cellCrit(1, 3)).toBe('low');
  });
});
