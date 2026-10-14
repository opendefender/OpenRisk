// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react';
import {
  boundFormatters,
  catalogs,
  catalogsVersion,
  isCatalogLoaded,
  loadCatalog,
  subscribeCatalogs,
  formatPreferredDate,
  translate,
  type BoundFormatters,
  type DateInput,
  type LocaleCode,
  type TranslateParams,
} from '../i18n';
import { useUIStore } from '../store/uiStore';
import { usePreferenceStore } from '../shared/preferences/preferenceStore';

/**
 * The component-facing i18n hook.
 *
 * The language lives in the central UI store, so the header language toggle
 * re-renders every consumer reactively. Everything below it — key lookup,
 * fallback, pluralization, formatting — is the pure core in `src/i18n`, which is
 * why it is testable without React.
 *
 * The second argument of `t` accepts both shapes on purpose. Roughly 200 call
 * sites already pass a default string, and rewriting them was not the job of
 * this change; passing an object opts into interpolation and plurals:
 *
 *   t('risks.title')
 *   t('risks.title', 'Risk register')                 // legacy default value
 *   t('common.results', { count: rows.length })        // plural + grouping
 */
export interface UseI18nReturn {
  /** Translate a dotted key. Falls back to the default locale, then the key. */
  t: (key: string, defaultOrParams?: string | TranslateParams) => string;
  /** The active language. */
  locale: LocaleCode;
  setLocale: (locale: LocaleCode) => void;
  /** Formatters already bound to the active locale. */
  fmt: BoundFormatters;
}

export type CatalogStatus = 'ready' | 'loading' | 'failed';

/**
 * Makes sure `locale`'s catalogue is loaded and re-renders when it lands (#966).
 *
 * Only the default language ships in the entry bundle. The store's `setLang`
 * already waits for a catalogue before switching, but a screen that picks its
 * own language (the public vendor questionnaire follows the language the
 * organization chose) or a boot whose fetch failed would otherwise translate
 * against a catalogue that is not there and never notice when it arrives.
 * A failed fetch is retried when the browser comes back online.
 */
export function useCatalog(locale: LocaleCode): CatalogStatus {
  useSyncExternalStore(subscribeCatalogs, catalogsVersion, catalogsVersion);
  const ready = isCatalogLoaded(locale);
  const [failedFor, setFailedFor] = useState<LocaleCode | null>(null);

  useEffect(() => {
    if (ready) return;
    let live = true;
    const attempt = () => {
      loadCatalog(locale).then(
        () => undefined,
        () => {
          if (live) setFailedFor(locale);
        },
      );
    };
    attempt();
    window.addEventListener('online', attempt);
    return () => {
      live = false;
      window.removeEventListener('online', attempt);
    };
  }, [locale, ready]);

  if (ready) return 'ready';
  return failedFor === locale ? 'failed' : 'loading';
}

export function useI18n(): UseI18nReturn {
  const locale = useUIStore((s) => s.lang);
  const setLocale = useUIStore((s) => s.setLang);
  // Re-render, and rebuild `t`, when the active catalogue lands.
  const status = useCatalog(locale);

  const t = useCallback(
    (key: string, defaultOrParams?: string | TranslateParams): string => {
      if (typeof defaultOrParams === 'string') {
        return translate(catalogs, locale, key, { defaultValue: defaultOrParams });
      }
      return translate(catalogs, locale, key, { params: defaultOrParams });
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- status is the catalogue's identity
    [locale, status],
  );

  const fmt = useMemo(() => boundFormatters(locale), [locale]);

  return { t, locale, setLocale, fmt };
}

/** Formatters alone, for components that format but do not translate. */
/**
 * Dates written the way the signed-in person asked (#719): their time zone and
 * their numeric pattern, falling back to the language's own style.
 */
export function usePreferredDate(): {
  date: (value: DateInput) => string;
  dateTime: (value: DateInput) => string;
  /** A calendar day with no instant (a due date): pattern applied, no zone
   *  shift, so "2026-10-01" never becomes 30 September west of Greenwich. */
  calendarDate: (value: DateInput) => string;
} {
  const locale = useUIStore((s) => s.lang);
  const timeZone = usePreferenceStore((s) => s.timeZone);
  const pattern = usePreferenceStore((s) => s.pattern);
  return useMemo(
    () => ({
      date: (value: DateInput) => formatPreferredDate(locale, value, { timeZone, pattern }),
      dateTime: (value: DateInput) =>
        formatPreferredDate(locale, value, { timeZone, pattern }, true),
      calendarDate: (value: DateInput) =>
        formatPreferredDate(locale, value, { timeZone: 'UTC', pattern }),
    }),
    [locale, timeZone, pattern],
  );
}

export function useFormat(): BoundFormatters {
  const locale = useUIStore((s) => s.lang);
  return useMemo(() => boundFormatters(locale), [locale]);
}

/**
 * Standalone interpolation, kept for the call sites that read a string first and
 * substitute afterwards (`interpolate(t('actionCenter.range'), { … })`). New
 * code should pass the params straight to `t` so plurals apply too.
 */
export { interpolate as interpolateRaw } from '../i18n/translate';

export function interpolate(str: string, values: Record<string, unknown>): string {
  return str.replace(/\{(\w+)\}/g, (match, key: string) =>
    Object.prototype.hasOwnProperty.call(values, key) ? String(values[key] ?? '') : match,
  );
}
