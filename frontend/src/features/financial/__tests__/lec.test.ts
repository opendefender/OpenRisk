// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';

import { axisMax, curvePath, exceedance, niceCeil, paybackTone, ticks } from '../lec';

/** Percentiles of a uniform loss on [0, 100]: q[p] = p. */
const UNIFORM = Array.from({ length: 101 }, (_, p) => p);

describe('#904 — loss-exceedance maths', () => {
  it('reads the probability of exceeding an amount off the percentiles', () => {
    expect(exceedance(UNIFORM, -1)).toBe(1);
    expect(exceedance(UNIFORM, 0)).toBeCloseTo(1, 6);
    expect(exceedance(UNIFORM, 32)).toBeCloseTo(0.68, 6);
    expect(exceedance(UNIFORM, 32.5)).toBeCloseTo(0.675, 6);
    expect(exceedance(UNIFORM, 100)).toBe(0);
    expect(exceedance(UNIFORM, 1e9)).toBe(0);
  });

  it('is never increasing', () => {
    let prev = 1;
    for (let x = -5; x <= 105; x += 0.5) {
      const p = exceedance(UNIFORM, x);
      expect(p).toBeLessThanOrEqual(prev);
      prev = p;
    }
  });

  it('handles a register with one fixed loss (all percentiles equal)', () => {
    const flat = Array.from({ length: 101 }, () => 50);
    expect(exceedance(flat, 49)).toBe(1);
    expect(exceedance(flat, 50)).toBe(0);
    expect(exceedance([], 10)).toBe(0);
  });

  it('rounds axes to readable values and keeps the appetite on screen', () => {
    expect(niceCeil(117)).toBe(200);
    expect(niceCeil(230)).toBe(250);
    expect(niceCeil(0)).toBe(1);
    expect(axisMax(UNIFORM, null)).toBe(200); // 99 × 1.15 → 113.85 → 200
    expect(axisMax(UNIFORM, 300)).toBe(500);
    expect(ticks(250, 5)).toEqual([0, 50, 100, 150, 200, 250]);
  });

  it('draws from the top-left down to the bottom-right', () => {
    const d = curvePath(UNIFORM, 200, { x0: 50, x1: 250, y0: 10, y1: 110 }, 4);
    expect(d).toBe('M50.0 10.0 L100.0 60.0 L150.0 110.0 L200.0 110.0 L250.0 110.0');
  });

  it('colours payback like the design', () => {
    expect(paybackTone(5)).toBe('var(--success-text)');
    expect(paybackTone(15)).toBe('var(--fg-primary)');
    expect(paybackTone(38)).toBe('var(--warning-text)');
  });
});
