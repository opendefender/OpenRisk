// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #902 — the redesigned Action Center: deadline groups, kind filters with
// counts, the facts line, and a tick that does the real work after the undo
// window, through the record's own endpoint.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import type { ActionCenterResponse, ActionItem } from '../actionCenterService';
import { actionFacts, dueGroup } from '../actionFacts';

const patch = vi.fn<(...args: unknown[]) => Promise<{ data: object }>>(() =>
  Promise.resolve({ data: {} }),
);
vi.mock('../../../lib/api', () => ({
  api: {
    get: () => Promise.resolve({ data: {} }),
    patch: (...args: unknown[]) => patch(...args),
    put: (...args: unknown[]) => patch(...args),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

vi.mock('../actionCenterService', async () => {
  const actual = await vi.importActual<typeof import('../actionCenterService')>('../actionCenterService');
  return { ...actual, actionCenterService: { list: vi.fn() } };
});
import { actionCenterService } from '../actionCenterService';
import { ActionCenterPage } from '../ActionCenterPage';

const list = vi.mocked(actionCenterService.list);
const DAY = 86_400_000;
const at = (days: number) => new Date(Date.now() + days * DAY).toISOString();
const MIT = '8f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0';

function item(overrides: Partial<ActionItem> & Pick<ActionItem, 'id' | 'type'>): ActionItem {
  return {
    title: 'Untitled',
    subject_resource_type: 'risk',
    subject_resource_id: MIT,
    deep_link: '/governance',
    due_at: null,
    category_rank: 3,
    tenant_id: '3f1b0f9e-5a4c-4c7e-9a1d-8b2c6d5e4f30',
    ...overrides,
  };
}

const ITEMS: ActionItem[] = [
  item({
    id: 'vulnerability:1',
    type: 'vulnerability_sla',
    title: 'Kerberos delegation',
    deep_link: '/vulnerabilities?drawer=vulnerability&entity=' + MIT,
    due_at: at(-2),
    meta: { kev: 'true', cve_id: 'CVE-2025-55241', asset_name: 'ad-dc-01', status: 'open' },
  }),
  item({ id: 'approval:1', type: 'pending_approval', title: 'Accept R-0115', due_at: at(0) }),
  item({
    id: 'mitigation_review:1',
    type: 'mitigation_review',
    title: 'Immutable backups',
    subject_resource_type: 'mitigation',
    deep_link: '/risks/mitigations/' + MIT,
    due_at: at(2),
    meta: { status: 'REVIEW' },
  }),
  item({ id: 'evidence:1', type: 'expiring_evidence', title: 'External scan', deep_link: '/compliance/evidence', due_at: at(12) }),
  item({ id: 'risk:1', type: 'critical_risk', title: 'Ransomware', deep_link: '/risks' }),
];

function envelope(items: ActionItem[]): ActionCenterResponse {
  return { data: items, generated_at: new Date().toISOString(), limit: 100, offset: 0, total: items.length };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchInterval: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/action-center']}>
        <ActionCenterPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useUIStore.setState({ lang: 'fr' });
  list.mockResolvedValue(envelope(ITEMS));
});
afterEach(() => vi.useRealTimers());

describe('ActionCenterPage (#902)', () => {
  it('says how much is waiting and how much is late', async () => {
    renderPage();
    await waitFor(() =>
      expect(screen.getByTestId('action-center-summary')).toHaveTextContent('5 actions vous attendent · 1 en retard'),
    );
  });

  it('groups by deadline: overdue, this week, later, no date', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByTestId('action-group-late')).toBeInTheDocument());
    const titles = (g: string) =>
      within(screen.getByTestId(`action-group-${g}`))
        .getAllByTestId('action-center-item')
        .map((n) => n.textContent ?? '');
    expect(titles('late')[0]).toContain('Kerberos delegation');
    expect(titles('week').join('|')).toContain('Accept R-0115');
    expect(titles('week').join('|')).toContain('Immutable backups');
    expect(titles('later')[0]).toContain('External scan');
    expect(titles('none')[0]).toContain('Ransomware');
  });

  it('counts each kind on its chip and narrows the list', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByTestId('action-filter-vuln')).toHaveTextContent('1'));
    expect(screen.getByTestId('action-filter-all')).toHaveTextContent('5');
    expect(screen.getByTestId('action-filter-plans')).toHaveTextContent('2');
    expect(screen.queryByTestId('action-filter-incident')).toBeNull();
    fireEvent.click(screen.getByTestId('action-filter-plans'));
    expect(screen.getAllByTestId('action-center-item')).toHaveLength(2);
    fireEvent.click(screen.getByTestId('action-filter-vendor'));
    expect(screen.getByTestId('action-center-empty')).toHaveTextContent('Rien en attente dans cette catégorie');
  });

  it('offers a tick only where the work can be finished from here', async () => {
    renderPage();
    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(5));
    const row = (title: string) => screen.getByText(title).closest('li') as HTMLElement;
    expect(within(row('Immutable backups')).queryByTestId('action-center-done')).not.toBeNull();
    expect(within(row('Kerberos delegation')).queryByTestId('action-center-done')).not.toBeNull();
    expect(within(row('Accept R-0115')).queryByTestId('action-center-done')).toBeNull();
    expect(within(row('Ransomware')).queryByTestId('action-center-done')).toBeNull();
  });

  it('hides a ticked row at once and completes the record after the undo window', async () => {
    renderPage();
    await waitFor(() => expect(screen.getAllByTestId('action-center-item')).toHaveLength(5));
    vi.useFakeTimers();
    const row = screen.getByText('Immutable backups').closest('li') as HTMLElement;
    fireEvent.click(within(row).getByTestId('action-center-done'));
    expect(screen.queryByText('Immutable backups')).toBeNull();
    expect(patch).not.toHaveBeenCalled();
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(patch).toHaveBeenCalledWith(`/mitigations/${MIT}`, { status: 'DONE' });
  });
});

describe('actionFacts', () => {
  const t = (key: string, p?: Record<string, string | number>) => (p ? `${key}(${JSON.stringify(p)})` : key);

  it('words a vulnerability from its facts, never from a server sentence', () => {
    expect(actionFacts(ITEMS[0], t)).toBe(
      'actionCenter.kind.vulnerability_sla · actionCenter.fact.kev · actionCenter.fact.slaBreached · CVE-2025-55241 · ad-dc-01',
    );
  });

  it('counts the days a vendor has not answered', () => {
    const now = new Date('2026-10-08T12:00:00Z');
    const v = item({ id: 'v:1', type: 'vendor_followup', title: 'PayLink', meta: { sent_at: '2026-09-29T09:00:00Z' } });
    expect(actionFacts(v, t, now)).toContain('actionCenter.fact.unanswered({"count":9})');
  });

  it('puts an item without a date in its own group', () => {
    expect(dueGroup(ITEMS[4])).toBe('none');
    expect(dueGroup(ITEMS[0])).toBe('late');
  });
});
