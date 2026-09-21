// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/**
 * Locale-aware formatting — the only place the product turns a number, a date
 * or an amount into text.
 *
 * Before this module there were 77 `toLocale*` call sites, ten of which pinned
 * `'fr-FR'` or `'en-US'` regardless of the language the user had selected, so an
 * English user read French thousands separators. Every formatter here takes the
 * active `LocaleCode` and resolves its BCP-47 tag through the registry, so a new
 * language formats correctly the moment it is registered.
 *
 * `Intl` already knows the market's conventions: XAF prints as "1 234 567 FCFA"
 * in French and "FCFA 1,234,567" in English, with zero decimals in both, because
 * the CFA franc has no minor unit. Do not hand-roll that.
 */

import { localeTag, localeDefinition, type CurrencyCode, type LocaleCode } from './locales';

/**
 * `Intl` constructors are expensive relative to the formatting itself and these
 * run inside table cells, so every configuration is built once and reused.
 */
function memoize<TOptions extends object, TFormatter>(
  build: (tag: string, options: TOptions) => TFormatter,
): (tag: string, options?: TOptions) => TFormatter {
  const cache = new Map<string, TFormatter>();
  return (tag: string, options?: TOptions) => {
    const key = `${tag}|${options ? JSON.stringify(options) : ''}`;
    let found = cache.get(key);
    if (!found) {
      found = build(tag, (options ?? {}) as TOptions);
      cache.set(key, found);
    }
    return found;
  };
}

const numberFormat = memoize<Intl.NumberFormatOptions, Intl.NumberFormat>(
  (tag, options) => new Intl.NumberFormat(tag, options),
);
const dateFormat = memoize<Intl.DateTimeFormatOptions, Intl.DateTimeFormat>(
  (tag, options) => new Intl.DateTimeFormat(tag, options),
);
const relativeFormat = memoize<Intl.RelativeTimeFormatOptions, Intl.RelativeTimeFormat>(
  (tag, options) => new Intl.RelativeTimeFormat(tag, options),
);
const listFormat = memoize<Intl.ListFormatOptions, Intl.ListFormat>(
  (tag, options) => new Intl.ListFormat(tag, options),
);

/* ------------------------------- numbers -------------------------------- */

export function formatNumber(
  locale: LocaleCode,
  value: number,
  options?: Intl.NumberFormatOptions,
): string {
  if (!Number.isFinite(value)) return '—';
  return numberFormat(localeTag(locale), options).format(value);
}

/** "1,2 M" / "1.2M" — for dense tiles where the exact figure is not the point. */
export function formatCompact(locale: LocaleCode, value: number, maximumFractionDigits = 1): string {
  if (!Number.isFinite(value)) return '—';
  return numberFormat(localeTag(locale), {
    notation: 'compact',
    compactDisplay: 'short',
    maximumFractionDigits,
  }).format(value);
}

/**
 * `value` is the ratio, not the percentage: pass 0.42 for 42 %. French puts a
 * non-breaking space before the sign and English does not; `Intl` handles that.
 */
export function formatPercent(locale: LocaleCode, value: number, fractionDigits = 0): string {
  if (!Number.isFinite(value)) return '—';
  return numberFormat(localeTag(locale), {
    style: 'percent',
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(value);
}

/* ------------------------------ currency -------------------------------- */

export interface CurrencyOptions {
  /** ISO 4217. Defaults to the locale's own default currency. */
  currency?: CurrencyCode | string;
  /** Compact notation for dashboards: "1,2 M FCFA". */
  compact?: boolean;
  /** Force decimals. Omitted, the currency's own minor-unit rule applies. */
  fractionDigits?: number;
}

/**
 * Money in the reader's conventions. XAF carries no minor unit, so `Intl` gives
 * it zero decimals and the "FCFA" symbol the target market actually reads —
 * that is why this does not special-case XAF by hand.
 */
export function formatCurrency(
  locale: LocaleCode,
  value: number,
  options: CurrencyOptions = {},
): string {
  if (!Number.isFinite(value)) return '—';
  const currency = options.currency || localeDefinition(locale).defaultCurrency;
  const intlOptions: Intl.NumberFormatOptions = { style: 'currency', currency };
  if (options.compact) {
    intlOptions.notation = 'compact';
    intlOptions.compactDisplay = 'short';
    intlOptions.maximumFractionDigits = options.fractionDigits ?? 1;
  } else if (options.fractionDigits !== undefined) {
    intlOptions.minimumFractionDigits = options.fractionDigits;
    intlOptions.maximumFractionDigits = options.fractionDigits;
  }
  try {
    return numberFormat(localeTag(locale), intlOptions).format(value);
  } catch {
    // An unknown ISO code must not blank a dashboard cell.
    return `${formatNumber(locale, value)} ${currency}`;
  }
}

/* -------------------------------- dates --------------------------------- */

/** Anything the API or a component might hand us as a moment in time. */
export type DateInput = Date | string | number;

function toDate(value: DateInput): Date | null {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDate(
  locale: LocaleCode,
  value: DateInput,
  options: Intl.DateTimeFormatOptions = { dateStyle: 'medium' },
): string {
  const date = toDate(value);
  if (!date) return '—';
  return dateFormat(localeTag(locale), options).format(date);
}

export function formatDateTime(
  locale: LocaleCode,
  value: DateInput,
  options: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' },
): string {
  return formatDate(locale, value, options);
}

export function formatTime(
  locale: LocaleCode,
  value: DateInput,
  options: Intl.DateTimeFormatOptions = { timeStyle: 'short' },
): string {
  return formatDate(locale, value, options);
}

const RELATIVE_UNITS: ReadonlyArray<[Intl.RelativeTimeFormatUnit, number]> = [
  ['year', 31_536_000_000],
  ['month', 2_592_000_000],
  ['week', 604_800_000],
  ['day', 86_400_000],
  ['hour', 3_600_000],
  ['minute', 60_000],
  ['second', 1000],
];

/**
 * "il y a 3 jours" / "3 days ago". `numeric: 'auto'` is deliberate: it lets the
 * language use "hier"/"yesterday" where it has a word for it.
 */
export function formatRelativeTime(
  locale: LocaleCode,
  value: DateInput,
  now: DateInput = Date.now(),
): string {
  const date = toDate(value);
  const base = toDate(now);
  if (!date || !base) return '—';
  const diff = date.getTime() - base.getTime();
  const formatter = relativeFormat(localeTag(locale), { numeric: 'auto' });
  for (const [unit, ms] of RELATIVE_UNITS) {
    if (Math.abs(diff) >= ms) return formatter.format(Math.round(diff / ms), unit);
  }
  return formatter.format(0, 'second');
}

/* --------------------------------- lists -------------------------------- */

/** "a, b et c" / "a, b, and c" — never a hand-joined ", " with a hardcoded word. */
export function formatList(
  locale: LocaleCode,
  items: readonly string[],
  type: Intl.ListFormatType = 'conjunction',
): string {
  if (items.length === 0) return '';
  return listFormat(localeTag(locale), { style: 'long', type }).format(items);
}

/** Every formatter bound to one locale — what `useFormat()` hands a component. */
export interface BoundFormatters {
  readonly locale: LocaleCode;
  number: (value: number, options?: Intl.NumberFormatOptions) => string;
  compact: (value: number, maximumFractionDigits?: number) => string;
  percent: (value: number, fractionDigits?: number) => string;
  currency: (value: number, options?: CurrencyOptions) => string;
  date: (value: DateInput, options?: Intl.DateTimeFormatOptions) => string;
  dateTime: (value: DateInput, options?: Intl.DateTimeFormatOptions) => string;
  time: (value: DateInput, options?: Intl.DateTimeFormatOptions) => string;
  relative: (value: DateInput, now?: DateInput) => string;
  list: (items: readonly string[], type?: Intl.ListFormatType) => string;
}

export function boundFormatters(locale: LocaleCode): BoundFormatters {
  return {
    locale,
    number: (value, options) => formatNumber(locale, value, options),
    compact: (value, digits) => formatCompact(locale, value, digits),
    percent: (value, digits) => formatPercent(locale, value, digits),
    currency: (value, options) => formatCurrency(locale, value, options),
    date: (value, options) => formatDate(locale, value, options),
    dateTime: (value, options) => formatDateTime(locale, value, options),
    time: (value, options) => formatTime(locale, value, options),
    relative: (value, now) => formatRelativeTime(locale, value, now),
    list: (items, type) => formatList(locale, items, type),
  };
}

/* ------------------------- preference-aware dates ------------------------ */

/** A person's effective date preferences (#719): own choice, else the org's. */
export type DatePattern = 'DD/MM/YYYY' | 'MM/DD/YYYY' | 'YYYY-MM-DD';

export interface DatePreferences {
  /** IANA zone. Absent = the browser's zone. */
  readonly timeZone?: string;
  /** Absent = the language's own medium style ("17 sept. 2026"). */
  readonly pattern?: DatePattern;
}

/** A zone the running `Intl` does not know is dropped, never thrown. */
function safeZone(timeZone: string | undefined): string | undefined {
  if (!timeZone) return undefined;
  try {
    dateFormat('en-US', { timeZone });
    return timeZone;
  } catch {
    return undefined;
  }
}

/**
 * The one place a date is written the way the person asked: in their time zone
 * and, when they chose one, in their numeric pattern. `withTime` appends the
 * language's short time in the same zone.
 */
export function formatPreferredDate(
  locale: LocaleCode,
  value: DateInput,
  prefs: DatePreferences = {},
  withTime = false,
): string {
  const date = toDate(value);
  if (!date) return '—';
  const timeZone = safeZone(prefs.timeZone);
  const tag = localeTag(locale);
  const zone = timeZone ? { timeZone } : {};

  let day: string;
  switch (prefs.pattern) {
    case 'DD/MM/YYYY':
    case 'MM/DD/YYYY':
    case 'YYYY-MM-DD': {
      const parts = dateFormat(tag, { year: 'numeric', month: '2-digit', day: '2-digit', ...zone })
        .formatToParts(date)
        .reduce<Record<string, string>>((acc, p) => ({ ...acc, [p.type]: p.value }), {});
      const d = parts.day ?? '';
      const m = parts.month ?? '';
      const y = parts.year ?? '';
      day =
        prefs.pattern === 'DD/MM/YYYY'
          ? `${d}/${m}/${y}`
          : prefs.pattern === 'MM/DD/YYYY'
            ? `${m}/${d}/${y}`
            : `${y}-${m}-${d}`;
      break;
    }
    default:
      day = dateFormat(tag, { dateStyle: 'medium', ...zone }).format(date);
  }
  if (!withTime) return day;
  return `${day} ${dateFormat(tag, { timeStyle: 'short', ...zone }).format(date)}`;
}
