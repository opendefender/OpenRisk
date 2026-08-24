// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Everything the client holds on behalf of ONE signed-in identity, and the one
// call that drops it.
//
// Why this exists (W0-05 / D9). Sign-out cleared the auth store, the in-memory
// access token and the cached profile — and nothing else. It did not clear the
// TanStack Query cache (staleTime 30s, gcTime 24h), the Zustand data stores, or
// the localStorage keys that hold user-scoped state.
//
// That would be survivable if signing out reloaded the page. It does not: logout
// is `navigate('/login')` and login is `navigate(landing)`, both soft SPA
// navigations. The tab is never torn down, so the cache that user A filled was
// still warm and still authoritative when user B signed in. B's first paint of
// the register, the inventory, the members list and the compliance posture came
// from A's tenant, and for any query still inside its 30-second stale window it
// came from A's tenant WITHOUT a refetch — no flicker, no correction, no clue.
//
// Every other deceptive-UI finding invents data belonging to nobody. This one
// shows one organisation's real data to somebody in another organisation, on
// every screen at once. It is the reason the wave exists.
//
// Called at BOTH ends of the transition:
//
//   - at logout, so nothing outlives the session that fetched it;
//   - at login/adoptSession, because a session does not always end through the
//     logout button. It can end by cookie expiry, by the axios 401 redirect, or
//     by a crash — and in each of those the next sign-in is the first chance to
//     clear. Clearing on the way in makes the guarantee hold regardless of how
//     the previous session ended.
//
// Clearing twice costs one refetch of data the new user is entitled to. Clearing
// once, on the wrong side, costs a cross-tenant disclosure.

import type { QueryClient } from '@tanstack/react-query';

import { realtime } from './realtime';

/**
 * The app's QueryClient, registered by main.tsx at start-up.
 *
 * A module-level handle rather than a hook: this has to run from the auth store,
 * which is plain Zustand and has no React context to read.
 */
let queryClient: QueryClient | null = null;

export function registerQueryClient(client: QueryClient): void {
  queryClient = client;
}

/**
 * Zustand stores that cache tenant data, with the state that a fresh session
 * must start from.
 *
 * Registered rather than imported, so that adding a store cannot create an
 * import cycle back through the auth store, and so a store only ever declares
 * its own reset shape.
 */
type Resettable = { setState: (partial: Record<string, unknown>) => void };
const stores: { store: Resettable; initial: Record<string, unknown>; teardown?: () => void }[] = [];

/**
 * @param teardown Optional: torn down BEFORE the state is reset. A store holding
 *   a live connection (an EventSource opened under the previous session) has to
 *   close it first, or the old stream keeps pushing into the cleared store.
 */
export function registerTenantStore(
  store: Resettable,
  initial: Record<string, unknown>,
  teardown?: () => void,
): void {
  stores.push({ store, initial, teardown });
}

/**
 * localStorage keys scoped to one signed-in person.
 *
 * NOT listed here, deliberately: `openrisk-ui` (theme, accent, language),
 * `openrisk_tour_seen_v1` and `locale`. Those are device preferences, not tenant
 * data — wiping them would relight the product tour and flip the theme for
 * whoever signs in next, which is a worse experience with no security gain.
 */
const USER_SCOPED_KEYS = [
  'auth_user',
  // Pre-cookie era. Removed on every transition so an upgraded tab cannot leave
  // a readable token behind.
  'auth_token',
  'auth_refresh_token',
  'auth_expires_in',
  // Whether THIS person has finished / dismissed the activation checklist.
  'openrisk-onboarding',
  // Per-user notification and policy switches. Now server-backed (W0-05 / D2);
  // the key is still dropped so an upgraded tab does not keep answering from
  // the previous user's local copy.
  'openrisk-settings-prefs',
  // Last-used view mode per register. Harmless on its own, but it is per-person
  // state and there is no reason for it to cross an identity boundary.
  'riskView',
  'assetView',
  'incidentView',
  'mitigation_view_mode',
];

/**
 * Prefixes whose every key is user-scoped: saved table filters and column
 * layouts, written as `openrisk.table.<tableId>.views` / `.columns`. A saved
 * view embeds tenant vocabulary in its values — framework names, owner names,
 * asset tags — so it is tenant data, not just a layout preference.
 */
const USER_SCOPED_PREFIXES = ['openrisk.table.'];

/**
 * Drops every trace of the currently-scoped identity.
 *
 * Safe to call when nothing is scoped (first load, repeated logout): each step
 * is idempotent.
 */
export function clearSessionScope(): void {
  // 0. The realtime stream. It is torn down FIRST, before the caches it feeds:
  //    a stream left open across a session boundary would keep delivering the
  //    previous identity's events into the new one's cache, and its replay
  //    cursor is a position in the previous tenant's sequence — a number that
  //    means something different in the next tenant's log. Reconnecting is the
  //    new session's business, not this function's.
  realtime.switchTenant();

  // 1. Server responses. `clear()` removes cached data AND in-flight queries, so
  //    a request issued by the previous session cannot land in the new one's
  //    cache after the switch.
  queryClient?.clear();

  // 2. Client-held collections.
  for (const { store, initial, teardown } of stores) {
    try {
      teardown?.();
      store.setState({ ...initial });
    } catch {
      // A store that refuses to reset must not block the rest of the wipe.
    }
  }

  // 3. Persisted per-user state.
  try {
    // Snapshot the key list through the Storage API before removing anything.
    // `Object.keys(localStorage)` is not a reliable enumeration — Storage keys
    // are not guaranteed own enumerable properties, and it returns nothing under
    // jsdom, which silently turned the prefix sweep below into a no-op in tests
    // and, worse, would have left saved table views behind in any browser that
    // implements Storage the same way.
    const keys: string[] = [];
    for (let i = 0; i < localStorage.length; i += 1) {
      const key = localStorage.key(i);
      if (key !== null) keys.push(key);
    }

    for (const key of keys) {
      if (USER_SCOPED_KEYS.includes(key) || USER_SCOPED_PREFIXES.some((p) => key.startsWith(p))) {
        localStorage.removeItem(key);
      }
    }

    // Session storage is tab-scoped, and the tab outlives the session.
    sessionStorage.clear();
  } catch {
    // Storage can be unavailable (privacy mode, quota). The cache clear above is
    // the part that carries the cross-tenant guarantee; do not abort on this.
  }
}

/** Test seam: drops the registrations so each test starts from a known state. */
export function __resetSessionScopeRegistry(): void {
  queryClient = null;
  stores.length = 0;
}

/** Test seam: what would be wiped, without wiping it. */
export const __sessionScopeKeys = { USER_SCOPED_KEYS, USER_SCOPED_PREFIXES };
