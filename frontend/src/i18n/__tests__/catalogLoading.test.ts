// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #966 — only the default language ships in the entry bundle; every other one
// is fetched before it becomes the active language. The shared test setup
// loads every catalogue up front, so each test here takes a fresh copy of the
// modules to see the state a cold page load starts from.

import { describe, it, expect, vi, beforeEach } from 'vitest';

async function freshCatalog() {
  vi.resetModules();
  const catalog = await import('../catalog');
  const { translate } = await import('../translate');
  return { ...catalog, translate };
}

async function freshStore() {
  vi.resetModules();
  const { useUIStore } = await import('../../store/uiStore');
  const { isCatalogLoaded } = await import('../catalog');
  return { useUIStore, isCatalogLoaded };
}

describe('lazy language catalogues (#966)', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('has the default language complete from module load and English still to fetch', async () => {
    const { catalogs, isCatalogLoaded, translate } = await freshCatalog();
    expect(isCatalogLoaded('fr')).toBe(true);
    expect(isCatalogLoaded('en')).toBe(false);
    expect(translate(catalogs, 'fr', 'common.cancel')).toBe('Annuler');
    // Before the fetch an English lookup falls back to French, never to blank.
    expect(translate(catalogs, 'en', 'common.cancel')).toBe('Annuler');
  });

  it('merges the fetched JSON under the TypeScript overlay', async () => {
    const { catalogs, loadCatalog, isCatalogLoaded, translate } = await freshCatalog();
    // The overlay is present before the fetch: plurals are not lazy.
    expect(translate(catalogs, 'en', 'common.results', { params: { count: 2 } })).toBe('2 results');
    await loadCatalog('en');
    expect(isCatalogLoaded('en')).toBe(true);
    expect(translate(catalogs, 'en', 'common.cancel')).toBe('Cancel');
    expect(translate(catalogs, 'en', 'common.results', { params: { count: 2 } })).toBe('2 results');
  });

  it('fetches a language once however many callers ask at the same time', async () => {
    const { loadCatalog } = await freshCatalog();
    const first = loadCatalog('en');
    expect(loadCatalog('en')).toBe(first);
    await first;
  });

  it('resolves at once for a language with nothing to fetch', async () => {
    const { loadCatalog, isCatalogLoaded } = await freshCatalog();
    // `ar` is registered with no catalogue at all; `fr` is in the bundle.
    expect(isCatalogLoaded('ar')).toBe(true);
    await expect(loadCatalog('ar')).resolves.toBeUndefined();
    await expect(loadCatalog('fr')).resolves.toBeUndefined();
  });
});

describe('switching to a lazy language (#966)', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('switches only once the catalogue has arrived', async () => {
    const { useUIStore, isCatalogLoaded } = await freshStore();
    expect(useUIStore.getState().lang).toBe('fr');

    useUIStore.getState().setLang('en');
    // Not yet: the screen must not flip to a language it cannot print.
    expect(useUIStore.getState().lang).toBe('fr');

    await vi.waitFor(() => expect(useUIStore.getState().lang).toBe('en'));
    expect(isCatalogLoaded('en')).toBe(true);
    expect(document.documentElement.getAttribute('lang')).toBe('en');
    expect(localStorage.getItem('locale')).toBe('en');
  });

  it('lets a later choice win over a slow earlier one', async () => {
    const { useUIStore, isCatalogLoaded } = await freshStore();
    useUIStore.getState().setLang('en');
    // French is loaded, so this one applies at once…
    useUIStore.getState().setLang('fr');
    expect(useUIStore.getState().lang).toBe('fr');

    // …and the English fetch landing afterwards must not undo it.
    await vi.waitFor(() => expect(isCatalogLoaded('en')).toBe(true));
    await Promise.resolve();
    expect(useUIStore.getState().lang).toBe('fr');
  });
});
