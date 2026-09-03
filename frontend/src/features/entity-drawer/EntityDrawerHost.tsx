// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Mounts the universal drawer once, for the whole app (W1-02 §6-§9, §33).
//
// It lives in the shell rather than in each page, and that is what makes the
// drawer a navigation primitive instead of a per-screen feature:
//
//   - a deep link works on ANY route, because the host reads the URL and not a
//     page's local state;
//   - a page does not have to opt in, wire a query, or hold a selected id;
//   - opening a related entity from a risk drawer while standing on /assets
//     works, because nothing about the drawer is bound to the list behind it.
//
// The page keeps its own state — its filters, sort, page number, tab — because
// the drawer only ever adds and removes three reserved query parameters and
// never touches the rest.

import { lazy, Suspense, useEffect, useRef } from 'react';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useDrawerController } from './drawerState';
import type { EntitySection } from './types';

// Code-split: the drawer, its four sections and their queries are dead weight
// for a session that never opens one.
const EntityDrawer = lazy(() =>
  import('./EntityDrawer').then((m) => ({ default: m.EntityDrawer })),
);

export function EntityDrawerHost() {
  const { state, open, close, setTab } = useDrawerController();
  const tenantId = useAuthStore((s) => s.user?.tenant_id);

  // Close the drawer when the identity behind it changes (§33).
  //
  // clearSessionScope() already drops the whole query cache on login and
  // logout, so no tenant's data can be SERVED to another. This closes the
  // remaining gap: the drawer's state is in the URL, and a URL survives a
  // soft navigation. Without this, switching identity while a drawer is open
  // leaves `?drawer=risk&entity=<A's id>` in the address bar, and the new
  // session immediately re-requests A's record. The server refuses it — the
  // result is a 404, not a leak — but presenting the previous tenant's record
  // id back to a different user and asking the server about it is not something
  // the drawer should do at all.
  //
  // The first render is not a change: `previous` starts as the current tenant,
  // so a deep link opened in a fresh tab is not closed on arrival.
  const previousTenant = useRef(tenantId);
  useEffect(() => {
    if (previousTenant.current !== tenantId) {
      previousTenant.current = tenantId;
      if (state) close();
    }
  }, [tenantId, state, close]);

  if (!state) return null;

  return (
    <Suspense fallback={null}>
      <EntityDrawer
        // Keyed by the entity so that opening a RELATED record remounts rather
        // than reusing the previous one's component state. Without the key, the
        // new entity would briefly render inside the old one's tab selection and
        // scroll position, which reads as the drawer failing to update.
        key={`${state.type}:${state.id}`}
        type={state.type}
        id={state.id}
        tab={state.tab ?? 'summary'}
        onTabChange={(tab: EntitySection) => setTab(tab)}
        onClose={close}
        onOpenEntity={open}
      />
    </Suspense>
  );
}
