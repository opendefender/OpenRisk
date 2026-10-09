// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Action Center's rendering contract (#430, criteria 1-6 and 9), carried by
// the dashboard's priority list since #901 replaced the panel.
//
// What is asserted here is that the list tells the truth about the server:
//
//   - it renders the server's order, and does not impose one of its own;
//   - loading, error and empty are three different screens, and empty reads as
//     an all-clear rather than as a failure;
//   - every one of the six categories produces the exact deep link the #429
//     contract documents, resolving to a route that exists in routeModel;
//   - an item whose link goes nowhere is dropped and logged, never drawn as a
//     dead row;
//   - the list has no serious or critical axe violations.
//
// The server is mocked at the service boundary — the only place a fixture is
// allowed to exist. Nothing built here ships in a production route.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import axe from 'axe-core';

import { useUIStore } from '../../../store/uiStore';
import { PriorityList } from '../console/PriorityList';
import { safeDeepLink } from '../../action-center/actionLinks';
import type { ActionCenterResponse, ActionItem } from '../../action-center/actionCenterService';

vi.mock('../../action-center/actionCenterService', async () => {
  const actual =
    await vi.importActual<typeof import('../../action-center/actionCenterService')>('../../action-center/actionCenterService');
  return { ...actual, actionCenterService: { list: vi.fn() } };
});

import { actionCenterService } from '../../action-center/actionCenterService';

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
  {
    type: 'overdue_mitigation',
    rank: 1,
    deepLink: '/risks/mitigations/8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0',
  },
  {
    type: 'critical_risk',
    rank: 2,
    deepLink: '/risks?drawer=risk&entity=1c2d3e4f-5a6b-7c8d-9e0f-1a2b3c4d5e6f',
  },
  { type: 'pending_approval', rank: 3, deepLink: '/governance' },
  { type: 'open_incident', rank: 4, deepLink: '/incidents/412/war-room' },
  {
    type: 'expiring_evidence',
    rank: 5,
    deepLink: '/compliance/evidence?drawer=evidence&entity=aa11bb22-cc33-dd44-ee55-ff6677889900',
  },
  {
    type: 'overdue_remediation',
    rank: 6,
    deepLink: '/compliance/remediation/9b8a7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d',
  },
];

function renderList() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchInterval: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/']}>
        <PriorityList variant="rssi" />
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

// #901: the dashboard's priority list replaced the action-center panel. The
// contract it keeps: the server's order, honest states, and no dead or
// off-origin link ever becoming a row.
describe('dashboard PriorityList', () => {
  it('renders items in the exact order the API returned, across categories', async () => {
    const returned = [
      item({ id: 'incident:412', type: 'open_incident', title: 'Ransomware', deep_link: '/incidents/412/war-room', category_rank: 4 }),
      item({ id: 'mitigation:1', type: 'overdue_mitigation', title: 'Patch the VPN', deep_link: '/risks/mitigations/8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0', category_rank: 1 }),
      item({ id: 'approval:2', type: 'pending_approval', title: 'Accept SFTP', deep_link: '/governance', category_rank: 3 }),
    ];
    list.mockResolvedValue(page(returned));
    renderList();
    await waitFor(() => expect(screen.getAllByTestId('dash-priority-item')).toHaveLength(3));
    const rendered = screen.getAllByTestId('dash-priority-item');
    expect(rendered.map((n) => n.getAttribute('href'))).toEqual(returned.map((r) => r.deep_link));
    expect(rendered.map((n) => n.getAttribute('data-action-type'))).toEqual([
      'open_incident',
      'overdue_mitigation',
      'pending_approval',
    ]);
  });

  it('shows at most five rows', async () => {
    list.mockResolvedValue(
      page(Array.from({ length: 8 }, (_, i) => item({ id: `risk:${i}`, type: 'critical_risk', title: `R${i}` }))),
    );
    renderList();
    await waitFor(() => expect(screen.getAllByTestId('dash-priority-item')).toHaveLength(5));
  });

  it('renders an explicit empty state', async () => {
    list.mockResolvedValue(page([]));
    renderList();
    await waitFor(() => expect(screen.getByTestId('dash-priorities-empty')).toBeInTheDocument());
  });

  it('renders a skeleton before the query resolves', () => {
    list.mockReturnValue(new Promise<ActionCenterResponse>(() => undefined));
    renderList();
    expect(document.querySelector('[aria-busy="true"]')).not.toBeNull();
  });

  it('renders an error with a retry when the request fails', async () => {
    list.mockRejectedValue(new Error('boom'));
    renderList();
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(screen.queryByTestId('dash-priority-item')).toBeNull();
  });

  it('every documented deep link resolves to a route that exists in routeModel', () => {
    for (const { type, deepLink } of CONTRACT_LINKS) {
      expect(safeDeepLink(deepLink), `${type} -> ${deepLink}`).toBe(deepLink);
    }
  });

  it('drops an item whose deep link does not resolve, and logs why', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    list.mockResolvedValue(
      page([
        item({ id: 'ghost:1', type: 'critical_risk', title: 'Ghost', deep_link: '/nowhere/at/all' }),
        item({ id: 'risk:2', type: 'critical_risk', title: 'Real', deep_link: '/risks' }),
      ]),
    );
    renderList();
    await waitFor(() => expect(screen.getAllByTestId('dash-priority-item')).toHaveLength(1));
    expect(screen.queryByText('Ghost')).toBeNull();
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('ghost:1'));
  });

  it('refuses a deep link that leaves the origin', () => {
    expect(safeDeepLink('https://evil.example/risks')).toBeNull();
    expect(safeDeepLink('//evil.example/risks')).toBeNull();
    expect(safeDeepLink('javascript:alert(1)')).toBeNull();
    expect(safeDeepLink('risks')).toBeNull();
    expect(safeDeepLink('')).toBeNull();
    expect(safeDeepLink(undefined)).toBeNull();
  });

  it('renders an unrecognised item type under a generic label rather than hiding real work', async () => {
    list.mockResolvedValue(
      page([
        item({
          id: 'newthing:1',
          type: 'audit_finding' as ActionItem['type'],
          title: 'Something this build has never heard of',
          deep_link: '/governance',
        }),
      ]),
    );
    renderList();
    await waitFor(() => expect(screen.getByTestId('dash-priority-item')).toBeInTheDocument());
    expect(screen.getByText('Action required')).toBeInTheDocument();
  });

  it('has no serious or critical axe violations with items rendered', async () => {
    list.mockResolvedValue(
      page(
        CONTRACT_LINKS.slice(0, 5).map(({ type, rank, deepLink }) =>
          item({ id: `${type}:x`, type, title: `A ${type}`, deep_link: deepLink, category_rank: rank, due_at: '2026-07-01T00:00:00Z' }),
        ),
      ),
    );
    const { container } = renderList();
    await waitFor(() => expect(screen.getAllByTestId('dash-priority-item')).toHaveLength(5));
    const results = await axe.run(container, { rules: { 'color-contrast': { enabled: false } } });
    const blocking = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
    expect(blocking.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});
