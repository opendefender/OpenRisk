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

/** Score → color threshold (Score Engine: ≥7 critical · ≥4 high · ≥2 medium · <2 low). */
export function scoreColor(s: number): string {
  return s >= 7 ? 'var(--critical)' : s >= 4 ? 'var(--high)' : s >= 2 ? 'var(--medium)' : 'var(--low)';
}

/** Numeric score → criticality bucket. */
export function scoreToCriticality(s: number): Criticality {
  return s >= 7 ? 'critical' : s >= 4 ? 'high' : s >= 2 ? 'medium' : 'low';
}

/** A translucent fill of a token color (works with var(--…) or hex). */
export function softFill(color: string, pct = 14): string {
  return `color-mix(in srgb, ${color} ${pct}%, transparent)`;
}
