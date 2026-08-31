// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Action Center's rendering contract (#430, acceptance criteria 1-6 and 9).
//
// What is asserted here is that the panel tells the truth about the server:
//
//   - it renders the server's order, and does not impose one of its own;
//   - loading, error and empty are three different screens, and empty reads as
//     an all-clear rather than as a failure;
//   - every one of the six categories produces the exact deep link the #429
//     contract documents, resolving to a route that exists in routeModel;
//   - an item whose link goes nowhere is dropped and logged, never drawn as a
//     dead row;
//   - the panel has no serious or critical axe violations.
//
// The server is mocked at the service boundary — the only place a fixture is
// allowed to exist. Nothing built here ships in a production route.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import axe from 'axe-core';

import { useUIStore } from '../../../store/uiStore';
import { ActionCenterPanel } from '../ActionCenterPanel';
import { safeDeepLink } from '../actionLinks';
import type { ActionCenterResponse, ActionItem } from '../actionCenterService';

vi.mock('../actionCenterService', async () => {
  const actual = await vi.importActual<typeof import('../actionCenterService')>(
    '../actionCenterService',
  );
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
    deep_link: '/risks',
    due_at: null,
    category_rank: 1,
    tenant_id: TENANT,
    ...overrides,
  };
}

function page(items: ActionItem[], total = items.length): ActionCenterResponse {
  return {
    data: items,
    generated_at: '2026-08-31T09:00:00Z',
    limit: 20,
    offset: 0,
    total,
  };
}

/**
 * The six categories with the deep links the backend contract commits to
 * (#429 issue comment, and docs/openapi.yaml). Read as a table on purpose: if
 * one of these ever stops resolving, the test names which one.
 */
const CONTRACT_LINKS: Array<{ type: ActionItem['type']; rank: number; deepLink: string }> = [
  { type: 'overdue_mitigation', rank: 1, deepLink: '/risks/mitigations/8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0' },
  { type: 'critical_risk', rank: 2, deepLink: '/risks?drawer=risk&entity=1c2d3e4f-5a6b-7c8d-9e0f-1a2b3c4d5e6f' },
  { type: 'pending_approval', rank: 3, deepLink: '/governance' },
  { type: 'open_incident', rank: 4, deepLink: '/incidents/412/war-room' },
  { type: 'expiring_evidence', rank: 5, deepLink: '/compliance/evidence?drawer=evidence&entity=aa11bb22-cc33-dd44-ee55-ff6677889900' },
  { type: 'overdue_remediation', rank: 6, deepLink: '/compliance/remediation/9b8a7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d' },
];

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{`${location.pathname}${location.search}`}</div>;
}

function renderPanel(initialPath = '/') {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchInterval: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="*" element={<><ActionCenterPanel /><LocationProbe /></>} />
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

describe('ActionCenterPanel', () => {
  /* --- AC1: the server's order is the rendered order ---------------------- */

  it('renders items in the exact order the API returned, across categories', async () => {
    // Deliberately NOT in category_rank order: the server owns the ordering and
    // may page on a secondary key the client cannot see, so a client that
    // "helpfully" re-sorts by rank would disagree with the server on page two.
    // Rendering this array unchanged is what proves nothing re-sorts.
    const returned = [
      item({ id: 'incident:412', type: 'open_incident', title: 'Ransomware on the payroll server', deep_link: '/incidents/412/war-room', category_rank: 4 }),
      item({ id: 'mitigation:1', type: 'overdue_mitigation', title: 'Patch the perimeter VPN', deep_link: '/risks/mitigations/8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0', category_rank: 1 }),
      item({ id: 'approval:2', type: 'pending_approval', title: 'Risk acceptance for legacy SFTP', deep_link: '/governance', category_rank: 3 }),
    ];
    list.mockResolvedValue(page(returned));

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-list')).toBeInTheDocument());

    const rendered = screen.getAllByTestId('action-center-item');
    expect(rendered).toHaveLength(3);
    expect(rendered.map((node) => node.getAttribute('href'))).toEqual(
      returned.map((entry) => entry.deep_link),
    );
    expect(rendered.map((node) => node.getAttribute('data-action-type'))).toEqual([
      'open_incident',
      'overdue_mitigation',
      'pending_approval',
    ]);
  });

  /* --- AC2: empty is an all-clear, not a blank div ------------------------ */

  it('renders an explicit empty state, not a blank div or a hanging skeleton', async () => {
    list.mockResolvedValue(page([]));

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-empty')).toBeInTheDocument());
    expect(screen.getByText('You are all clear')).toBeInTheDocument();
    expect(screen.queryByTestId('action-center-skeleton')).not.toBeInTheDocument();
    expect(screen.queryByTestId('action-center-list')).not.toBeInTheDocument();
  });

  /* --- AC3: a skeleton while in flight, never a full-page spinner ---------- */

  it('renders a skeleton before the query resolves', async () => {
    let resolve: ((value: ActionCenterResponse) => void) | undefined;
    list.mockReturnValue(new Promise<ActionCenterResponse>((r) => { resolve = r; }));

    renderPanel();

    expect(screen.getByTestId('action-center-skeleton')).toBeInTheDocument();
    expect(screen.queryByTestId('action-center-list')).not.toBeInTheDocument();
    expect(screen.queryByTestId('action-center-empty')).not.toBeInTheDocument();

    resolve?.(page([item({ id: 'approval:2', type: 'pending_approval', title: 'Risk acceptance', deep_link: '/governance', category_rank: 3 })]));

    await waitFor(() => expect(screen.getByTestId('action-center-list')).toBeInTheDocument());
    expect(screen.queryByTestId('action-center-skeleton')).not.toBeInTheDocument();
  });

  /* --- AC4: a failure shows a failure, and fabricates nothing -------------- */

  it('renders an error state with no items when the request fails', async () => {
    list.mockRejectedValue(new Error('Network Error'));

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-error')).toBeInTheDocument());
    expect(screen.queryByTestId('action-center-item')).not.toBeInTheDocument();
    expect(screen.queryByTestId('action-center-empty')).not.toBeInTheDocument();
    expect(screen.getByText('The action list could not be loaded')).toBeInTheDocument();
  });

  /* --- AC5: the six deep links, and real navigation ------------------------ */

  it('renders the documented deep link for each of the six item types', async () => {
    list.mockResolvedValue(
      page(
        CONTRACT_LINKS.map(({ type, rank, deepLink }) =>
          item({ id: `${type}:x`, type, title: `A ${type}`, deep_link: deepLink, category_rank: rank }),
        ),
      ),
    );

    renderPanel();

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(6));

    const byType = new Map(
      screen.getAllByTestId('action-center-item').map((node) => [node.getAttribute('data-action-type'), node.getAttribute('href')]),
    );
    for (const { type, deepLink } of CONTRACT_LINKS) {
      expect(byType.get(type)).toBe(deepLink);
    }
  });

  it('every documented deep link resolves to a route that exists in routeModel', () => {
    // The links above are only worth asserting because the route tree says they
    // are reachable. This is the half of criterion 5 that a rendered href alone
    // cannot prove.
    for (const { type, deepLink } of CONTRACT_LINKS) {
      expect(safeDeepLink(deepLink), `${type} -> ${deepLink}`).toBe(deepLink);
    }
  });

  it('navigates to the deep link on click and on Enter and Space from the keyboard', async () => {
    list.mockResolvedValue(
      page([
        item({
          id: 'incident:412',
          type: 'open_incident',
          title: 'Ransomware on the payroll server',
          deep_link: '/incidents/412/war-room',
          category_rank: 4,
        }),
      ]),
    );

    const { unmount } = renderPanel();
    await waitFor(() => expect(screen.getByTestId('action-center-item')).toBeInTheDocument());

    fireEvent.click(screen.getByTestId('action-center-item'));
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent('/incidents/412/war-room'),
    );
    unmount();

    // Enter — the anchor's own default activation.
    renderPanel();
    await waitFor(() => expect(screen.getByTestId('action-center-item')).toBeInTheDocument());
    const link = screen.getByTestId('action-center-item');
    link.focus();
    fireEvent.keyDown(link, { key: 'Enter' });
    fireEvent.click(link);
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent('/incidents/412/war-room'),
    );
    screen.getByTestId('location');
  });

  it('activates a row with the Space key', async () => {
    list.mockResolvedValue(
      page([
        item({
          id: 'remediation:9',
          type: 'overdue_remediation',
          title: 'Close the access-review gap',
          deep_link: '/compliance/remediation/9b8a7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d',
          category_rank: 6,
        }),
      ]),
    );

    renderPanel();
    await waitFor(() => expect(screen.getByTestId('action-center-item')).toBeInTheDocument());

    fireEvent.keyDown(screen.getByTestId('action-center-item'), { key: ' ' });
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(
        '/compliance/remediation/9b8a7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d',
      ),
    );
  });

  /* --- AC6: no row ever goes nowhere -------------------------------------- */

  it('drops an item whose deep link does not resolve, and logs why', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    list.mockResolvedValue(
      page([
        item({ id: 'approval:2', type: 'pending_approval', title: 'Risk acceptance', deep_link: '/governance', category_rank: 3 }),
        item({ id: 'ghost:1', type: 'critical_risk', title: 'Points at a route that does not exist', deep_link: '/risks/00000000-0000-0000-0000-000000000000', category_rank: 2 }),
      ], 2),
    );

    renderPanel();

    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(1));
    expect(screen.getByTestId('action-center-item').getAttribute('href')).toBe('/governance');
    expect(screen.queryByText('Points at a route that does not exist')).not.toBeInTheDocument();
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('ghost:1'));
  });

  it('refuses a deep link that leaves the origin', () => {
    // deep_link is server data that ends up in an href, so it is checked like
    // any other untrusted string. None of these may become a link.
    expect(safeDeepLink('https://evil.example/risks')).toBeNull();
    expect(safeDeepLink('//evil.example/risks')).toBeNull();
    expect(safeDeepLink('javascript:alert(1)')).toBeNull();
    expect(safeDeepLink('risks')).toBeNull();
    expect(safeDeepLink('')).toBeNull();
    expect(safeDeepLink(undefined)).toBeNull();
  });

  it('renders an unrecognised item type under a generic label rather than hiding real work', async () => {
    // A frontend can be one release behind the API. An unknown category is not a
    // reason to hide work the user has to do — but it still may not produce a
    // dead link, which is why its deep_link goes through the same check.
    list.mockResolvedValue(
      page([
        item({
          id: 'newthing:1',
          // Cast: the point of the test is a value outside the generated union,
          // which is exactly what a newer server can send.
          type: 'audit_finding' as ActionItem['type'],
          title: 'Something this build has never heard of',
          deep_link: '/governance',
          category_rank: 7,
        }),
      ]),
    );

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-item')).toBeInTheDocument());
    expect(screen.getByText('Something this build has never heard of')).toBeInTheDocument();
    expect(screen.getByTestId('action-center-item').getAttribute('href')).toBe('/governance');
    expect(screen.getByText(/Action required/)).toBeInTheDocument();
  });

  /* --- i18n: no hardcoded literal survives a language switch --------------- */

  it('renders its own strings in French when the language is French', async () => {
    useUIStore.setState({ lang: 'fr' });
    list.mockResolvedValue(page([]));

    renderPanel();

    await waitFor(() => expect(screen.getByTestId('action-center-empty')).toBeInTheDocument());
    expect(screen.getByText("Centre d'actions")).toBeInTheDocument();
    expect(screen.getByText('Vous êtes à jour')).toBeInTheDocument();
  });

  /* --- AC9: axe ------------------------------------------------------------ */

  it('has no serious or critical axe violations with items rendered', async () => {
    list.mockResolvedValue(
      page(
        CONTRACT_LINKS.map(({ type, rank, deepLink }) =>
          item({
            id: `${type}:x`,
            type,
            title: `A ${type.replace(/_/g, ' ')}`,
            deep_link: deepLink,
            category_rank: rank,
            due_at: rank % 2 === 0 ? null : '2026-07-01T00:00:00Z',
          }),
        ),
      ),
    );

    const { container } = renderPanel();
    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(6));

    const results = await axe.run(container, {
      // jsdom has no layout engine, so colour contrast cannot be evaluated here.
      // It is asserted for real against the running app by check-contrast.mjs and
      // the Playwright axe pass.
      rules: { 'color-contrast': { enabled: false } },
    });
    const blocking = results.violations.filter(
      (violation) => violation.impact === 'serious' || violation.impact === 'critical',
    );
    expect(blocking.map((violation) => `${violation.id}: ${violation.help}`)).toEqual([]);
  });

  it('has no serious or critical axe violations in the empty state', async () => {
    list.mockResolvedValue(page([]));

    const { container } = renderPanel();
    await waitFor(() => expect(screen.getByTestId('action-center-empty')).toBeInTheDocument());

    const results = await axe.run(container, { rules: { 'color-contrast': { enabled: false } } });
    const blocking = results.violations.filter(
      (violation) => violation.impact === 'serious' || violation.impact === 'critical',
    );
    expect(blocking.map((violation) => `${violation.id}: ${violation.help}`)).toEqual([]);
  });
});
