// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * The chart layer, deliberately NOT re-exported from `shared/ds/index.ts`.
 *
 * The ds barrel is imported by code on the preloaded path, so anything it
 * re-exports is reachable from the entry graph. visx is ~40 KB gzipped that
 * belongs in the lazy `charts` chunk, and a barrel re-export is exactly how it
 * would quietly stop being lazy — the failure `check-bundle-budget.mjs` exists
 * to catch, and which D-024 obliges this work not to cause.
 *
 * Import from `shared/ds/charts` directly, only from lazily-routed screens.
 */

export {
  CartesianChart,
  type CartesianChartProps,
  type ChartSeries,
  type SeriesType,
} from './CartesianChart';

export { PieChart, type PieChartProps, type PieSlice, type PieSlices } from './PieChart';
export {
  RadarChart,
  type RadarChartProps,
  type RadarAxis,
  type RadarSeries,
} from './RadarChart';
