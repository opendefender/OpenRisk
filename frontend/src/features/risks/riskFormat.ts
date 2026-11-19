// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// ONE display format for a risk's figures, used by the register table, the
// matrix, the drawer, the new-risk modal and the CSV export (#913, D-069 A).
//
// The score is the server's `score`: Probability (0–1) × Impact (0–10) ×
// AssetCriticality (0.1–3). Nothing here multiplies, bins or compares: the band
// arrives from the server beside the number, and this file only prints.

import { formatNumber } from '../../i18n/format';
import type { LocaleCode } from '../../i18n/locales';

const EMPTY = '—';

const digits = (n: number): Intl.NumberFormatOptions => ({
  minimumFractionDigits: n,
  maximumFractionDigits: n,
});

const isNum = (v: number | null | undefined): v is number =>
  typeof v === 'number' && Number.isFinite(v);

/** A score, one decimal: "7,2" (fr) / "7.2" (en). Never "/25", never "/100". */
export function formatScore(locale: LocaleCode, score: number | null | undefined): string {
  return isNum(score) ? formatNumber(locale, score, digits(1)) : EMPTY;
}

/** A probability, two decimals: "0,62". */
export function formatProbability(locale: LocaleCode, p: number | null | undefined): string {
  return isNum(p) ? formatNumber(locale, p, digits(2)) : EMPTY;
}

/** An impact, one decimal: "8,0". */
export function formatImpact(locale: LocaleCode, i: number | null | undefined): string {
  return isNum(i) ? formatNumber(locale, i, digits(1)) : EMPTY;
}

/** "P 0,62 · I 8,0" — the two inputs that sit beside a score. */
export function formatPI(
  locale: LocaleCode,
  p: number | null | undefined,
  i: number | null | undefined,
): string {
  return `P ${formatProbability(locale, p)} · I ${formatImpact(locale, i)}`;
}
