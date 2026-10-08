// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Money display shared by the redesigned dashboard, executive and financial
// screens (#901): amounts read in millions of the tenant's display currency,
// one decimal, in the reader's number conventions ("117,2 M FCFA").

import { formatNumber } from '../../i18n/format';
import type { LocaleCode } from '../../i18n/locales';

/** The label a currency code is shown with. The CFA franc reads as FCFA. */
export function currencyLabel(code: string | undefined): string {
  if (!code || code === 'XAF') return 'FCFA';
  return code;
}

/** Converts a canonical XAF figure into the display currency. */
export function toDisplay(xaf: number, fxRateXaf: number | undefined): number {
  const rate = fxRateXaf && fxRateXaf > 0 ? fxRateXaf : 1;
  return xaf / rate;
}

/** "117,2" for 117 200 000 — the figure the design prints before "M FCFA". */
export function millions(value: number, locale: LocaleCode, digits = 1): string {
  return formatNumber(locale, value / 1e6, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}
