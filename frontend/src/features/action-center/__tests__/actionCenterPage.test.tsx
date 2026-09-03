// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The full Action Center page's contract (#433, acceptance criteria 1-6 and 9).
//
// The panel's own contract is asserted in actionCenter.test.tsx; what is new
// here is everything paging brings with it:
//
//   - every item is reachable, not just the first page;
//   - the page number lives in the URL, so Back works and a link is shareable;
//   - a mangled ?page= never becomes a negative offset the server 400s on;
//   - the server's order survives paging, on page two as on page one;
//   - the page shares ONE row implementation with the panel, so the deep-link
//     guard cannot be bypassed by rendering through the other surface.
//
// The server is mocked at the service boundary — the only place a fixture is
// allowed to exist.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import axe from 'axe-core';

import { useUIStore } from '../../../store/uiStore';
import { ROUTES, resolveRoute, parentHref } from '../../../shared/routeModel';
import { NAV_GROUPS } from '../../../shared/navModel';
import { ActionCenterPage } from '../ActionCenterPage';
import { pageFromParam } from '../paging';
import { ActionCenterPanel } from '../ActionCenterPanel';
import { PAGE_LIMIT } from '../useActionItems';
import type { ActionCenterResponse, ActionItem } from '../actionCenterService';

vi.mock('../actionCenterService', async () => {
  const actual =
    await vi.importActual<typeof import('../actionCenterService')>('../actionCenterService');
  return { ...actual, actionCenterService: { list: vi.fn() } };
});

import { actionCenterService } from '../actionCenterService';

const list = vi.mocked(actionCenterService.list);
const TENANT = '3f1b0f9e-5a4c-4c7e-9a1d-8b2c6d5e4f30';

function item(overrides: Partial<ActionItem> & Pick<ActionItem, 'id' | 'type'>): ActionItem {
  return {
    title: 'Untitled',
    subject_resource_type: 'risk',
    subject_resource_id: '8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0',
    deep_link: '/governance',
    due_at: null,
    category_rank: 3,
    tenant_id: TENANT,
    ...overrides,
  };
}

function envelope(items: ActionItem[], total: number, offset = 0): ActionCenterResponse {
  return { data: items, generated_at: '2026-08-31T09:00:00Z', limit: PAGE_LIMIT, offset, total };
}

/** `count` approvals, all with a link that resolves, titled by their index. */
function approvals(from: number, count: number): ActionItem[] {
  return Array.from({ length: count }, (_, i) =>
    item({ id: `approval:${from + i}`, type: 'pending_approval', title: `Approval ${from + i}` }),
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{`${location.pathname}${location.search}`}</div>;
}

function renderPage(initialPath = '/action-center') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchInterval: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/action-center"
            element={
              <>
                <ActionCenterPage />
                <LocationProbe />
              </>
            }
          />
          <Route path="*" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useUIStore.setState({ lang: 'en' });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('the /action-center route declaration', () => {
  /* --- AC3 --------------------------------------------------------------- */

  it('is declared in routeModel with a parent, and resolves', () => {
    const node = ROUTES.find((r) => r.path === '/action-center');
    expect(node, '/action-center is missing from ROUTES').toBeDefined();
    expect(node!.parent).toBe('/');
    expect(ROUTES.some((r) => r.path === node!.parent)).toBe(true);
    expect(resolveRoute('/action-center')?.node.path).toBe('/action-center');
  });

  it('gives the page somewhere to go back to when opened in a fresh tab', () => {
    // Depth 1, so the tree invariant does not demand a parent — but a page
    // reached from a dashboard panel and opened cold in a new tab is exactly
    // the dead end the route model exists to prevent.
    expect(parentHref('/action-center')).toBe('/');
  });

  it('is reachable from the sidebar', () => {
    const entries = NAV_GROUPS.flatMap((group) => group.items).filter(
      (navItem) => navItem.path === '/action-center',
    );
    expect(entries).toHaveLength(1);
    expect(entries[0].labelKey).toBe('n_actionCenter');
  });
});

describe('pageFromParam', () => {
  /* --- guards the offset the server would reject with a 400 --------------- */

  it('collapses anything that is not a positive integer to page 1', () => {
    expect(pageFromParam(null)).toBe(1);
    expect(pageFromParam('')).toBe(1);
    expect(pageFromParam('0')).toBe(1);
    expect(pageFromParam('-4')).toBe(1);
    expect(pageFromParam('2.5')).toBe(1);
    expect(pageFromParam('banana')).toBe(1);
  });

  it('passes a real page number through', () => {
    expect(pageFromParam('1')).toBe(1);
    expect(pageFromParam('7')).toBe(7);
  });
});

describe('ActionCenterPage', () => {
  /* --- AC1: every item is reachable --------------------------------------- */

  it('reaches the second page through the pager, and asks the server for it', async () => {
    list.mockImplementation(async ({ offset = 0 } = {}) =>
      offset === 0
        ? envelope(approvals(1, PAGE_LIMIT), 25, 0)
        : envelope(approvals(21, 5), 25, PAGE_LIMIT),
    );

    renderPage();

    await waitFor(() =>
      expect(screen.getAllByTestId('action-center-item')).toHaveLength(PAGE_LIMIT),
    );
    expect(screen.getByText('Approval 1')).toBeInTheDocument();
    expect(screen.queryByText('Approval 21')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('action-center-next'));

    await waitFor(() => expect(screen.getByText('Approval 21')).toBeInTheDocument());
    expect(screen.getAllByTestId('action-center-item')).toHaveLength(5);
    expect(screen.queryByText('Approval 1')).not.toBeInTheDocument();

    // The offset the server actually received, not merely what rendered.
    expect(list).toHaveBeenCalledWith({ limit: PAGE_LIMIT, offset: PAGE_LIMIT });
  });

  it('puts the page in the URL so it is shareable and Back works', async () => {
    list.mockImplementation(async ({ offset = 0 } = {}) =>
      offset === 0
        ? envelope(approvals(1, PAGE_LIMIT), 25, 0)
        : envelope(approvals(21, 5), 25, PAGE_LIMIT),
    );

    renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId('action-center-item')).toHaveLength(PAGE_LIMIT),
    );

    fireEvent.click(screen.getByTestId('action-center-next'));
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent('/action-center?page=2'),
    );

    // Let page 2 settle before going back: the pager is disabled while a page
    // is in flight, so clicking mid-fetch would be a no-op and this test would
    // pass or fail on timing rather than on behaviour.
    await waitFor(() => expect(screen.getByText('Approval 21')).toBeInTheDocument());

    // Page 1 is the bare URL — two addresses for one view is how a stale link
    // ends up shared.
    fireEvent.click(screen.getByTestId('action-center-prev'));
    await waitFor(() => expect(screen.getByTestId('location').textContent).toBe('/action-center'));
  });

  it('opens directly on the page named in the URL', async () => {
    list.mockImplementation(async ({ offset = 0 } = {}) => envelope(approvals(21, 5), 25, offset));

    renderPage('/action-center?page=2');

    await waitFor(() => expect(screen.getByText('Approval 21')).toBeInTheDocument());
    expect(list).toHaveBeenCalledWith({ limit: PAGE_LIMIT, offset: PAGE_LIMIT });
  });

  it('never sends a negative offset, however mangled the URL', async () => {
    list.mockResolvedValue(envelope(approvals(1, 3), 3, 0));

    renderPage('/action-center?page=-7');

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(3));
    expect(list).toHaveBeenCalledWith({ limit: PAGE_LIMIT, offset: 0 });
  });

  it('disables the pager at both ends', async () => {
    list.mockImplementation(async ({ offset = 0 } = {}) =>
      offset === 0
        ? envelope(approvals(1, PAGE_LIMIT), 25, 0)
        : envelope(approvals(21, 5), 25, PAGE_LIMIT),
    );

    renderPage();
    // Wait for the page to SETTLE, not merely to render: both buttons are
    // disabled while a page is in flight, so asserting too early would pass on
    // the loading state and prove nothing about the bounds.
    await waitFor(() => expect(screen.getByText('Approval 1')).toBeInTheDocument());
    expect(screen.getByTestId('action-center-prev')).toBeDisabled();
    expect(screen.getByTestId('action-center-next')).not.toBeDisabled();

    fireEvent.click(screen.getByTestId('action-center-next'));
    await waitFor(() => expect(screen.getByText('Approval 21')).toBeInTheDocument());
    expect(screen.getByTestId('action-center-next')).toBeDisabled();
    expect(screen.getByTestId('action-center-prev')).not.toBeDisabled();
  });

  it('hides the pager when everything fits on one page', async () => {
    list.mockResolvedValue(envelope(approvals(1, 4), 4, 0));

    renderPage();
    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(4));
    expect(screen.queryByTestId('action-center-prev')).not.toBeInTheDocument();
    expect(screen.queryByTestId('action-center-next')).not.toBeInTheDocument();
  });

  /* --- AC4: the server's order survives paging ---------------------------- */

  it('renders the server order unchanged on a later page', async () => {
    // Not in category_rank order on purpose: the server owns the ordering and
    // pages on it. A client that re-sorted by rank would agree on page one and
    // contradict the server here.
    const secondPage = [
      item({
        id: 'incident:9',
        type: 'open_incident',
        title: 'Ransomware',
        deep_link: '/incidents/9/war-room',
        category_rank: 4,
      }),
      item({
        id: 'mitigation:3',
        type: 'overdue_mitigation',
        title: 'Patch the VPN',
        deep_link: '/risks/mitigations/8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0',
        category_rank: 1,
      }),
      item({
        id: 'approval:7',
        type: 'pending_approval',
        title: 'Accept legacy SFTP',
        category_rank: 3,
      }),
    ];
    list.mockResolvedValue(envelope(secondPage, 23, PAGE_LIMIT));

    renderPage('/action-center?page=2');

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(3));
    expect(
      screen
        .getAllByTestId('action-center-item')
        .map((node) => node.getAttribute('data-action-type')),
    ).toEqual(['open_incident', 'overdue_mitigation', 'pending_approval']);
  });

  /* --- AC5: the three states, on the page too ----------------------------- */

  it('renders a skeleton before the query resolves', async () => {
    let resolve: ((value: ActionCenterResponse) => void) | undefined;
    list.mockReturnValue(
      new Promise<ActionCenterResponse>((r) => {
        resolve = r;
      }),
    );

    renderPage();

    expect(screen.getByTestId('action-center-skeleton')).toBeInTheDocument();
    expect(screen.queryByTestId('action-center-list')).not.toBeInTheDocument();

    resolve?.(envelope(approvals(1, 2), 2, 0));
    await waitFor(() => expect(screen.getByTestId('action-center-list')).toBeInTheDocument());
    expect(screen.queryByTestId('action-center-skeleton')).not.toBeInTheDocument();
  });

  it('renders an error state with no items when the request fails', async () => {
    list.mockRejectedValue(new Error('Network Error'));

    renderPage();

    await waitFor(() => expect(screen.getByTestId('action-center-error')).toBeInTheDocument());
    expect(screen.queryByTestId('action-center-item')).not.toBeInTheDocument();
    expect(screen.queryByTestId('action-center-prev')).not.toBeInTheDocument();
  });

  it('reads an empty first page as an all-clear', async () => {
    list.mockResolvedValue(envelope([], 0, 0));

    renderPage();

    await waitFor(() => expect(screen.getByTestId('action-center-empty')).toBeInTheDocument());
    expect(screen.getByText('You are all clear')).toBeInTheDocument();
  });

  it('distinguishes an empty page past the end from an empty queue', async () => {
    // Two different facts with two different remedies: "you have nothing to do"
    // and "this page number is past the end of your list".
    list.mockResolvedValue(envelope([], 3, PAGE_LIMIT * 4));

    renderPage('/action-center?page=5');

    await waitFor(() => expect(screen.getByTestId('action-center-empty')).toBeInTheDocument());
    expect(screen.getByText('Nothing on this page')).toBeInTheDocument();
    expect(screen.queryByText('You are all clear')).not.toBeInTheDocument();
  });

  /* --- AC6: one guarded row implementation, no second path ---------------- */

  it('drops an item whose deep link does not resolve, exactly as the panel does', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    list.mockResolvedValue(
      envelope(
        [
          item({ id: 'approval:2', type: 'pending_approval', title: 'Risk acceptance' }),
          item({
            id: 'ghost:1',
            type: 'critical_risk',
            title: 'Points nowhere',
            deep_link: '/risks/00000000-0000-0000-0000-000000000000',
            category_rank: 2,
          }),
        ],
        2,
        0,
      ),
    );

    renderPage();

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(1));
    expect(screen.getByTestId('action-center-item').getAttribute('href')).toBe('/governance');
    expect(screen.queryByText('Points nowhere')).not.toBeInTheDocument();
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('ghost:1'));
  });

  /* --- i18n --------------------------------------------------------------- */

  it('renders its own strings in French', async () => {
    useUIStore.setState({ lang: 'fr' });
    list.mockResolvedValue(envelope(approvals(1, PAGE_LIMIT), 25, 0));

    renderPage();

    await waitFor(() =>
      expect(screen.getAllByTestId('action-center-item')).toHaveLength(PAGE_LIMIT),
    );
    expect(screen.getByTestId('action-center-next')).toHaveAttribute('aria-label', 'Page suivante');
    expect(screen.getByTestId('action-center-prev')).toHaveAttribute(
      'aria-label',
      'Page précédente',
    );
  });

  /* --- AC9: axe ----------------------------------------------------------- */

  it('has no serious or critical axe violations, pager included', async () => {
    list.mockResolvedValue(envelope(approvals(1, PAGE_LIMIT), 25, 0));

    const { container } = renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId('action-center-item')).toHaveLength(PAGE_LIMIT),
    );

    const results = await axe.run(container, {
      // jsdom has no layout engine, so contrast cannot be evaluated here; it is
      // covered by check-contrast.mjs against the real tokens.
      rules: { 'color-contrast': { enabled: false } },
    });
    const blocking = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(blocking.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});

/* --- AC2: the dashboard panel's way in ------------------------------------ */

describe('the dashboard panel', () => {
  function renderPanel() {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, refetchInterval: false } },
    });
    return render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route
              path="*"
              element={
                <>
                  <ActionCenterPanel />
                  <LocationProbe />
                </>
              }
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
  }

  it('offers a real link to the full page when more is outstanding than it shows', async () => {
    list.mockResolvedValue(envelope(approvals(1, 8), 25, 0));

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-view-all')).toBeInTheDocument());
    expect(screen.getByTestId('action-center-view-all').getAttribute('href')).toBe(
      '/action-center',
    );
  });

  it('does not offer it when the panel already shows everything', async () => {
    list.mockResolvedValue(envelope(approvals(1, 3), 3, 0));

    renderPanel();

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(3));
    expect(screen.queryByTestId('action-center-view-all')).not.toBeInTheDocument();
  });

  it('navigates to the full page when the link is activated', async () => {
    list.mockResolvedValue(envelope(approvals(1, 8), 25, 0));

    renderPanel();
    await waitFor(() => expect(screen.getByTestId('action-center-view-all')).toBeInTheDocument());

    fireEvent.click(screen.getByTestId('action-center-view-all'));
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/action-center'));
  });
});
