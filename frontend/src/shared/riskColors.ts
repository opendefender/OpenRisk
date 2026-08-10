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

/** Framework badge colors (dc.html §11). */
export const frameworkColor: Record<string, string> = {
  ISO27001: '#7c6cff',
  COBAC: '#30d158',
  BCEAO: '#ff9f0a',
  NIST: '#0a84ff',
  DORA: '#ff2d92',
  SOC2: '#64d2ff',
  ANSSI: '#ff453a',
};

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
