// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The drawer's state model (W1-02 §6-§9).
//
// THE URL IS THE STATE. Not a mirror of it, not a side effect of it — the only
// copy. Every previous drawer in this codebase (risk, incident, control,
// evidence, asset history, CVE, vulnerability, mitigation — eight of them) held
// the selected entity in a local useState, and every one of them therefore
// could not be shared, did not survive a refresh, and did not close on Back.
//
// The shape:
//
//   /risks?severity=critical&page=3            ← the page's own state, untouched
//   /risks?severity=critical&page=3&drawer=risk&entity=42&etab=timeline
//
// Three reserved parameters, prefixed so they cannot collide with a page's own
// filters (a register already uses `q`, `sort`, `page`, `size`, `f.*`):
//
//   drawer  — the entity type
//   entity  — the entity id
//   etab    — the open section (omitted when it is the default)
//
// Everything else in the query string is left exactly as it was found, which is
// what §9 means by preserving page context: opening and closing a drawer must
// return the user to the same filtered, sorted, paginated view they were
// reading, not to the top of an unfiltered list.

import { useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router';
import { isEntityType, type EntitySection, type EntityType } from './types';

export const DRAWER_PARAM = 'drawer';
export const ENTITY_PARAM = 'entity';
export const TAB_PARAM = 'etab';

/** The reserved keys, in one place, so `stripDrawer` and the tests agree. */
export const DRAWER_PARAMS = [DRAWER_PARAM, ENTITY_PARAM, TAB_PARAM] as const;

export interface DrawerState {
  type: EntityType;
  id: string;
  tab?: EntitySection;
}

const SECTIONS: readonly EntitySection[] = ['summary', 'relations', 'timeline', 'audit'];

function parseSection(v: string | null): EntitySection | undefined {
  return v && (SECTIONS as readonly string[]).includes(v) ? (v as EntitySection) : undefined;
}

/**
 * Read the drawer out of a query string.
 *
 * Returns null unless BOTH the type and the id are present and the type is one
 * the client knows. A half-written link (`?drawer=risk` with no entity) opens
 * nothing rather than opening an empty drawer — and an unknown type is ignored
 * here as well as refused by the server, so a typo in a shared URL degrades to
 * the plain page instead of an error screen.
 */
export function readDrawer(search: URLSearchParams | string): DrawerState | null {
  const params = typeof search === 'string' ? new URLSearchParams(search) : search;
  const type = params.get(DRAWER_PARAM);
  const id = params.get(ENTITY_PARAM);
  if (!isEntityType(type) || !id) return null;
  return { type, id, tab: parseSection(params.get(TAB_PARAM)) };
}

/**
 * Write a drawer into a query string, preserving everything already there.
 *
 * Returns a NEW URLSearchParams; the input is never mutated, because callers
 * hold the router's live params object.
 */
export function writeDrawer(search: URLSearchParams, state: DrawerState): URLSearchParams {
  const next = new URLSearchParams(search);
  next.set(DRAWER_PARAM, state.type);
  next.set(ENTITY_PARAM, state.id);
  // The default tab is not written: a link to an entity should be the short,
  // obvious one, and `&etab=summary` in every shared URL is noise that also
  // makes two links to the same thing look different.
  if (state.tab && state.tab !== 'summary') next.set(TAB_PARAM, state.tab);
  else next.delete(TAB_PARAM);
  return next;
}

/** Remove only the drawer's own parameters. The page's filters, sort, page
 *  number and tab survive untouched — that is the whole point (§9). */
export function stripDrawer(search: URLSearchParams): URLSearchParams {
  const next = new URLSearchParams(search);
  for (const key of DRAWER_PARAMS) next.delete(key);
  return next;
}

/**
 * Build a shareable drawer link for a path, preserving that path's own query.
 * Used by relation chips and timeline rows, which know a target's type and id
 * but not where its list lives — the server hands them a `url` built from the
 * same shape (entity.DeepLink), and this is its client-side twin for links the
 * client composes itself.
 */
export function drawerHref(listPath: string, type: EntityType, id: string, tab?: EntitySection): string {
  const [path, query = ''] = listPath.split('?');
  const params = writeDrawer(new URLSearchParams(query), { type, id, tab });
  return `${path}?${params.toString()}`;
}

export interface DrawerController {
  /** The open drawer, or null. */
  state: DrawerState | null;
  /** Open an entity. Pushes a history entry so Back closes it (§8). */
  open: (type: EntityType, id: string, tab?: EntitySection) => void;
  /** Close. Restores the URL to the page's own state, with no drawer keys. */
  close: () => void;
  /**
   * Switch tab. REPLACES rather than pushes: a user flicking between four tabs
   * of one entity should not have to press Back four times to get out of the
   * drawer. Opening a *different entity* does push — that is a navigation.
   */
  setTab: (tab: EntitySection) => void;
}

/**
 * The drawer controller, bound to the router.
 *
 * `open` pushes and `close` pops back to the page. That asymmetry is deliberate
 * and is what §8 asks for: entity A → entity B → Back lands on A, and one more
 * Back lands on the page the user started from, with their filters intact.
 */
export function useDrawerController(): DrawerController {
  const [params, setParams] = useSearchParams();

  const state = useMemo(() => readDrawer(params), [params]);

  const open = useCallback(
    (type: EntityType, id: string, tab?: EntitySection) => {
      setParams((prev) => writeDrawer(prev, { type, id, tab }), { replace: false });
    },
    [setParams]
  );

  const close = useCallback(() => {
    setParams((prev) => stripDrawer(prev), { replace: false });
  }, [setParams]);

  const setTab = useCallback(
    (tab: EntitySection) => {
      setParams(
        (prev) => {
          const current = readDrawer(prev);
          if (!current) return prev;
          return writeDrawer(prev, { ...current, tab });
        },
        { replace: true }
      );
    },
    [setParams]
  );

  return { state, open, close, setTab };
}
