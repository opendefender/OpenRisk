// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Table state, in the URL.
//
// Why the URL and not component state: a filtered table is a *place*. If the
// combination "P1 + KEV + sorted by CVSS" cannot be pasted into Slack, reloaded,
// or reached with the back button, then the filter panel is a toy. Everything a
// user can express about a table therefore lives in the query string:
//
//   ?q=log4j&sort=cvss:desc&page=2&size=50&f.severity=critical,high&f.kev=true
//
// Unknown params (notably `?focus=<id>` from universal search) are preserved.
// Column layout and saved views are per-user, not per-URL, so they live in
// localStorage keyed by the table id.

import { useCallback, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { EMPTY_TABLE_STATE, type ColumnPrefs, type SavedView, type TableState } from './types';

const FACET_PREFIX = 'f.';

/* ------------------------------------------------------------ URL <-> state */

function parseSort(raw: string | null): TableState['sort'] {
  if (!raw) return null;
  const [key, dir] = raw.split(':');
  if (!key) return null;
  return { key, dir: dir === 'asc' ? 'asc' : 'desc' };
}

function readState(params: URLSearchParams, prefix: string, defaults: Partial<TableState>): TableState {
  const k = (name: string) => `${prefix}${name}`;
  const filters: Record<string, string[]> = {};
  params.forEach((value, name) => {
    if (!name.startsWith(`${prefix}${FACET_PREFIX}`)) return;
    const key = name.slice(prefix.length + FACET_PREFIX.length);
    const values = value.split(',').map((v) => v.trim()).filter(Boolean);
    if (key && values.length) filters[key] = values;
  });

  const size = Number(params.get(k('size')));
  const page = Number(params.get(k('page')));
  return {
    q: params.get(k('q')) ?? '',
    sort: parseSort(params.get(k('sort'))) ?? defaults.sort ?? EMPTY_TABLE_STATE.sort,
    page: Number.isFinite(page) && page > 0 ? page : 1,
    pageSize: Number.isFinite(size) && size > 0 ? size : defaults.pageSize ?? EMPTY_TABLE_STATE.pageSize,
    filters: Object.keys(filters).length ? filters : (defaults.filters ?? {}),
  };
}

function writeState(prev: URLSearchParams, next: TableState, prefix: string, defaults: Partial<TableState>): URLSearchParams {
  const out = new URLSearchParams(prev);
  const k = (name: string) => `${prefix}${name}`;

  // Drop every param this table owns, then re-add only the non-default ones —
  // so a reset filter leaves a clean URL instead of `?f.severity=`.
  Array.from(out.keys()).forEach((name) => {
    if (name.startsWith(`${prefix}${FACET_PREFIX}`)) out.delete(name);
  });
  (['q', 'sort', 'page', 'size'] as const).forEach((name) => out.delete(k(name)));

  if (next.q) out.set(k('q'), next.q);
  if (next.sort) out.set(k('sort'), `${next.sort.key}:${next.sort.dir}`);
  if (next.page > 1) out.set(k('page'), String(next.page));
  if (next.pageSize !== (defaults.pageSize ?? EMPTY_TABLE_STATE.pageSize)) out.set(k('size'), String(next.pageSize));
  Object.entries(next.filters).forEach(([key, values]) => {
    if (values.length) out.set(`${prefix}${FACET_PREFIX}${key}`, values.join(','));
  });
  return out;
}

export interface UseTableStateOptions {
  /** Prefix for the query params when two tables share a route. Default ''. */
  urlPrefix?: string;
  defaultSort?: TableState['sort'];
  defaultPageSize?: number;
  defaultFilters?: Record<string, string[]>;
}

export interface TableStateApi {
  state: TableState;
  setQuery: (q: string) => void;
  setSort: (sort: TableState['sort']) => void;
  toggleSort: (key: string) => void;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;
  /** Toggles one value of a facet (or replaces it, for single-choice facets). */
  toggleFilter: (facet: string, value: string, single?: boolean) => void;
  setFilter: (facet: string, values: string[]) => void;
  clearFilters: () => void;
  /** Applies a saved view (q + filters + sort) and returns to page 1. */
  apply: (partial: Partial<TableState>) => void;
  /** Number of facets currently narrowing the result set. */
  activeFilterCount: number;
}

export function useTableState(options: UseTableStateOptions = {}): TableStateApi {
  const { urlPrefix = '', defaultSort = null, defaultPageSize = 50, defaultFilters } = options;
  const [params, setParams] = useSearchParams();

  const defaults = useMemo<Partial<TableState>>(
    () => ({ sort: defaultSort, pageSize: defaultPageSize, filters: defaultFilters }),
    // Callers build these inline; comparing by value keeps the identity stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [JSON.stringify(defaultSort), defaultPageSize, JSON.stringify(defaultFilters)],
  );

  const state = useMemo(() => readState(params, urlPrefix, defaults), [params, urlPrefix, defaults]);

  const patch = useCallback(
    (mutate: (prev: TableState) => TableState, replace = true) => {
      setParams(
        (prev) => {
          const current = readState(prev, urlPrefix, defaults);
          return writeState(prev, mutate(current), urlPrefix, defaults);
        },
        { replace },
      );
    },
    [setParams, urlPrefix, defaults],
  );

  const setQuery = useCallback((q: string) => patch((s) => ({ ...s, q, page: 1 })), [patch]);
  const setSort = useCallback((sort: TableState['sort']) => patch((s) => ({ ...s, sort, page: 1 })), [patch]);
  const toggleSort = useCallback(
    (key: string) =>
      patch((s) => {
        if (s.sort?.key !== key) return { ...s, sort: { key, dir: 'desc' }, page: 1 };
        if (s.sort.dir === 'desc') return { ...s, sort: { key, dir: 'asc' }, page: 1 };
        return { ...s, sort: null, page: 1 };
      }),
    [patch],
  );
  const setPage = useCallback((page: number) => patch((s) => ({ ...s, page: Math.max(1, page) }), false), [patch]);
  const setPageSize = useCallback((pageSize: number) => patch((s) => ({ ...s, pageSize, page: 1 })), [patch]);

  const setFilter = useCallback(
    (facet: string, values: string[]) =>
      patch((s) => {
        const filters = { ...s.filters };
        if (values.length) filters[facet] = values;
        else delete filters[facet];
        return { ...s, filters, page: 1 };
      }),
    [patch],
  );

  const toggleFilter = useCallback(
    (facet: string, value: string, single = false) =>
      patch((s) => {
        const current = s.filters[facet] ?? [];
        const next = single
          ? current.includes(value) ? [] : [value]
          : current.includes(value) ? current.filter((v) => v !== value) : [...current, value];
        const filters = { ...s.filters };
        if (next.length) filters[facet] = next;
        else delete filters[facet];
        return { ...s, filters, page: 1 };
      }),
    [patch],
  );

  const clearFilters = useCallback(() => patch((s) => ({ ...s, filters: {}, q: '', page: 1 })), [patch]);
  const apply = useCallback((partial: Partial<TableState>) => patch((s) => ({ ...s, ...partial, page: 1 })), [patch]);

  const activeFilterCount = useMemo(
    () => Object.values(state.filters).filter((v) => v.length > 0).length,
    [state.filters],
  );

  return { state, setQuery, setSort, toggleSort, setPage, setPageSize, toggleFilter, setFilter, clearFilters, apply, activeFilterCount };
}

/* -------------------------------------------------------- local persistence */

function readJson<T>(key: string, fallback: T): T {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

function writeJson(key: string, value: unknown): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* quota / private mode — the table still works, it just forgets. */
  }
}

/** Per-user saved filter combinations for one table. */
export function useSavedViews(tableId: string) {
  const key = `openrisk.table.${tableId}.views`;
  const [views, setViews] = useState<SavedView[]>(() => readJson<SavedView[]>(key, []));

  const save = useCallback(
    (name: string, state: SavedView['state']) => {
      setViews((prev) => {
        const next = [...prev.filter((v) => v.name !== name), { id: `${Date.now()}`, name, state }];
        writeJson(key, next);
        return next;
      });
    },
    [key],
  );

  const remove = useCallback(
    (id: string) => {
      setViews((prev) => {
        const next = prev.filter((v) => v.id !== id);
        writeJson(key, next);
        return next;
      });
    },
    [key],
  );

  return { views, save, remove };
}

/**
 * Per-user column order + visibility for one table.
 *
 * Stored prefs are reconciled against the live column set *during render*
 * rather than in an effect: a column added by a later release then simply
 * appears (appended, visible), and one removed simply drops out, with no
 * cascading re-render and no stale write-back.
 */
export function useColumnPrefs(tableId: string, allKeys: string[], defaultHidden: string[]) {
  const key = `openrisk.table.${tableId}.columns`;
  const [prefs, setPrefs] = useState<ColumnPrefs>(() =>
    readJson<ColumnPrefs>(key, { order: allKeys, hidden: defaultHidden }),
  );

  const update = useCallback(
    (next: ColumnPrefs) => {
      setPrefs(next);
      writeJson(key, next);
    },
    [key],
  );

  const order = useMemo(() => {
    const known = prefs.order.filter((k) => allKeys.includes(k));
    return [...known, ...allKeys.filter((k) => !known.includes(k))];
  }, [prefs.order, allKeys]);

  const hidden = useMemo(() => prefs.hidden.filter((k) => allKeys.includes(k)), [prefs.hidden, allKeys]);

  const toggle = useCallback(
    (columnKey: string) =>
      update({
        order,
        hidden: hidden.includes(columnKey) ? hidden.filter((k) => k !== columnKey) : [...hidden, columnKey],
      }),
    [order, hidden, update],
  );

  const move = useCallback(
    (columnKey: string, delta: -1 | 1) => {
      const idx = order.indexOf(columnKey);
      const target = idx + delta;
      if (idx < 0 || target < 0 || target >= order.length) return;
      const next = [...order];
      next.splice(idx, 1);
      next.splice(target, 0, columnKey);
      update({ order: next, hidden });
    },
    [order, hidden, update],
  );

  const reset = useCallback(() => update({ order: allKeys, hidden: defaultHidden }), [allKeys, defaultHidden, update]);

  return { order, hidden, toggle, move, reset };
}
