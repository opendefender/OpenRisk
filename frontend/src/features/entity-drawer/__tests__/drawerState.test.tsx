// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The drawer's URL state machine (W1-02 §6-§9, §53).
//
// This is the file that matters most in the frontend suite. The drawer's entire
// claim — shareable, refresh-proof, Back-closable, context-preserving — reduces
// to "the URL is the state, and nothing but the drawer's own three keys ever
// changes". Every assertion below is that claim from a different angle.

import { describe, it, expect } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router';
import {
  readDrawer,
  writeDrawer,
  stripDrawer,
  drawerHref,
  useDrawerController,
  DRAWER_PARAMS,
  type DrawerController,
} from '../drawerState';

describe('readDrawer', () => {
  it('reads a type and an id', () => {
    expect(readDrawer('drawer=risk&entity=42')).toEqual({ type: 'risk', id: '42', tab: undefined });
  });

  it('reads the tab when present', () => {
    expect(readDrawer('drawer=asset&entity=a1&etab=timeline')).toEqual({
      type: 'asset',
      id: 'a1',
      tab: 'timeline',
    });
  });

  it('ignores an unknown tab rather than passing it through', () => {
    // The drawer would render an empty panel for a section that does not exist.
    expect(readDrawer('drawer=risk&entity=1&etab=nonsense')?.tab).toBeUndefined();
  });

  it('opens nothing for a half-written link', () => {
    // A shared URL truncated at the ampersand must degrade to the plain page,
    // not to an empty drawer.
    expect(readDrawer('drawer=risk')).toBeNull();
    expect(readDrawer('entity=42')).toBeNull();
    expect(readDrawer('')).toBeNull();
  });

  it('ignores a type the client does not know', () => {
    // A typo in a pasted link degrades to the page. The server refuses it too;
    // this stops the request being made at all.
    expect(readDrawer('drawer=employee&entity=42')).toBeNull();
  });

  it('accepts every supported type', () => {
    for (const t of [
      'asset',
      'risk',
      'vulnerability',
      'finding',
      'control',
      'incident',
      'vendor',
      'evidence',
    ]) {
      expect(readDrawer(`drawer=${t}&entity=x`)?.type).toBe(t);
    }
  });
});

describe('writeDrawer', () => {
  it('preserves every parameter the page already had', () => {
    const before = new URLSearchParams('severity=critical&page=3&sort=score:desc&f.tier=P1');
    const after = writeDrawer(before, { type: 'risk', id: '42' });

    expect(after.get('severity')).toBe('critical');
    expect(after.get('page')).toBe('3');
    expect(after.get('sort')).toBe('score:desc');
    expect(after.get('f.tier')).toBe('P1');
    expect(after.get('drawer')).toBe('risk');
    expect(after.get('entity')).toBe('42');
  });

  it('does not mutate the params it was given', () => {
    // Callers hold the router's live object; mutating it would change the URL
    // behind the router's back.
    const before = new URLSearchParams('page=3');
    writeDrawer(before, { type: 'risk', id: '42' });
    expect(before.get('drawer')).toBeNull();
  });

  it('omits the default tab so two links to the same record look the same', () => {
    const params = writeDrawer(new URLSearchParams(), { type: 'risk', id: '42', tab: 'summary' });
    expect(params.get('etab')).toBeNull();
  });

  it('writes a non-default tab', () => {
    const params = writeDrawer(new URLSearchParams(), { type: 'risk', id: '42', tab: 'audit' });
    expect(params.get('etab')).toBe('audit');
  });

  it('clears a stale tab when switching back to the default', () => {
    const params = writeDrawer(new URLSearchParams('etab=audit'), {
      type: 'risk',
      id: '42',
      tab: 'summary',
    });
    expect(params.get('etab')).toBeNull();
  });
});

describe('stripDrawer', () => {
  it('removes only the drawer keys', () => {
    const before = new URLSearchParams('severity=critical&page=3&drawer=risk&entity=42&etab=audit');
    const after = stripDrawer(before);

    expect(after.get('severity')).toBe('critical');
    expect(after.get('page')).toBe('3');
    for (const key of DRAWER_PARAMS) expect(after.get(key)).toBeNull();
  });

  it('round-trips: open then close returns the exact original query', () => {
    // This is §9 in one assertion. If it ever fails, closing a drawer drops the
    // user's filters.
    const original = 'q=log&severity=critical%2Chigh&page=3&size=50&sort=score%3Adesc';
    const opened = writeDrawer(new URLSearchParams(original), {
      type: 'risk',
      id: '42',
      tab: 'timeline',
    });
    const closed = stripDrawer(opened);
    expect(closed.toString()).toBe(new URLSearchParams(original).toString());
  });
});

describe('drawerHref', () => {
  it('builds a link that matches the server-built deep link', () => {
    // entity.DeepLink on the server emits exactly this shape. The two must agree
    // or a relation chip and a timeline row would point at different URLs for
    // the same record.
    expect(drawerHref('/risks', 'risk', 'abc')).toBe('/risks?drawer=risk&entity=abc');
  });

  it('preserves a query already on the path', () => {
    const href = drawerHref('/risks?severity=critical', 'risk', 'abc');
    expect(href).toContain('severity=critical');
    expect(href).toContain('drawer=risk');
    expect(href).toContain('entity=abc');
  });

  it('carries a tab', () => {
    expect(drawerHref('/assets', 'asset', 'a1', 'timeline')).toContain('etab=timeline');
  });
});

// ---------------------------------------------------------------------------
// The controller, against a real router
// ---------------------------------------------------------------------------

/** Renders the controller and exposes it plus the live location. */
function Harness({ onReady }: { onReady: (c: DrawerController, search: string) => void }) {
  const controller = useDrawerController();
  const location = useLocation();
  onReady(controller, location.search);
  return (
    <div>
      <span data-testid="search">{location.search}</span>
      <span data-testid="state">{JSON.stringify(controller.state)}</span>
    </div>
  );
}

function mount(initial: string) {
  let controller!: DrawerController;
  let search = '';
  render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route
          path="/risks"
          element={
            <Harness
              onReady={(c, s) => {
                controller = c;
                search = s;
              }}
            />
          }
        />
      </Routes>
    </MemoryRouter>,
  );
  return {
    get controller() {
      return controller;
    },
    get search() {
      return search;
    },
    rendered: () => screen.getByTestId('search').textContent ?? '',
  };
}

describe('useDrawerController', () => {
  it('reads a drawer straight out of the initial URL — the deep-link case', () => {
    // A link opened in a new tab, or a refresh: there is no prior state to
    // restore from, only the URL.
    const h = mount('/risks?drawer=risk&entity=42&etab=timeline');
    expect(h.controller.state).toEqual({ type: 'risk', id: '42', tab: 'timeline' });
  });

  it('opens an entity while keeping the page context', () => {
    const h = mount('/risks?severity=critical&page=3');
    act(() => h.controller.open('risk', '42'));

    const params = new URLSearchParams(h.rendered());
    expect(params.get('drawer')).toBe('risk');
    expect(params.get('entity')).toBe('42');
    expect(params.get('severity')).toBe('critical');
    expect(params.get('page')).toBe('3');
  });

  it('closes back to exactly the page state it started from', () => {
    const h = mount('/risks?severity=critical&page=3');
    act(() => h.controller.open('risk', '42'));
    act(() => h.controller.close());

    const params = new URLSearchParams(h.rendered());
    expect(params.get('drawer')).toBeNull();
    expect(params.get('entity')).toBeNull();
    expect(params.get('severity')).toBe('critical');
    expect(params.get('page')).toBe('3');
  });

  it('switches tab without losing the entity', () => {
    const h = mount('/risks?drawer=risk&entity=42');
    act(() => h.controller.setTab('audit'));

    expect(h.controller.state).toEqual({ type: 'risk', id: '42', tab: 'audit' });
  });

  it('does nothing when asked to switch tab with no drawer open', () => {
    const h = mount('/risks?page=2');
    act(() => h.controller.setTab('audit'));

    expect(h.controller.state).toBeNull();
    expect(new URLSearchParams(h.rendered()).get('etab')).toBeNull();
    expect(new URLSearchParams(h.rendered()).get('page')).toBe('2');
  });

  it('navigating to a related entity replaces the target, not the page', () => {
    const h = mount('/risks?severity=critical&drawer=risk&entity=42');
    act(() => h.controller.open('asset', 'a-1'));

    const params = new URLSearchParams(h.rendered());
    expect(params.get('drawer')).toBe('asset');
    expect(params.get('entity')).toBe('a-1');
    expect(params.get('severity')).toBe('critical');
  });

  it('drops a stale tab when the new entity is opened without one', () => {
    // Opening an asset from a risk's Audit tab must not land on the asset's
    // Audit tab by accident — the user asked for the asset, not for a tab.
    const h = mount('/risks?drawer=risk&entity=42&etab=audit');
    act(() => h.controller.open('asset', 'a-1'));

    expect(h.controller.state).toEqual({ type: 'asset', id: 'a-1', tab: undefined });
  });
});
