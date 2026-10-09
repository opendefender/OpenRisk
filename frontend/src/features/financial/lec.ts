// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Loss-exceedance curve maths for the financial page (#904). The API sends the
// total annual loss at every percentile 0..100 (101 ascending values, XAF);
// everything here is pure so it can be tested without a browser.

/**
 * Probability that a year's total loss exceeds `x`, read off the percentiles
 * by linear interpolation. 1 below the smallest simulated year, 0 at or above
 * the largest.
 */
export function exceedance(q: readonly number[], x: number): number {
  const n = q.length;
  if (n < 2) return 0;
  if (x < q[0]) return 1;
  if (x >= q[n - 1]) return 0;
  // Last index whose value is ≤ x; flat runs (many identical years) resolve to
  // their right end, so a loss equal to a plateau is "not exceeded" there.
  let i = 0;
  while (i < n - 1 && q[i + 1] <= x) i++;
  const lo = q[i];
  const hi = q[i + 1];
  const frac = hi > lo ? (x - lo) / (hi - lo) : 0;
  const pct = (i + frac) / (n - 1);
  return Math.min(1, Math.max(0, 1 - pct));
}

/** 1, 2, 2.5 or 5 × 10^k, the smallest such value ≥ v. */
export function niceCeil(v: number): number {
  if (!(v > 0)) return 1;
  const exp = Math.floor(Math.log10(v));
  const base = 10 ** exp;
  for (const m of [1, 2, 2.5, 5, 10]) {
    if (m * base >= v) return m * base;
  }
  return 10 * base;
}

/**
 * The x-axis span: the inherent curve's 99th percentile with room to spare, and
 * the appetite if it sits further right, so the threshold is always on screen.
 */
export function axisMax(inherent: readonly number[], appetite: number | null): number {
  const tail = inherent.length > 99 ? inherent[99] : (inherent[inherent.length - 1] ?? 0);
  return niceCeil(Math.max(tail * 1.15, appetite ? appetite * 1.1 : 0, 1));
}

/** `count + 1` evenly spaced ticks from 0 to max. */
export function ticks(max: number, count = 5): number[] {
  return Array.from({ length: count + 1 }, (_, i) => (max * i) / count);
}

/** SVG path of the curve over [0, max] in a box, `steps` segments. */
export function curvePath(
  q: readonly number[],
  max: number,
  box: { x0: number; x1: number; y0: number; y1: number },
  steps = 120,
): string {
  const parts: string[] = [];
  for (let k = 0; k <= steps; k++) {
    const x = (max * k) / steps;
    const px = box.x0 + ((box.x1 - box.x0) * k) / steps;
    const py = box.y1 - (box.y1 - box.y0) * exceedance(q, x);
    parts.push(`${k ? 'L' : 'M'}${px.toFixed(1)} ${py.toFixed(1)}`);
  }
  return parts.join(' ');
}

/** Payback colour bands of the design: within a year, within two, beyond. */
export function paybackTone(months: number): string {
  if (months <= 12) return 'var(--success-text)';
  if (months <= 24) return 'var(--fg-primary)';
  return 'var(--warning-text)';
}
