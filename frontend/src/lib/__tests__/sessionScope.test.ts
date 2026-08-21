// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// W0-05 / D9 — the client must not carry one identity's data into the next
// session. These tests assert the effect (the cache is empty, the store is back
// to its initial shape, the key is gone), not that a function was called.

import { describe, it, expect, beforeEach } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { create } from 'zustand';
import {
  clearSessionScope,
  registerQueryClient,
  registerTenantStore,
  __resetSessionScopeRegistry,
} from '../sessionScope';

describe('clearSessionScope', () => {
  beforeEach(() => {
    __resetSessionScopeRegistry();
    localStorage.clear();
    sessionStorage.clear();
  });

  it('empties the query cache, so the next user cannot be painted from it', () => {
    const client = new QueryClient();
    // What user A's session fetched.
    client.setQueryData(['risks'], [{ id: 'r1', title: 'Tenant A only' }]);
    client.setQueryData(['members'], [{ email: 'a@tenant-a.test' }]);
    registerQueryClient(client);

    expect(client.getQueryData(['risks'])).toBeDefined();

    clearSessionScope();

    expect(client.getQueryData(['risks'])).toBeUndefined();
    expect(client.getQueryData(['members'])).toBeUndefined();
    expect(client.getQueryCache().getAll()).toHaveLength(0);
  });

  it('returns registered tenant stores to their initial state', () => {
    const useStore = create(() => ({ risks: [{ id: 'r1' }], total: 1, isLoading: true }));
    registerTenantStore(useStore, { risks: [], total: 0, isLoading: false });

    clearSessionScope();

    expect(useStore.getState()).toEqual({ risks: [], total: 0, isLoading: false });
  });

  it('tears a store down before resetting it, so a live stream cannot refill it', () => {
    const order: string[] = [];
    const useStore = create(() => ({ rows: [1, 2, 3] }));
    // Zustand's setState is the reset; record the order the two happen in.
    const wrapped = {
      setState: (partial: Record<string, unknown>) => {
        order.push('reset');
        useStore.setState(partial as never);
      },
    };
    registerTenantStore(wrapped, { rows: [] }, () => order.push('teardown'));

    clearSessionScope();

    expect(order).toEqual(['teardown', 'reset']);
    expect(useStore.getState().rows).toEqual([]);
  });

  it('drops user-scoped storage keys but keeps device preferences', () => {
    localStorage.setItem('auth_user', '{"email":"a@tenant-a.test"}');
    localStorage.setItem('auth_token', 'legacy-token');
    localStorage.setItem('openrisk-settings-prefs', '{"prefs":{}}');
    localStorage.setItem('openrisk-onboarding', 'dismissed');
    localStorage.setItem('riskView', 'map');
    // Saved table views embed tenant vocabulary in their filter values.
    localStorage.setItem('openrisk.table.risks.views', '[{"name":"ISO gaps"}]');
    localStorage.setItem('openrisk.table.risks.columns', '["name","score"]');
    // Device preferences: these must survive, or signing in flips the theme and
    // replays the product tour for whoever comes next.
    localStorage.setItem('openrisk-ui', '{"theme":"dark"}');
    localStorage.setItem('openrisk_tour_seen_v1', '1');
    localStorage.setItem('locale', 'fr');
    sessionStorage.setItem('anything', 'x');

    clearSessionScope();

    expect(localStorage.getItem('auth_user')).toBeNull();
    expect(localStorage.getItem('auth_token')).toBeNull();
    expect(localStorage.getItem('openrisk-settings-prefs')).toBeNull();
    expect(localStorage.getItem('openrisk-onboarding')).toBeNull();
    expect(localStorage.getItem('riskView')).toBeNull();
    expect(localStorage.getItem('openrisk.table.risks.views')).toBeNull();
    expect(localStorage.getItem('openrisk.table.risks.columns')).toBeNull();
    expect(sessionStorage.getItem('anything')).toBeNull();

    expect(localStorage.getItem('openrisk-ui')).toBe('{"theme":"dark"}');
    expect(localStorage.getItem('openrisk_tour_seen_v1')).toBe('1');
    expect(localStorage.getItem('locale')).toBe('fr');
  });

  it('is safe with nothing registered and safe to call twice', () => {
    expect(() => {
      clearSessionScope();
      clearSessionScope();
    }).not.toThrow();
  });

  it('does not let one failing store abort the rest of the wipe', () => {
    const exploding = {
      setState: () => {
        throw new Error('store is broken');
      },
    };
    const useStore = create(() => ({ rows: [1] }));
    registerTenantStore(exploding, {});
    registerTenantStore(useStore, { rows: [] });
    localStorage.setItem('auth_user', 'x');

    clearSessionScope();

    // The broken store did not stop the good one, nor the storage wipe — a
    // partial clear is exactly the state that leaks.
    expect(useStore.getState().rows).toEqual([]);
    expect(localStorage.getItem('auth_user')).toBeNull();
  });
});
