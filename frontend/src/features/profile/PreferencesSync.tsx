// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Applies the signed-in person's server-side preferences to the interface (#719).
//
// Theme and language are applied when the SERVER value changes — at sign-in, and
// right after they are edited in the profile — not on every refetch, so a quick
// toggle in the header is not snapped back behind the person's back. Date
// preferences are copied on every change: nothing else writes them.
//
// Renders nothing; mounted once inside the authenticated layout.

import { useEffect, useRef } from 'react';

import { ENABLED_LOCALES, isLocaleCode } from '../../i18n/locales';
import { useUIStore } from '../../store/uiStore';
import { usePreferenceStore, toDatePattern } from '../../shared/preferences/preferenceStore';
import { useMyProfile } from './useProfile';

export function PreferencesSync() {
  const { data } = useMyProfile();
  const setThemeMode = useUIStore((s) => s.setThemeMode);
  const setLang = useUIStore((s) => s.setLang);
  const setDatePreferences = usePreferenceStore((s) => s.setDatePreferences);
  const applied = useRef<{ theme?: string; locale?: string }>({});

  useEffect(() => {
    if (!data) return;
    setDatePreferences({
      timeZone: data.effective.timezone || undefined,
      pattern: toDatePattern(data.effective.date_format),
    });

    const theme = data.theme_mode;
    if (theme && theme !== applied.current.theme) {
      setThemeMode(theme);
    }
    applied.current.theme = theme;

    const locale = data.effective.locale;
    if (
      locale &&
      locale !== applied.current.locale &&
      isLocaleCode(locale) &&
      ENABLED_LOCALES.includes(locale)
    ) {
      setLang(locale);
    }
    applied.current.locale = locale;
  }, [data, setDatePreferences, setThemeMode, setLang]);

  return null;
}
