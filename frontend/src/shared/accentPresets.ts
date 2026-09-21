// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The accent variants a person or an organization may choose (#718). The single
// list both pickers render and the organization form validates against. Each
// key must have its `:root[data-variant=…]` rules for both themes in
// styles/tokens.css — contrast-checked by scripts/check-contrast.mjs — and the
// backend's domain.AccentPresets is held equal to those rules by a Go test.

import type { Variant } from '../store/uiStore';

export const ACCENT_PRESETS = ['azure', 'iris'] as const satisfies readonly Variant[];

export type AccentPreset = (typeof ACCENT_PRESETS)[number];

/** Compile-time guard: a Variant added to the store without a preset fails here. */
type Missing = Exclude<Variant, AccentPreset>;
const exhaustive: Missing extends never ? true : never = true;
void exhaustive;

export const ACCENT_LABELS: Record<AccentPreset, string> = {
  azure: 'Azure',
  iris: 'Iris',
};

export function isAccentPreset(value: unknown): value is AccentPreset {
  return typeof value === 'string' && (ACCENT_PRESETS as readonly string[]).includes(value);
}
