// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/**
 * Where a language's words are wired in.
 *
 * Adding a language is two lines: an entry in `LOCALES` (src/i18n/locales.ts)
 * and an entry here pointing at its catalogue. `ar` deliberately has no entry —
 * it is registered for its direction and plural rules, and until someone writes
 * `ar.json` it falls back to French through `translate()`. That is the whole
 * point of the fallback chain: a partially translated language degrades to
 * readable text instead of to blank cells.
 *
 * `runtimeMessages` is the extension seam for plural forms and messages that the
 * legacy flat JSON files cannot express. Keys defined there win over the JSON,
 * which lets a screen migrate to real pluralization one key at a time without a
 * big-bang rewrite of `locales/fr.json`.
 *
 * Only the default language's JSON is in the entry bundle (#966). Every other
 * language is a lazy chunk, fetched by `loadCatalog()` before it becomes the
 * active language: main.tsx awaits the stored language before the first render,
 * and `setLang` awaits it before switching. French stays eager because it is
 * both the primary market's language and the fallback every lookup ends on.
 */

import frJson from '../locales/fr.json';
import { runtimeMessages } from './messages';
import { DEFAULT_LOCALE, LOCALE_CODES, type LocaleCode } from './locales';
import { isPluralForms } from './plural';
import type { Catalog } from './translate';

/** A branch of the tree — as opposed to a leaf, which is a string or a plural set. */
function isBranch(value: unknown): value is Catalog {
  return (
    typeof value === 'object' && value !== null && !Array.isArray(value) && !isPluralForms(value)
  );
}

/** Deep-merge two catalogues, with `overlay` winning at every leaf. */
export function merge(base: Catalog, overlay: Catalog): Catalog {
  const out: Catalog = { ...base };
  for (const [key, value] of Object.entries(overlay)) {
    const existing = out[key];
    out[key] =
      isBranch(existing) && isBranch(value) ? merge(existing, value) : (value as Catalog[string]);
  }
  return out;
}

/**
 * The JSON catalogues, keyed by locale. One line per shipped language: the
 * default language is imported, every other one is a loader.
 */
const jsonLoaders: Partial<Record<LocaleCode, () => Promise<Catalog>>> = {
  en: () => import('../locales/en.json').then((m) => m.default as Catalog),
};

/**
 * Every registered locale that has words, with its TypeScript overlay merged
 * over its JSON. The default locale is complete from module load; a lazy one
 * holds only its overlay until `loadCatalog()` merges its JSON in. A registered
 * locale with neither (today: `ar`) is simply absent, and `translate()` falls
 * back for it.
 *
 * The object is filled in place rather than replaced, so every module that
 * imported it sees a catalogue the moment it loads.
 */
export const catalogs: Partial<Record<LocaleCode, Catalog>> = {};
for (const code of LOCALE_CODES) {
  const catalog = mergeFor(code, code === DEFAULT_LOCALE ? (frJson as Catalog) : undefined);
  if (catalog) catalogs[code] = catalog;
}

function mergeFor(code: LocaleCode, json: Catalog | undefined): Catalog | undefined {
  const overlay = runtimeMessages[code];
  if (!json) return overlay;
  return overlay ? merge(json, overlay) : json;
}

const loaded = new Set<LocaleCode>([DEFAULT_LOCALE]);
const inFlight = new Map<LocaleCode, Promise<void>>();

/*
 * Bumped each time a catalogue lands, so React can re-render on it (see
 * useCatalog in hooks/useI18n). The language alone is not enough: when a fetch
 * fails and a retry succeeds, the language has not changed, yet every string
 * on screen has.
 */
let version = 0;
const listeners = new Set<() => void>();

export function subscribeCatalogs(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function catalogsVersion(): number {
  return version;
}

/** True once `code` has every word it will ever have — nothing left to fetch. */
export function isCatalogLoaded(code: LocaleCode): boolean {
  return loaded.has(code) || !jsonLoaders[code];
}

/**
 * Fetch a language's JSON and merge it in. Resolves at once when there is
 * nothing to fetch. A failed fetch (offline on a cold cache) rejects and leaves
 * the language falling back to the default, and a later call retries.
 */
export function loadCatalog(code: LocaleCode): Promise<void> {
  if (isCatalogLoaded(code)) return Promise.resolve();
  const pending = inFlight.get(code);
  if (pending) return pending;
  const load = jsonLoaders[code]!()
    .then((json) => {
      const catalog = mergeFor(code, json);
      if (catalog) catalogs[code] = catalog;
      loaded.add(code);
      version += 1;
      listeners.forEach((listener) => listener());
    })
    .finally(() => inFlight.delete(code));
  inFlight.set(code, load);
  return load;
}
