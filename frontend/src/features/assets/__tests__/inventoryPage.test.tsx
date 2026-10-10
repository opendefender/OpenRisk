// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The inventory of the October 2026 redesign (#906), against the API at its
// HTTP boundary. Permissions go through the real auth store.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';

const posts = vi.fn<(url: string) => Promise<{ data: unknown }>>();
const gets = vi.fn<(url: string) => void>();
let assets: unknown[] = [];
let configs: unknown[] = [];
let failAssets = false;

const CORE = 'aaaaaaaa-0000-4000-8000-000000000001';
const AD = 'bbbbbbbb-0000-4000-8000-000000000002';
const SAAS = 'cccccccc-0000-4000-8000-000000000003';

vi.mock('../../../lib/api', () => ({
  api: {
    get: (url: string) => {
      gets(url);
      if (url.startsWith('/assets/exposure'))
        return Promise.resolve({
          data: {
            items: [
              {
                asset_id: AD,
                open_vulnerabilities: 1,
                kev_open: 1,
                last_detected_at: '2026-10-07T09:00:00Z',
              },
              {
                asset_id: CORE,
                open_vulnerabilities: 2,
                kev_open: 0,
                last_detected_at: '2026-10-06T09:00:00Z',
              },
            ],
          },
        });
      if (url.startsWith('/assets'))
        return failAssets ? Promise.reject(new Error('boom')) : Promise.resolve({ data: assets });
      if (url === '/scanner/configs') return Promise.resolve({ data: configs });
      return Promise.resolve({ data: {} });
    },
    post: (url: string) => posts(url),
    put: () => Promise.resolve({ data: {} }),
    patch: () => Promise.resolve({ data: {} }),
    delete: () => Promise.resolve({ data: {} }),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

import { InventoryPage } from '../InventoryPage';

function signIn(permissions: string[]) {
  useAuthStore.setState({
    user: {
      id: 'u',
      email: 'u@example.test',
      username: 'u',
      full_name: 'U',
      role: '',
      tenant_id: 'org-a',
      permissions,
    },
  });
}

function Where() {
  const l = useLocation();
  return <div data-testid="where">{`${l.pathname}${l.search}`}</div>;
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/assets']}>
        <Routes>
          <Route path="*" element={<InventoryPage />} />
        </Routes>
        <Where />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  posts.mockReset();
  gets.mockReset();
  failAssets = false;
  configs = [];
  useUIStore.setState({ lang: 'fr' });
  signIn(['*']);
  assets = [
    {
      id: CORE,
      name: 'Système cœur bancaire',
      type: 'Application',
      criticality: 'CRITICAL',
      owner: 'Jean-Marc Ekotto',
      category: 'application',
      attributes: { physical_location: 'DC Dakar', resource_id: 'core-banking' },
      risks: [
        { id: 'r1', score: 6 },
        { id: 'r2', score: 4 },
        { id: 'r3', score: 2 },
      ],
    },
    {
      id: AD,
      name: 'Contrôleur de domaine AD',
      type: 'Annuaire',
      criticality: 'CRITICAL',
      owner: '',
      category: 'server',
      hostnames: ['ad-dc-01'],
      attributes: {},
      risks: [{ id: 'r4', score: 5 }],
    },
    {
      id: SAAS,
      name: 'Kora RH — paie',
      type: 'SaaS',
      criticality: 'MEDIUM',
      owner: 'Mariam Traoré',
      category: 'cloud',
      attributes: {},
      risks: [],
    },
  ];
});

function rowOf(name: string) {
  const cell = screen.getByText(name);
  const tr = cell.closest('tr');
  if (!tr) throw new Error(`no row for ${name}`);
  return within(tr);
}

describe('#906 — inventory', () => {
  it('summarises the inventory and says agent coverage is not measured', async () => {
    renderPage();
    expect(await screen.findByTestId('inv-summary')).toHaveTextContent(
      '3 actifs · 2 critiques · couverture des agents non mesurée',
    );
  });

  it('shows each asset with its slug, type, owner, location, risks, vulnerabilities and KEV', async () => {
    renderPage();
    await screen.findByText('Système cœur bancaire');
    const core = rowOf('Système cœur bancaire');
    expect(core.getByText('core-banking')).toBeInTheDocument();
    expect(core.getByText('DC Dakar')).toBeInTheDocument();
    expect(core.getByText('Jean-Marc Ekotto')).toBeInTheDocument();
    expect(core.getByText('3')).toBeInTheDocument();
    await waitFor(() => expect(core.getByTestId('inv-vulns')).toHaveTextContent('2'));
    expect(core.queryByText('KEV')).toBeNull();

    const ad = rowOf('Contrôleur de domaine AD');
    expect(ad.getByText('ad-dc-01')).toBeInTheDocument();
    await waitFor(() => expect(ad.getByTestId('inv-vulns')).toHaveTextContent('1KEV'));
  });

  it('filters by category tab, with counts, and by KEV', async () => {
    renderPage();
    await screen.findByText('Kora RH — paie');
    const tabs = screen.getAllByRole('tab');
    expect(tabs.map((tab) => tab.textContent)).toEqual([
      'Tous3',
      'Serveurs1',
      'Applications1',
      'Cloud1',
    ]);
    fireEvent.click(screen.getByTestId('inv-cat-cloud'));
    expect(screen.queryByText('Système cœur bancaire')).toBeNull();
    expect(screen.getByText('Kora RH — paie')).toBeInTheDocument();
    expect(screen.getByTestId('inv-cat-cloud')).toHaveAttribute('aria-selected', 'true');

    fireEvent.click(screen.getByTestId('inv-cat-all'));
    const kev = await screen.findByTestId('inv-kev-only');
    expect(kev).toHaveTextContent('1');
    fireEvent.click(kev);
    expect(screen.getByText('Contrôleur de domaine AD')).toBeInTheDocument();
    expect(screen.queryByText('Système cœur bancaire')).toBeNull();
    expect(kev).toHaveAttribute('aria-pressed', 'true');
  });

  it('does not ask for vulnerabilities a member may not read', async () => {
    signIn(['assets:read']);
    renderPage();
    await screen.findByText('Système cœur bancaire');
    expect(gets.mock.calls.some(([u]) => u.startsWith('/assets/exposure'))).toBe(false);
    expect(screen.queryByTestId('inv-vulns')).toBeNull();
    expect(screen.queryByTestId('inv-discover')).toBeNull();
    expect(screen.queryByTestId('inv-kev-only')).toBeNull();
  });

  it('opens the asset drawer from a row', async () => {
    renderPage();
    fireEvent.click(await screen.findByText('Kora RH — paie'));
    await waitFor(() =>
      expect(screen.getByTestId('where')).toHaveTextContent(`drawer=asset&entity=${SAAS}`),
    );
  });

  it('discovery with no source goes to Infrastructure', async () => {
    renderPage();
    const btn = await screen.findByTestId('inv-discover');
    await waitFor(() => expect(btn).not.toHaveAttribute('aria-busy', 'true'));
    fireEvent.click(btn);
    await waitFor(() => expect(screen.getByTestId('where')).toHaveTextContent('/infrastructure'));
    expect(posts).not.toHaveBeenCalled();
  });

  it('discovery with one source starts it', async () => {
    configs = [{ id: 'cfg1', name: 'Plages internes', enabled: true }];
    posts.mockResolvedValue({ data: { id: 'job1' } });
    renderPage();
    const btn = await screen.findByTestId('inv-discover');
    await waitFor(() => expect(gets).toHaveBeenCalledWith('/scanner/configs'));
    await waitFor(() => expect(btn).not.toHaveAttribute('aria-busy', 'true'));
    fireEvent.click(btn);
    await waitFor(() => expect(posts).toHaveBeenCalledWith('/scanner/configs/cfg1/scan'));
  });

  it('a failed load offers a retry', async () => {
    failAssets = true;
    renderPage();
    expect(await screen.findByRole('button', { name: /réessayer|retry/i })).toBeInTheDocument();
  });

  it('an empty inventory says what to do', async () => {
    assets = [];
    renderPage();
    expect(await screen.findByText('Aucun actif inventorié')).toBeInTheDocument();
  });
});
