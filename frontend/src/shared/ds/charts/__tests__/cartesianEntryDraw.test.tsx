// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The entry-draw static fallback (#445, D-028).
 *
 * This is the assertion the design guide's *"reduced motion is not 'less
 * motion'"* rule turns on, and the one a screenshot suite would never catch: a
 * chart that hides its marks waiting for a draw which — for a reduced-motion
 * user — is never going to run, and so shows an empty frame forever.
 *
 * `CartesianChart` reveals each series through its own clip rect. The rects are
 * therefore the fallback: collapsed when the animation WILL run, full-size when
 * it will not. Both are asserted here.
 *
 * The sibling suite notes that jsdom measures every element at zero, so
 * `ParentSize` renders nothing and the SVG never appears. That is exactly what
 * makes this file necessary — `ParentSize` is stubbed with real dimensions so
 * there is an SVG to inspect at all.
 */

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';

vi.mock('@visx/responsive', () => ({
  ParentSize: ({ children }: { children: (s: { width: number; height: number }) => unknown }) =>
    children({ width: 600, height: 300 }),
}));

vi.mock('animejs', () => ({
  createScope: () => ({
    add: (cb: () => void) => {
      cb();
      return { revert: vi.fn() };
    },
    revert: vi.fn(),
  }),
  animate: vi.fn(),
  stagger: vi.fn(() => 0),
}));

import { CartesianChart } from '../CartesianChart';

interface Row {
  month: string;
  opened: number;
  trend: number;
}
const DATA: Row[] = [
  { month: 'Jan', opened: 12, trend: 3 },
  { month: 'Feb', opened: 8, trend: 6 },
];

function setReducedMotion(reduce: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: reduce && query.includes('prefers-reduced-motion'),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

/*
 * Selected by `[data-draw]`, NOT by `clipPath rect`. In an HTML document a CSS
 * type selector is lowercased before matching, so `clipPath` never matches the
 * camelCase SVG element — the selector silently returns nothing and every
 * assertion below would pass vacuously against an empty list.
 */
function clipRects(container: HTMLElement): SVGRectElement[] {
  return Array.from(container.querySelectorAll('[data-draw]'));
}

beforeEach(() => setReducedMotion(false));
afterEach(() => setReducedMotion(false));

const CHART = (
  <CartesianChart
    data={DATA}
    x="month"
    series={[
      { type: 'bar', key: 'opened', label: 'Opened' },
      { type: 'line', key: 'trend', label: 'Trend' },
    ]}
    ariaLabel="Risks per month"
  />
);

describe('CartesianChart entry draw', () => {
  it('renders every series at FULL size under prefers-reduced-motion', () => {
    setReducedMotion(true);
    const { container } = render(CHART);
    const rects = clipRects(container);
    expect(rects.length).toBe(2);
    for (const r of rects) {
      // Nothing is clipped away: the chart paints its finished state.
      expect(Number(r.getAttribute('width'))).toBeGreaterThan(0);
      expect(Number(r.getAttribute('height'))).toBeGreaterThan(0);
    }
  });

  it('starts collapsed when the animation WILL run, so there is something to reveal', () => {
    const { container } = render(CHART);
    const rects = clipRects(container);
    expect(rects.length).toBe(2);
    // Bar series grows upward from the baseline: zero height.
    expect(Number(rects[0].getAttribute('height'))).toBe(0);
    // Line series wipes left to right: zero width.
    expect(Number(rects[1].getAttribute('width'))).toBe(0);
  });

  it('gives each series its own clip, so the stagger has something to stagger', () => {
    const { container } = render(CHART);
    const clips = Array.from(container.getElementsByTagName('clipPath'));
    const ids = clips.map((c) => c.getAttribute('id'));
    expect(clips.length).toBe(2);
    expect(new Set(ids).size).toBe(2);
  });
});
