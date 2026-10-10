// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Topology on the redesign (#907): exposure paths, node panel, search, the
// list view, and the pure helpers behind them.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { highlightFor, neighbourhood, searchNodes } from '../topologyModel';
import type { TopologyEdge, TopologyNode } from '../topologyTypes';

const FW = 'f0000000-0000-4000-8000-000000000001';
const API = 'a0000000-0000-4000-8000-000000000002';
const DB = 'd0000000-0000-4000-8000-000000000003';
const AD = 'ad000000-0000-4000-8000-000000000004';
const E1 = 'e1000000-0000-4000-8000-000000000001';
const E2 = 'e2000000-0000-4000-8000-000000000002';
const E3 = 'e3000000-0000-4000-8000-000000000003';

const node = (id: string, name: string, criticality: string, exposed = false): TopologyNode =>
  ({
    id,
    name,
    type: 'Serveur',
    criticality,
    zone: 'dc',
    internet_exposed: exposed,
    risk_count: 1,
    max_risk_score: 6.4,
    vuln_count: 2,
    degree: 1,
  }) as TopologyNode;
const NODES = [
  node(FW, 'Pare-feu périmétrique', 'HIGH', true),
  node(API, 'API banque mobile', 'HIGH'),
  node(DB, 'Base clients', 'CRITICAL'),
  node(AD, 'Contrôleur de domaine AD', 'CRITICAL'),
];
const EDGES = [
  { id: E1, source: FW, target: API, type: 'connects_to', raw_type: 'connects_to' },
  { id: E2, source: API, target: DB, type: 'depends_on', raw_type: 'depends_on' },
  { id: E3, source: FW, target: AD, type: 'connects_to', raw_type: 'connects_to' },
] as TopologyEdge[];

let topology: unknown;
vi.mock('../../../lib/api', () => ({
  api: {
    get: (url: string) => {
      if (url === '/attack-surface/topology') return Promise.resolve({ data: topology });
      if (url.startsWith('/asset-dependencies')) return Promise.resolve({ data: [] });
      if (url.endsWith('/analysis'))
        return Promise.resolve({
          data: {
            asset_id: AD,
            criticality: 'CRITICAL',
            internet_facing: false,
            by_severity: { critical: 0, high: 1, medium: 0, low: 0 },
            open: 1,
            kev_open: 1,
            overdue: 0,
            resolved: 0,
            last_detected_at: '2026-10-08T00:00:00Z',
            truncated: false,
            vulnerabilities: [],
            risks: [],
            findings: [{ code: 'kev_open', count: 1 }],
            next_action: null,
          },
        });
      return Promise.resolve({ data: {} });
    },
    post: () => Promise.resolve({ data: {} }),
    delete: () => Promise.resolve({ data: {} }),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

import TopologyView from '../TopologyView';

function Where() {
  const l = useLocation();
  return <div data-testid="where">{l.search}</div>;
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/assets/topology']}>
        <Routes>
          <Route path="*" element={<TopologyView />} />
        </Routes>
        <Where />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  useAuthStore.setState({
    user: {
      id: 'u',
      email: 'u@example.test',
      username: 'u',
      full_name: 'U',
      role: '',
      tenant_id: 'org-a',
      permissions: ['*'],
    },
  });
  topology = {
    nodes: NODES,
    edges: EDGES,
    zones: [{ key: 'dc', label: 'DC Dakar', count: 4 }],
    truncated: false,
    exposure_paths: [
      { asset_ids: [FW, AD], edge_ids: [E3], hops: 2 },
      { asset_ids: [FW, API, DB], edge_ids: [E1, E2], hops: 3 },
    ],
  };
});

describe('#907 — topology helpers', () => {
  it('reads a node’s neighbourhood both ways', () => {
    const n = neighbourhood(EDGES, API);
    expect(n.up).toEqual([DB]);
    expect(n.down).toEqual([FW]);
    expect(n.edges).toEqual([E1, E2]);
  });

  it('highlights a chain first, then paths, then the neighbourhood', () => {
    const chain = { nodes: new Set([AD]), edges: new Set<string>() };
    expect(highlightFor({ chain, edges: EDGES })).toBe(chain);
    const paths = highlightFor({
      paths: [{ asset_ids: [FW, AD], edge_ids: [E3], hops: 2 }],
      edges: EDGES,
    });
    expect([...(paths?.nodes ?? [])]).toEqual([FW, AD]);
    const sel = highlightFor({ selectedId: DB, edges: EDGES });
    expect([...(sel?.nodes ?? [])].sort()).toEqual([API, DB].sort());
    expect(highlightFor({ edges: EDGES })).toBeNull();
  });

  it('searches names without accents, prefixes first', () => {
    expect(searchNodes(NODES, 'controleur').map((n) => n.name)).toEqual([
      'Contrôleur de domaine AD',
    ]);
    expect(searchNodes(NODES, 'ba').map((n) => n.name)).toEqual([
      'Base clients',
      'API banque mobile',
    ]);
    expect(searchNodes(NODES, '  ')).toEqual([]);
  });
});

describe('#907 — topology page', () => {
  it('lists the exposure paths from the internet, shortest first', async () => {
    renderPage();
    const paths = await screen.findAllByTestId('topo-path');
    expect(paths).toHaveLength(2);
    expect(paths[0]).toHaveTextContent('Chemin 1 · 2 sauts');
    expect(paths[0]).toHaveTextContent(
      'Internet → Pare-feu périmétrique → Contrôleur de domaine AD',
    );
    expect(paths[1]).toHaveTextContent(
      'Internet → Pare-feu périmétrique → API banque mobile → Base clients',
    );
    expect(screen.getByTestId('topo-paths')).toHaveTextContent(
      '2 chemins relient Internet à un actif critique en 3 sauts au plus.',
    );
    fireEvent.click(paths[0]);
    expect(paths[0]).toHaveAttribute('aria-pressed', 'true');
  });

  it('says so when no asset is declared exposed', async () => {
    topology = {
      ...(topology as object),
      nodes: NODES.map((n) => ({ ...n, internet_exposed: false })),
      exposure_paths: [],
    };
    renderPage();
    expect(await screen.findByTestId('topo-paths')).toHaveTextContent(
      "Aucun actif n'est déclaré exposé",
    );
  });

  it('search centres on an asset and opens its panel with its exposure', async () => {
    renderPage();
    await screen.findAllByTestId('topo-path');
    fireEvent.change(screen.getByTestId('topo-search'), { target: { value: 'contro' } });
    fireEvent.keyDown(screen.getByTestId('topo-search'), { key: 'Enter' });
    const panel = await screen.findByTestId('topo-node');
    expect(panel).toHaveTextContent('Contrôleur de domaine AD');
    expect(await within(panel).findByTestId('exposure-digest')).toHaveTextContent(
      '1 vulnérabilité exploitée activement',
    );
    expect(within(panel).getByText('Pare-feu périmétrique')).toBeInTheDocument();
    fireEvent.click(within(panel).getByRole('button', { name: "Ouvrir l'actif" }));
    await waitFor(() =>
      expect(screen.getByTestId('where')).toHaveTextContent(`drawer=asset&entity=${AD}`),
    );
  });

  it('the list view gives the same graph as a table', async () => {
    renderPage();
    await screen.findAllByTestId('topo-path');
    fireEvent.click(screen.getByRole('tab', { name: 'Liste' }));
    const table = await screen.findByTestId('topo-list');
    const api = within(table).getByRole('rowheader', { name: 'API banque mobile' }).closest('tr');
    expect(api).toHaveTextContent('Base clients');
    expect(api).toHaveTextContent('Pare-feu périmétrique');
    // Critical first.
    expect(within(table).getAllByRole('rowheader')[0]).toHaveTextContent(/Base clients|Contrôleur/);
  });

  it('an empty estate says what to do', async () => {
    topology = { nodes: [], edges: [], zones: [], truncated: false, exposure_paths: [] };
    renderPage();
    expect(await screen.findByText('Aucun actif à cartographier.')).toBeInTheDocument();
  });
});
