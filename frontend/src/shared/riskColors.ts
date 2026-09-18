// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Small color helpers mapping risk semantics to the design-token CSS variables
// (OpenRisk.dc.html §11). Returning `var(--…)` keeps everything theme-aware.

export type Criticality = 'critical' | 'high' | 'medium' | 'low';

export const critColor: Record<Criticality, string> = {
  critical: 'var(--critical)',
  high: 'var(--high)',
  medium: 'var(--medium)',
  low: 'var(--low)',
};

/**
 * Framework identity colours.
 *
 * A framework is a CATEGORY, not a verdict, so it takes a chart series token
 * and never a semantic one — BCEAO drawn in --danger would read as an alert.
 * Series tokens are re-tuned per theme, which the hex literals that used to
 * live here were not: #ff9f0a on the light canvas measured under 2:1.
 *
 * This is the only copy. complianceOverview.ts and FwBadge used to carry their
 * own, and they disagreed (BCEAO was green on one screen, orange on the next).
 */
export const frameworkColor: Record<string, string> = {
  ISO27001: 'var(--chart-2)',
  COBAC: 'var(--chart-3)',
  BCEAO: 'var(--chart-4)',
  NIST: 'var(--chart-1)',
  DORA: 'var(--chart-7)',
  SOC2: 'var(--chart-6)',
  ANSSI: 'var(--chart-5)',
  ANTIC: 'var(--chart-8)',
};

/** Fallback series for a framework with no assigned colour, in chart order. */
export const SERIES = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
  'var(--chart-6)',
  'var(--chart-7)',
  'var(--chart-8)',
];

// scoreColor(number) and scoreToCriticality(number) USED TO LIVE HERE.
//
// They were two of the four incompatible number→band mappings shipping at once
// (≥7/≥4/≥2 here, ≥15/≥8/≥4 in RiskRegisterPage, ≥15/≥9/≥5 in the shipped audit
// badges, ≥40 in MitigationCard), each calibrated for a different scale than the
// number displayed beside it. They are deleted, not fixed: as long as the client
// CAN derive a band from a number, some screen eventually will, with its own
// thresholds.
//
// The band now arrives from the server with the value it describes. Map a BAND to
// a colour with bandColor() in services/scoreService.ts — it takes a band, never
// a number, so it cannot disagree about where a boundary lies.
//
// See docs/scoring/SCORE_MODEL.md.

/** A translucent fill of a token color (works with var(--…) or hex). */
export function softFill(color: string, pct = 14): string {
  return `color-mix(in srgb, ${color} ${pct}%, transparent)`;
}
