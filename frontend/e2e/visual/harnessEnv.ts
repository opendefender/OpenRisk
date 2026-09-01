// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Puts a visual harness into a known theme AND a known language.
 *
 * The HTML files already stamp `data-theme` before first paint, which is right —
 * a theme applied after mount is a flash, and in a screenshot suite a flash is a
 * race that looks like a colour regression. But the attribute alone is not
 * enough once the STORE is involved, for two reasons that both bite silently:
 *
 *  1. `uiStore` persists through localStorage. It rehydrates whatever the last
 *     run left behind, so a harness that only reads `?theme` renders in the
 *     previous run's theme as soon as anything touches the store.
 *  2. `setLang` calls the store's `applyDom`, which rewrites `data-theme` from
 *     the STORE's value. Setting the language would therefore undo the
 *     attribute the HTML just stamped, and the page would silently render the
 *     persisted theme instead of the requested one.
 *
 * So the store is the source of truth here, and both axes are pushed through it
 * before render. `setLang` stamps `lang` on <html>, which is what the spec
 * asserts — without that assertion a harness that ignored the parameter would
 * produce two identical snapshots and pass while testing one language twice.
 */

import { useUIStore, type Lang, type Theme } from '../../src/store/uiStore';

export interface HarnessEnv {
  theme: Theme;
  lang: Lang;
}

/** Reads `?theme` / `?lang`, defaulting to the values the HTML stamps. */
export function applyHarnessEnv(): HarnessEnv {
  const q = new URLSearchParams(location.search);

  /* Normalised rather than cast: an unexpected `?lang=de` must land on a real
     language, not on `undefined` and a dictionary lookup that returns the key
     name for every string on the page. */
  const theme: Theme = q.get('theme') === 'light' ? 'light' : 'dark';
  const lang: Lang = q.get('lang') === 'fr' ? 'fr' : 'en';

  const store = useUIStore.getState();
  store.setTheme(theme);
  store.setLang(lang);

  return { theme, lang };
}
