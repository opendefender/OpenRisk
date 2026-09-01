// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The visualisation contract: one palette and one set of chart furniture for
 * every chart in the product, and for the topology graph.
 *
 * The values are `var(--chart-N)` strings rather than resolved hex. SVG
 * attributes accept CSS variables, so a chart written against these follows the
 * theme with no re-render and no JS reading computed styles — the same property
 * that makes the token layer work for the rest of the UI, applied to the half
 * of the interface that usually gets left out of theming.
 *
 * The two rules that matter:
 *
 *   1. A CATEGORY takes a colour from `categorical`, in order. Categories carry
 *      no verdict, so they must not borrow the semantic palette: a bar chart of
 *      assets per environment coloured red/amber/green reads as an alert about
 *      environments.
 *
 *   2. A SEVERITY takes its colour from `severity`. This one IS the risk scale,
 *      deliberately, so a "critical" bar is the same red as a "critical" badge
 *      three rows above it.
 *
 * Neither is enough on its own for accessibility: colour must never be the only
 * encoding (WCAG 1.4.1). Every chart needs a legend, a label or a pattern, and
 * `chartAccessibleProps` covers the case where the chart is decorative next to
 * a table that already states the numbers.
 */

/** Categorical series, in assignment order. */
export const categorical = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
  'var(--chart-7)',
  'var(--chart-8)',
] as const;

/**
 * Colour for series index `i`. Wraps rather than running out — a chart with
 * nine series has a bigger problem than colour, and returning undefined would
 * make it render invisible.
 */
export function seriesColor(index: number): string {
  return categorical[index % categorical.length];
}

/** Severity encodings. These ARE the risk scale from tokens.css. */
export const severity = {
  critical: 'var(--risk-critical)',
  high: 'var(--risk-high)',
  medium: 'var(--risk-moderate)',
  low: 'var(--risk-low)',
  extreme: 'var(--risk-extreme)',
} as const;

export type SeverityKey = keyof typeof severity;

/** Direction-of-travel encodings, for deltas and trends. */
export const trend = {
  positive: 'var(--success)',
  negative: 'var(--danger)',
  neutral: 'var(--fg-muted)',
  /** The bar/area behind a value: a track, not data. */
  track: 'var(--chart-track)',
} as const;

/** Axes, grid and labels. Spread onto the Recharts primitives. */
export const chartAxis = {
  stroke: 'var(--chart-axis)',
  tick: { fill: 'var(--chart-label)', fontSize: 11 },
  tickLine: false,
  axisLine: { stroke: 'var(--chart-axis)' },
} as const;

export const chartGrid = {
  stroke: 'var(--chart-grid)',
  /* Horizontal only. Vertical gridlines on a time series add ink without
     adding readings — the category labels already mark the columns. */
  vertical: false,
} as const;

/**
 * Tooltip chrome. Recharts styles its tooltip with inline objects, so this is
 * a style object rather than a class — which also means it themes correctly
 * even though the tooltip renders outside the chart's DOM subtree.
 */
export const chartTooltip = {
  contentStyle: {
    background: 'var(--surface-2)',
    border: '1px solid var(--border-default)',
    borderRadius: 'var(--radius-sm)',
    boxShadow: 'var(--elev-2)',
    fontSize: 'var(--text-xs)',
    color: 'var(--fg-primary)',
    padding: '8px 10px',
  },
  labelStyle: { color: 'var(--fg-secondary)', marginBottom: 4 },
  itemStyle: { color: 'var(--fg-primary)' },
  cursor: { fill: 'var(--surface-3)' },
} as const;

/** Topology graph. */
export const graph = {
  node: 'var(--graph-node)',
  nodeStroke: 'var(--graph-node-stroke)',
  edge: 'var(--graph-edge)',
  edgeActive: 'var(--graph-edge-active)',
  /** Opacity for nodes outside the current selection or filter. Dimming rather
   *  than hiding keeps the shape of the graph legible while the focus changes. */
  dimmed: 'var(--graph-dimmed)',
} as const;

/**
 * Props for the chart's container.
 *
 * A chart is an image to a screen reader — an SVG full of unlabelled paths. It
 * needs either its own description, or to be marked decorative when the numbers
 * are already available as text nearby, which is the common case in this
 * product (most charts sit next to the table they summarise).
 */
export function chartAccessibleProps(
  description: string | { decorativeBecauseTableFollows: true },
): { role: 'img'; 'aria-label': string } | { role: 'presentation'; 'aria-hidden': true } {
  if (typeof description === 'string') {
    return { role: 'img', 'aria-label': description };
  }
  return { role: 'presentation', 'aria-hidden': true };
}
