// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useCallback } from 'react';
import frLocale from '../locales/fr.json';
import enLocale from '../locales/en.json';

type Locale = 'fr' | 'en';

interface UseI18nReturn {
  t: (key: string, defaultValue?: string) => string;
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

let currentLocale: Locale = (localStorage.getItem('locale') as Locale) || 'en';

const locales: Record<Locale, Record<string, any>> = {
  fr: frLocale,
  en: enLocale,
};

/**
 * Simple i18n hook for translations
 * Supports nested keys like "risks.title"
 */
export function useI18n(): UseI18nReturn {
  const getNestedValue = useCallback((obj: any, path: string): string => {
    const keys = path.split('.');
    let value = obj;

    for (const key of keys) {
      value = value?.[key];
      if (value === undefined) return path; // Return key if not found
    }

    return value ?? path;
  }, []);

  const t = useCallback(
    (key: string, defaultValue?: string): string => {
      const locale = locales[currentLocale];
      const translation = getNestedValue(locale, key);
      return translation || defaultValue || key;
    },
    [getNestedValue]
  );

  const setLocale = useCallback((locale: Locale) => {
    currentLocale = locale;
    localStorage.setItem('locale', locale);
    // Trigger a re-render by dispatching a custom event
    window.dispatchEvent(new CustomEvent('locale-change', { detail: { locale } }));
  }, []);

  return { t, locale: currentLocale, setLocale };
}

/**
 * Helper function to interpolate values in translation strings
 * Usage: interpolate(t('risks.selectedCount'), { count: 5 })
 */
export function interpolate(str: string, values: Record<string, any>): string {
  return str.replace(/{(\w+)}/g, (_, key) => values[key] ?? `{${key}}`);
}
