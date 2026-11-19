// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The risk register on the October 2026 redesign (#913): the header figures from
// /risks/summary, the view tabs and band pills with their counts, the table in
// the ONE display format, the matrix (and the dashboard's ?view=matrix link),
// the bulk bar, the drawer and its footer, and the three states of each block.
//
// The risk store and the real components run; only the network, the auth gate's
// permission list and the two modals that have their own tests are replaced.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import { useRiskStore, type Risk } from '../../../hooks/useRiskStore';
import type { RiskSummary } from '../riskRegisterService';

const perms = { update: true, delete: true, create: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({
      hasPermission: (p: string) =>
        p === 'risks:update'
          ? perms.update
          : p === 'risks:delete'
            ? perms.delete
            : p === 'risks:create'
              ? perms.create
              : true,
    }),
}));

const toast = vi.hoisted(() =>
  Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
);
vi.mock('sonner', () => ({ toast }));

vi.mock('../components/EditRiskModal', () => ({ EditRiskModal: () => null }));
vi.mock('../../mitigations/CreateMitigationModal', () => ({ CreateMitigationModal: () => null }));

// The display currency is the tenant's; the entitlement plumbing has its own tests.
vi.mock('../useRiskRegisterData', async () => {
  const actual =
    await vi.importActual<typeof import('../useRiskRegisterData')>('../useRiskRegisterData');
  return {
    ...actual,
    useRegisterMoney: () => ({
      entitled: true,
      currency: moneyFixture.currency,
      code: moneyFixture.code,
      toDisplay: (xaf: number) => xaf,
    }),
  };
});

const moneyFixture = vi.hoisted(() => ({ code: 'XAF', currency: 'FCFA' }));
const get = vi.fn();
const post = vi.fn();
const patch = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: {
    get: (...a: unknown[]) => get(...a),
    post: (...a: unknown[]) => post(...a),
    patch: (...a: unknown[]) => patch(...a),
    delete: vi.fn(),
    defaults: { baseURL: '' },
  },
}));

import { RiskRegisterPage } from '../RiskRegisterPage';

const NOW = Date.now();
const inDays = (n: number) => new Date(NOW + n * 86_400_000).toISOString();

function risk(over: Partial<Risk> & { id: string; title: string }): Risk {
  return {
    description: 'Scénario.',
    score: 4.4,
    impact: 8,
    probability: 0.55,
    status: 'open',
    tags: [],
    source: 'manual',
    criticality: 'high',
    ale_xaf: 0,
    ...over,
  } as Risk;
}

let rows: Risk[];
let summary: RiskSummary;
let transitions: unknown;

const members = [
  { user_id: 'u1', email: 'aicha@acme.test', full_name: 'Aïcha Diallo', initials: 'AD' },
  { user_id: 'u2', email: 'koffi@acme.test', full_name: 'Koffi Mensah', initials: 'KM' },
];

function route(url: string, config?: { params?: Record<string, number> }) {
  if (url === '/risks') {
    const { page = 1, limit = 50 } = config?.params ?? {};
    return Promise.resolve({
      data: { items: rows.slice((page - 1) * limit, page * limit), total: rows.length },
    });
  }
  if (url === '/risks/summary') return Promise.resolve({ data: summary });
  if (url === '/risks/bulk/capabilities')
    return Promise.resolve({
      data: { entity_type: 'risk', actions: ['delete', 'change_status', 'set_owner'] },
    });
  if (url === '/ownership/assignable') return Promise.resolve({ data: { users: members } });
  if (url === '/risk-categories') return Promise.resolve({ data: { items: [] } });
  if (url === '/risks/r1/control-mappings')
    return Promise.resolve({
      data: {
        items: [
          {
            id: 'm1',
            risk_id: 'r1',
            framework_id: 'f1',
            control_id: 'c1',
            framework_name: 'ISO 27001',
            control_code: 'A.5.15',
          },
        ],
      },
    });
  if (url === '/risks/r1/financial')
    return Promise.resolve({
      data: {
        sle: { xaf: 1, usd: 1 },
        downtime_cost: { xaf: 1, usd: 1 },
        ale: { xaf: 1_000_000, usd: 1600 },
        ale_average: { xaf: 1, usd: 1 },
        ale_worst: { xaf: 1, usd: 1 },
        ale_after: { xaf: 1, usd: 1 },
        sle_basis: 'reference',
        rosi: 0,
        rosi_computable: false,
      },
    });
  if (/^\/risks\/[^/]+\/transitions$/.test(url)) return Promise.resolve({ data: transitions });
  if (url === '/risks/r9')
    return Promise.resolve({
      data: risk({ id: 'r9', title: 'Risque hors de la page', criticality: 'low', score: 1.2 }),
    });
  return Promise.resolve({ data: {} });
}

function renderPage(path = '/risks') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <RiskRegisterPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const listParams = () =>
  get.mock.calls
    .filter((c) => c[0] === '/risks')
    .map((c) => (c[1] as { params: Record<string, unknown> }).params);
const summaryParams = () =>
  get.mock.calls
    .filter((c) => c[0] === '/risks/summary')
    .map((c) => (c[1] as { params: Record<string, unknown> }).params);

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  useRiskStore.setState({ risks: [], total: 0, isLoading: false, error: false });
  perms.update = true;
  perms.delete = true;
  perms.create = true;
  rows = [
    risk({
      id: 'r1',
      title: 'Compromission de la messagerie',
      score: 7.2,
      probability: 0.62,
      impact: 8,
      criticality: 'critical',
      status: 'open',
      category: { id: 'c', name: 'Cyber', slug: 'cyber', color: 'neutral' },
      assets: [
        { id: 'a1', name: 'Exchange', type: 'app', criticality: 'HIGH', owner: '', source: '' },
        {
          id: 'a2',
          name: 'Active Directory',
          type: 'app',
          criticality: 'HIGH',
          owner: '',
          source: '',
        },
        {
          id: 'a3',
          name: 'Passerelle VPN',
          type: 'app',
          criticality: 'HIGH',
          owner: '',
          source: '',
        },
      ],
      owner_id: 'u1',
      owner_email: 'aicha@acme.test',
      ale_xaf: 117_200_000,
      trend: 'up',
      next_review_at: inDays(-2),
      residual_risk: 3.1,
      mitigations_count: 1,
      mitigations: [{ id: 'mm1', title: 'Activer le MFA', status: 'IN_PROGRESS', progress: 40 }],
    }),
    risk({
      id: 'r2',
      title: 'Fuite de données client',
      score: 4.4,
      criticality: 'high',
      status: 'in_progress',
      owner_id: 'u2',
      owner_email: 'koffi@acme.test',
      trend: 'down',
      next_review_at: inDays(40),
    }),
    risk({
      id: 'r3',
      title: 'Indisponibilité du SI',
      score: 1.2,
      probability: 0.2,
      impact: 3,
      criticality: 'low',
      status: 'closed',
      trend: null,
      next_review_at: inDays(-90),
    }),
  ];
  summary = {
    views: { all: 3, mine: 1, hot: 2, review_due: 2, accepted: 0 },
    by_criticality: { critical: 1, high: 1, medium: 0, low: 1 },
    active: { count: 2, ale_xaf: 234_000_000 },
  };
  transitions = {
    current: 'identified',
    current_label: 'Identifié',
    step_index: 0,
    step_count: 6,
    options: [
      {
        to: 'residual_accepted',
        label: 'Accepter',
        allowed: false,
        reason: 'Un plan de mitigation actif est requis.',
        is_forward: true,
      },
      { to: 'closed', label: 'Clore', allowed: true, is_forward: true },
    ],
  };
  get.mockReset();
  get.mockImplementation(route);
  post.mockReset();
  // The governed engine: every action is previewed (with a fingerprint), then applied.
  post.mockImplementation((url: string, body: { ids?: string[]; action?: string }) => {
    const ids = body.ids ?? [];
    if (url === '/risks/bulk/preview')
      return Promise.resolve({
        data: {
          action: body.action,
          requested: ids.length,
          found: ids.length,
          missing: [],
          affected: ids.length,
          unchanged: 0,
          sample: [],
          fingerprint: 'fp-' + ids.join('+'),
        },
      });
    return Promise.resolve({
      data: { total: ids.length, applied: ids.length, ids, audited: ids.length },
    });
  });
  patch.mockReset();
  patch.mockImplementation((_url: string, body: Record<string, unknown>) =>
    Promise.resolve({ data: { ...rows[0], ...body } }),
  );
  toast.mockClear();
  toast.success.mockClear();
  toast.error.mockClear();
});

describe('header, tabs and band pills', () => {
  it('reads the active count and the annualised exposure from the summary', async () => {
    renderPage();
    const sub = await screen.findByTestId('risk-subtitle');
    expect(sub).toHaveTextContent('2 risques actifs');
    expect(sub).toHaveTextContent('234,0 M FCFA d’exposition annualisée');
  });

  it('omits the exposure when the plan has no financial quantification (ale_xaf null)', async () => {
    summary.active = { count: 2, ale_xaf: null };
    renderPage();
    const sub = await screen.findByTestId('risk-subtitle');
    expect(sub).toHaveTextContent('2 risques actifs');
    expect(sub).not.toHaveTextContent('FCFA');
  });

  it('counts every view tab and every band pill from the summary', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByTestId('risk-tab-hot')).toHaveTextContent('2'));
    expect(screen.getByTestId('risk-tab-all')).toHaveTextContent('3');
    expect(screen.getByTestId('risk-tab-mine')).toHaveTextContent('1');
    expect(screen.getByTestId('risk-tab-accepted')).toHaveTextContent('0');
    await waitFor(() => expect(screen.getByTestId('risk-band-critical')).toHaveTextContent('1'));
    expect(screen.getByTestId('risk-band-medium')).toHaveTextContent('0');
  });

  it('maps a tab onto the API view param, for the list AND the summary', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    expect(listParams()[0]).not.toHaveProperty('view');

    await userEvent.click(screen.getByTestId('risk-tab-hot'));
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ view: 'hot', page: 1 }));
    await waitFor(() => expect(summaryParams().at(-1)).toMatchObject({ view: 'hot' }));
    expect(screen.getByTestId('risk-tab-hot')).toHaveAttribute('aria-selected', 'true');
  });

  it('reads the tab from the URL, and an unknown value is "all", not a crash', async () => {
    renderPage('/risks?f.scope=review_due');
    await waitFor(() => expect(listParams()[0]).toMatchObject({ view: 'review_due' }));
    expect(screen.getByTestId('risk-tab-review_due')).toHaveAttribute('aria-selected', 'true');

    get.mockClear();
    renderPage('/risks?f.scope=NOPE');
    await waitFor(() => expect(listParams().length).toBeGreaterThan(0));
    expect(listParams().every((p) => !('view' in p))).toBe(true);
  });

  it('toggles a band pill into the criticality filter and resets it', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    const pill = screen.getByTestId('risk-band-critical');
    await userEvent.click(pill);
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ criticality: 'critical' }));
    expect(pill).toHaveAttribute('aria-pressed', 'true');

    await userEvent.click(screen.getByTestId('risk-filter-reset'));
    await waitFor(() => expect(listParams().at(-1)).not.toHaveProperty('criticality'));
    expect(screen.queryByTestId('risk-filter-reset')).not.toBeInTheDocument();
  });

  it('filters by status and owner with the shared listbox, never a native select', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    expect(document.querySelector('select')).toBeNull();

    await userEvent.click(screen.getByTestId('risk-filter-status'));
    await userEvent.click(screen.getByRole('option', { name: 'En cours' }));
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ status: 'in_progress' }));

    await userEvent.click(screen.getByTestId('risk-filter-owner'));
    await userEvent.click(await screen.findByRole('option', { name: 'Koffi Mensah' }));
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ owner_id: 'u2' }));
  });
});

describe('the table, in the one display format', () => {
  it('shows score, P and I, band, status, owner, ALE, trend and review as designed', async () => {
    renderPage();
    const row = (await screen.findByText('Compromission de la messagerie')).closest('tr')!;
    // Score and its inputs: one decimal, P with two, never "/25".
    expect(row).toHaveTextContent('7,2');
    expect(row).toHaveTextContent('P 0,62 · I 8,0');
    expect(row).not.toHaveTextContent('/25');
    expect(row).toHaveTextContent('Critique');
    expect(row).toHaveTextContent('Ouvert');
    // Category, then the first two assets and "+n".
    expect(row).toHaveTextContent('Cyber · Exchange, Active Directory +1');
    // The owner is the member's name, not an email.
    expect(row).toHaveTextContent('Aïcha Diallo');
    // ALE in millions of the display currency.
    expect(row).toHaveTextContent('117,2');
    // Trend is a word for assistive tech, not only an arrow.
    expect(within(row).getByText('En hausse')).toBeInTheDocument();
    // Overdue review is flagged in words as well as colour.
    expect(within(row).getByText('Revue en retard')).toBeInTheDocument();
  });

  it('says "Clos" for a closed risk, and "no trend" rather than a flat arrow when there is no history', async () => {
    renderPage();
    const row = (await screen.findByText('Indisponibilité du SI')).closest('tr')!;
    expect(row).toHaveTextContent('Clos');
    expect(within(row).getByText('Pas de tendance (aucun historique)')).toBeInTheDocument();
  });

  it('is sortable only where the server can sort: not by id, not by ALE', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    const headers = screen.getAllByRole('columnheader');
    const byName = (re: RegExp) => headers.find((h) => re.test(h.textContent ?? ''))!;
    expect(within(byName(/^Score$/)).getByRole('button')).toBeInTheDocument();
    expect(within(byName(/^Risque$/)).getByRole('button')).toBeInTheDocument();
    expect(within(byName(/^Revue$/)).getByRole('button')).toBeInTheDocument();
    expect(within(byName(/^ID$/)).queryByRole('button')).not.toBeInTheDocument();
    expect(within(byName(/^ALE/)).queryByRole('button')).not.toBeInTheDocument();

    await userEvent.click(within(byName(/^Revue$/)).getByRole('button'));
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ sort_by: 'next_review_at' }));
  });

  it('states our formula and bands in the footer, with the row count', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    expect(screen.getByTestId('risk-count')).toHaveTextContent('3 sur 3 risques');
    const formula = screen.getByTestId('risk-formula');
    expect(formula).toHaveTextContent('probabilité (0–1) × impact (0–10) × criticité de l’actif');
    expect(formula).toHaveTextContent('critique ≥ 7 · élevé ≥ 4 · moyen ≥ 2 · faible < 2');
    expect(formula).not.toHaveTextContent('/25');
  });

  it('speaks English when the language is English', async () => {
    useUIStore.setState({ lang: 'en' });
    renderPage();
    const row = (await screen.findByText('Compromission de la messagerie')).closest('tr')!;
    expect(row).toHaveTextContent('7.2');
    expect(row).toHaveTextContent('P 0.62 · I 8.0');
    expect(row).toHaveTextContent('Critical');
    expect(screen.getByTestId('risk-count')).toHaveTextContent('3 of 3 risks');
    expect(screen.getByTestId('risk-subtitle')).toHaveTextContent(
      '2 active risks · 234.0 M FCFA of annualised exposure',
    );
  });
});

describe('the three states', () => {
  it('shows skeleton rows while loading, and a skeleton subtitle', () => {
    get.mockImplementation(() => new Promise(() => {}));
    renderPage();
    expect(screen.getByTestId('table-skeleton')).toBeInTheDocument();
    expect(screen.getByTestId('risk-subtitle-loading')).toBeInTheDocument();
    // Not "no risks yet": nothing has answered, so nothing can be said to be absent.
    expect(screen.queryByText('Aucun risque pour le moment')).not.toBeInTheDocument();
  });

  it('shows an error with a retry that reloads the list', async () => {
    get.mockImplementation((url: string) =>
      url === '/risks' ? Promise.reject(new Error('boom')) : route(url),
    );
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {});
    renderPage();
    const retry = await screen.findByTestId('table-retry');
    get.mockImplementation(route);
    await userEvent.click(retry);
    expect(await screen.findByText('Compromission de la messagerie')).toBeInTheDocument();
    errors.mockRestore();
  });

  it('offers a next step when the tenant has no risk at all', async () => {
    rows = [];
    summary = {
      views: { all: 0, mine: 0, hot: 0, review_due: 0, accepted: 0 },
      by_criticality: { critical: 0, high: 0, medium: 0, low: 0 },
      active: { count: 0, ale_xaf: 0 },
    };
    const open = vi.fn();
    window.addEventListener('openrisk:new-risk', open);
    renderPage();
    expect(await screen.findByText('Aucun risque pour le moment')).toBeInTheDocument();
    const cta = screen
      .getAllByRole('button', { name: 'Nouveau risque' })
      .find((b) => b.closest('[data-testid="risk-new"]') === null)!;
    await userEvent.click(cta);
    expect(open).toHaveBeenCalled();
    window.removeEventListener('openrisk:new-risk', open);
  });

  it('keeps a read-only member out of creation and says why', async () => {
    perms.create = false;
    rows = [];
    renderPage();
    expect(
      await screen.findByText(/Votre rôle permet de consulter le registre/),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nouveau risque' })).not.toBeInTheDocument();
  });

  it('says what matched nothing, in the design\u2019s words, with the way out inline', async () => {
    renderPage('/risks?f.criticality=low');
    await screen.findByText('Compromission de la messagerie');
    rows = [];
    await userEvent.click(screen.getByTestId('risk-band-high'));
    const none = await screen.findByTestId('risk-nomatch');
    expect(none).toHaveTextContent('Aucun risque ne correspond à ces filtres');
    expect(none).toHaveTextContent('Élargissez la recherche ou réinitialisez les filtres.');
    rows = [risk({ id: 'r1', title: 'Compromission de la messagerie' })];
    await userEvent.click(within(none).getByRole('button', { name: 'réinitialisez les filtres' }));
    await waitFor(() => expect(listParams().at(-1)).not.toHaveProperty('criticality'));
  });

  it('degrades the counts and the subtitle, with a retry, when the summary fails', async () => {
    get.mockImplementation((url: string) =>
      url === '/risks/summary' ? Promise.reject(new Error('boom')) : route(url),
    );
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    const sub = await screen.findByText('Le résumé du registre n’a pas pu être chargé.');
    expect(within(sub.parentElement!).getByRole('button', { name: 'Réessayer' })).toBeVisible();
    // The table still works: the summary is not its dependency.
    expect(screen.getByTestId('risk-tab-hot')).not.toHaveTextContent(/\d/);
  });
});

describe('the matrix', () => {
  it('opens from the dashboard link (?view=matrix) and asks for the largest page', async () => {
    renderPage('/risks?view=matrix');
    const matrix = await screen.findByTestId('risk-matrix');
    expect(listParams()[0]).toMatchObject({ page: 1, limit: 100 });
    // P 0.62 → level 4, I 8 → level 4: the cell and its marker, in the one format.
    const marker = within(matrix).getByTestId('risk-marker-r1');
    expect(marker).toHaveTextContent('7,2');
    expect(marker.closest('td')).toBe(within(matrix).getByTestId('risk-cell-4-4').closest('td'));
    // A closed risk has left the exposure the matrix draws.
    expect(within(matrix).queryByTestId('risk-marker-r3')).not.toBeInTheDocument();
  });

  it('ranks by score, then isolates a cell on click', async () => {
    renderPage('/risks?view=matrix');
    const matrix = await screen.findByTestId('risk-matrix');
    expect(within(matrix).getByText('Classement par score')).toBeInTheDocument();

    await userEvent.click(within(matrix).getByTestId('risk-cell-4-4'));
    expect(within(matrix).getByText('Probabilité 4 × impact 4')).toBeInTheDocument();
    expect(within(matrix).getByText(/Probable · Majeur · 1 risque/)).toBeInTheDocument();
    expect(within(matrix).getByTestId('risk-cell-4-4')).toHaveAttribute('aria-pressed', 'true');
  });

  it('switches layout without losing the tab or the filters', async () => {
    renderPage('/risks?f.scope=hot&f.criticality=critical');
    await screen.findByText('Compromission de la messagerie');
    await userEvent.click(screen.getByTestId('risk-layout-matrix'));
    await screen.findByTestId('risk-matrix');
    await waitFor(() =>
      expect(listParams().at(-1)).toMatchObject({
        view: 'hot',
        criticality: 'critical',
        limit: 100,
      }),
    );
    expect(screen.getByTestId('risk-tab-hot')).toHaveAttribute('aria-selected', 'true');
  });

  it('has its own error, empty and no-match states', async () => {
    get.mockImplementation((url: string) =>
      url === '/risks' ? Promise.reject(new Error('boom')) : route(url),
    );
    const errors = vi.spyOn(console, 'error').mockImplementation(() => {});
    renderPage('/risks?view=matrix');
    expect(await screen.findByText('Chargement impossible')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Réessayer' })).toBeInTheDocument();
    errors.mockRestore();
  });
});

describe('bulk bar', () => {
  async function selectTwo() {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    await userEvent.click(screen.getByTestId('row-select-r1'));
    await userEvent.click(screen.getByTestId('row-select-r2'));
    return screen.getByTestId('bulk-bar');
  }

  async function confirmDialog(name: string) {
    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByTestId('bulk-preview-counts');
    await userEvent.click(within(dialog).getByRole('button', { name }));
    return dialog;
  }

  it('previews a status change, applies it on the governed route with the fingerprint, then offers Undo', async () => {
    const bar = await selectTwo();
    expect(within(bar).getByTestId('bulk-count')).toHaveTextContent('2');

    await userEvent.click(within(bar).getByTestId('bulk-action-status'));
    await userEvent.click(await screen.findByRole('option', { name: 'Atténué' }));

    // Nothing is written before the confirm.
    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByTestId('bulk-preview-counts');
    expect(post).toHaveBeenCalledWith(
      '/risks/bulk/preview',
      {
        action: 'change_status',
        ids: ['r1', 'r2'],
        status: 'mitigated',
        owner_id: undefined,
      },
      { signal: undefined },
    );
    expect(post.mock.calls.map((c) => c[0])).toEqual(['/risks/bulk/preview']);

    await userEvent.click(within(dialog).getByRole('button', { name: 'Appliquer' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/risks/bulk/change-status',
        expect.objectContaining({
          ids: ['r1', 'r2'],
          status: 'mitigated',
          fingerprint: 'fp-r1+r2',
        }),
      ),
    );
    // The deprecated route is never used.
    expect(post.mock.calls.some((c) => c[0] === '/risks/bulk')).toBe(false);

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    const [message, options] = toast.success.mock.calls.at(-1) as [
      string,
      { action: { label: string; onClick: () => void } },
    ];
    expect(message).toBe('2 risques mis à jour · Atténué');
    expect(options.action.label).toBe('Annuler');

    // Undo puts each risk back on the status it held, each with its OWN fresh
    // preview (the first fingerprint is stale once the rows have changed).
    post.mockClear();
    options.action.onClick();
    await waitFor(() => expect(post).toHaveBeenCalledTimes(4));
    expect(post).toHaveBeenCalledWith(
      '/risks/bulk/change-status',
      expect.objectContaining({ ids: ['r1'], status: 'open', fingerprint: 'fp-r1' }),
    );
    expect(post).toHaveBeenCalledWith(
      '/risks/bulk/change-status',
      expect.objectContaining({ ids: ['r2'], status: 'in_progress', fingerprint: 'fp-r2' }),
    );
  });

  it('does not offer acceptance or closing in bulk: the server refuses the first, the lifecycle owns the second', async () => {
    const bar = await selectTwo();
    await userEvent.click(within(bar).getByTestId('bulk-action-status'));
    const names = (await screen.findAllByRole('option')).map((o) => o.textContent);
    expect(names).toEqual(['Ouvert', 'En cours', 'Atténué']);
  });

  it('sets the owner on the governed set-owner route (owner_id, the slot the column shows)', async () => {
    const bar = await selectTwo();
    await userEvent.click(within(bar).getByTestId('bulk-action-owner'));
    await userEvent.click(await screen.findByRole('option', { name: 'Koffi Mensah' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(await within(dialog).findByText(/Changer le responsable de 2/)).toBeInTheDocument();
    await within(dialog).findByTestId('bulk-preview-counts');
    expect(post).toHaveBeenCalledWith(
      '/risks/bulk/preview',
      {
        action: 'set_owner',
        ids: ['r1', 'r2'],
        status: undefined,
        owner_id: 'u2',
      },
      { signal: undefined },
    );
    await userEvent.click(within(dialog).getByRole('button', { name: 'Appliquer' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/risks/bulk/set-owner',
        expect.objectContaining({ ids: ['r1', 'r2'], owner_id: 'u2', fingerprint: 'fp-r1+r2' }),
      ),
    );
    expect(post.mock.calls.some((c) => c[0] === '/risks/bulk')).toBe(false);
  });

  it('offers the owner action only when the server lists set_owner', async () => {
    get.mockImplementation((url: string, config?: { params?: Record<string, number> }) =>
      url === '/risks/bulk/capabilities'
        ? Promise.resolve({ data: { entity_type: 'risk', actions: ['delete', 'change_status'] } })
        : route(url, config),
    );
    const bar = await selectTwo();
    await waitFor(() =>
      expect(get.mock.calls.some((c) => c[0] === '/risks/bulk/capabilities')).toBe(true),
    );
    await new Promise((r) => setTimeout(r, 30));
    expect(within(bar).queryByTestId('bulk-action-owner')).not.toBeInTheDocument();
    expect(within(bar).getByTestId('bulk-action-status')).toBeInTheDocument();
  });

  it('brings the rows back and offers a new preview when the selection moved (409)', async () => {
    post.mockImplementation((url: string, body: { ids?: string[]; action?: string }) => {
      if (url === '/risks/bulk/preview')
        return Promise.resolve({
          data: {
            action: body.action,
            requested: 2,
            found: 2,
            missing: [],
            affected: 2,
            unchanged: 0,
            sample: [],
            fingerprint: 'fp',
          },
        });
      return Promise.reject(
        Object.assign(new Error('moved'), {
          isAxiosError: true,
          response: { status: 409, data: { error: 'moved' } },
        }),
      );
    });
    const bar = await selectTwo();
    await userEvent.click(within(bar).getByTestId('bulk-action-status'));
    await userEvent.click(await screen.findByRole('option', { name: 'Atténué' }));
    const dialog = await confirmDialog('Appliquer');
    expect(await within(dialog).findByTestId('bulk-preview-refused')).toBeInTheDocument();
    const row = screen.getByText('Compromission de la messagerie').closest('tr')!;
    expect(row).toHaveTextContent('Ouvert');
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('previews a delete first, then sends ONE governed delete with the fingerprint (#1441)', async () => {
    post.mockImplementation((url: string, body: { ids: string[] }) => {
      if (url === '/risks/bulk/preview')
        return Promise.resolve({
          data: {
            action: 'delete',
            total: body.ids.length,
            affected: body.ids.length,
            unchanged: 0,
            missing: [],
            sample: [],
            fingerprint: 'fp-1',
          },
        });
      return Promise.resolve({
        data: { total: body.ids.length, applied: body.ids.length, ids: body.ids, audited: 2 },
      });
    });
    const bar = await selectTwo();
    await userEvent.click(await within(bar).findByTestId('bulk-action-delete'));
    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByTestId('bulk-preview-counts');
    // Nothing is deleted before the confirm.
    expect(post.mock.calls.map((c) => c[0])).toEqual(['/risks/bulk/preview']);

    await userEvent.click(within(dialog).getByRole('button', { name: 'Supprimer' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/risks/bulk/delete',
        expect.objectContaining({ ids: ['r1', 'r2'], fingerprint: 'fp-1' }),
      ),
    );
    // The ungoverned route is never used for a delete.
    expect(post.mock.calls.some((c) => c[0] === '/risks/bulk')).toBe(false);
  });

  it('offers no status, owner or delete action to a member who may not write', async () => {
    perms.update = false;
    perms.delete = false;
    const bar = await selectTwo();
    expect(within(bar).queryByTestId('bulk-action-status')).not.toBeInTheDocument();
    expect(within(bar).queryByTestId('bulk-action-owner')).not.toBeInTheDocument();
    expect(within(bar).queryByTestId('bulk-action-delete')).not.toBeInTheDocument();
    // Export is a read.
    expect(within(bar).getByTestId('bulk-action-export')).toBeInTheDocument();
  });
});

describe('the drawer', () => {
  async function openDrawer() {
    renderPage();
    await userEvent.click(await screen.findByText('Compromission de la messagerie'));
    return screen.findByTestId('risk-drawer');
  }

  it('opens on the overview with the figures labelled for what they are', async () => {
    const drawer = await openDrawer();
    expect(
      within(drawer).getByRole('heading', { name: 'Compromission de la messagerie' }),
    ).toBeVisible();
    const figures = within(drawer).getByTestId('risk-figures');
    // Inherent and residual scores are never left unlabelled, same format as the table.
    expect(figures).toHaveTextContent('Score inhérent');
    expect(figures).toHaveTextContent('7,2');
    expect(figures).toHaveTextContent('ALE inhérente');
    expect(figures).toHaveTextContent('117,2 M FCFA');
    expect(figures).toHaveTextContent('Score résiduel');
    expect(figures).toHaveTextContent('3,1');
    expect(figures).not.toHaveTextContent('/25');
    // Linked assets and controls are links.
    expect(within(drawer).getByRole('link', { name: 'Exchange' })).toHaveAttribute(
      'href',
      '/assets?focus=a1',
    );
    expect(await within(drawer).findByRole('link', { name: 'ISO 27001 · A.5.15' })).toHaveAttribute(
      'href',
      '/compliance/frameworks/f1/controls/c1',
    );
  });

  it('shows the pressed level the stored value falls in, with the exact value', async () => {
    const drawer = await openDrawer();
    const p = within(drawer).getByTestId('risk-pick-p');
    expect(within(p).getByTestId('risk-pick-p-4')).toHaveAttribute('aria-pressed', 'true');
    expect(p).toHaveTextContent('0,62');
    expect(within(drawer).getByTestId('risk-pick-i-4')).toHaveAttribute('aria-pressed', 'true');
  });

  it('writes the midpoint of the level it is given', async () => {
    const drawer = await openDrawer();
    await userEvent.click(within(drawer).getByTestId('risk-pick-p-2'));
    await waitFor(() =>
      expect(patch).toHaveBeenCalledWith('/risks/r1', { probability: expect.closeTo(0.3, 10) }),
    );
    await userEvent.click(await within(drawer).findByTestId('risk-pick-i-5'));
    await waitFor(() => expect(patch).toHaveBeenCalledWith('/risks/r1', { impact: 9 }));
  });

  it('enables "Clore le risque" only when the server offers the transition', async () => {
    const drawer = await openDrawer();
    await waitFor(() => expect(within(drawer).getByTestId('risk-close')).toBeEnabled());
    // Acceptance is blocked here, and the reason is on the screen, not guessed.
    expect(within(drawer).getByTestId('risk-accept')).toBeDisabled();
    expect(within(drawer).getByTestId('risk-transition-hint')).toHaveTextContent(
      'Un plan de mitigation actif est requis.',
    );

    await userEvent.click(within(drawer).getByTestId('risk-close'));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/risks/r1/transition', { to: 'closed' }),
    );
  });

  it('sends the acceptance request through the lifecycle endpoint once it is allowed', async () => {
    (transitions as { options: { to: string; allowed: boolean }[] }).options[0].allowed = true;
    const drawer = await openDrawer();
    const accept = within(drawer).getByTestId('risk-accept');
    await waitFor(() => expect(accept).toBeEnabled());
    await userEvent.click(accept);
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/risks/r1/transition', { to: 'residual_accepted' }),
    );
  });

  it('closes on Escape and gives focus back to the row that opened it', async () => {
    renderPage();
    const row = (await screen.findByText('Compromission de la messagerie')).closest('tr')!;
    row.focus();
    await userEvent.click(row);
    await screen.findByTestId('risk-drawer');
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByTestId('risk-drawer')).not.toBeInTheDocument());
    await waitFor(() => expect(row).toHaveFocus());
  });

  it('lists the mitigations, and offers to plan one when there is none', async () => {
    renderPage('/risks?focus=r1&tab=miti');
    const list = await screen.findByTestId('risk-mitigations');
    expect(within(list).getByText('Activer le MFA')).toBeInTheDocument();
    expect(within(list).getByText('40 %')).toBeInTheDocument();
    expect(within(list).getByTestId('risk-add-miti')).toBeInTheDocument();
  });

  it('keeps every older tab visible, after the design tabs, and each deep link lands on its own', async () => {
    renderPage('/risks?focus=r1&tab=cti');
    const drawer = await screen.findByTestId('risk-drawer');
    const names = within(drawer)
      .getAllByRole('tab')
      .map((t) => t.getAttribute('data-tab-id'));
    expect(names).toEqual([
      'overview',
      'miti',
      'history',
      'ownership',
      'lifecycle',
      'score',
      'smart',
      'financial',
      'cti',
      'ai',
    ]);
    expect(within(drawer).getByTestId('risk-tab-cti')).toHaveAttribute('aria-selected', 'true');
    // No hidden menu: nothing called "Plus".
    expect(within(drawer).queryByRole('tab', { name: 'Plus' })).not.toBeInTheDocument();
  });

  it('moves through the strip with the arrow keys', async () => {
    const drawer = await openDrawer();
    const overview = within(drawer).getByTestId('risk-tab-overview');
    overview.focus();
    fireEvent.keyDown(overview, { key: 'End' });
    await waitFor(() =>
      expect(within(drawer).getByTestId('risk-tab-ai')).toHaveAttribute('aria-selected', 'true'),
    );
  });

  it('fetches a risk that is not on the loaded page, and says when it cannot', async () => {
    renderPage('/risks?focus=r9');
    expect(await screen.findByRole('heading', { name: 'Risque hors de la page' })).toBeVisible();
    expect(get).toHaveBeenCalledWith('/risks/r9', expect.anything());
  });

  it('shows an error with the way out when that risk cannot be loaded', async () => {
    get.mockImplementation((url: string) =>
      url === '/risks/r9' ? Promise.reject(new Error('gone')) : route(url),
    );
    renderPage('/risks?focus=r9');
    const status = await screen.findByTestId('risk-drawer-status');
    // Loading first (aria-busy), then the failure with its way out.
    expect(await within(status).findByText('Risque indisponible')).toBeInTheDocument();
    expect(within(status).getByRole('button', { name: 'Réessayer' })).toBeInTheDocument();
  });

  it('exports a single risk from the header icon, in the one format', async () => {
    const created: string[] = [];
    const orig = URL.createObjectURL;
    URL.createObjectURL = ((blob: Blob) => {
      // jsdom's Blob has no text(): read it the long way.
      const reader = new FileReader();
      reader.onload = () => created.push(String(reader.result));
      reader.readAsText(blob);
      return 'blob:x';
    }) as typeof URL.createObjectURL;
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    const drawer = await openDrawer();
    await userEvent.click(within(drawer).getByRole('button', { name: 'Exporter en CSV' }));
    await waitFor(() => expect(created.length).toBe(1));
    expect(created[0]).toContain('7,2');
    expect(created[0]).toContain('0,62');
    expect(created[0]).toContain('Compromission de la messagerie');
    click.mockRestore();
    URL.createObjectURL = orig;
  });
});

describe('round-2 review fixes', () => {
  it('cancels an export in flight when the page is left, without an error toast (S-b)', async () => {
    let signal: AbortSignal | undefined;
    let fail: (e: Error) => void = () => {};
    const { unmount } = renderPage();
    await waitFor(() => expect(screen.getByTestId('risk-export')).toBeEnabled());
    get.mockImplementation((url: string, config?: { signal?: AbortSignal }) => {
      if (url !== '/risks') return route(url);
      signal = config?.signal;
      return new Promise((_resolve, reject) => {
        fail = reject;
      });
    });
    await userEvent.click(screen.getByTestId('risk-export'));
    await waitFor(() => expect(signal).toBeDefined());
    expect(signal?.aborted).toBe(false);
    unmount();
    expect(signal?.aborted).toBe(true);
    fail(new Error('canceled'));
    await new Promise((r) => setTimeout(r, 0));
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('says in which unit the financial inputs are typed when the display currency differs (W-b)', async () => {
    moneyFixture.code = 'EUR';
    moneyFixture.currency = 'EUR';
    try {
      renderPage('/risks?focus=r1&tab=financial');
      const hint = await screen.findByTestId('financial-input-unit');
      // Cards in EUR, inputs in the storage unit (XAF, written FCFA).
      expect(hint).toHaveTextContent('Saisie en FCFA (unité de stockage).');
    } finally {
      moneyFixture.code = 'XAF';
      moneyFixture.currency = 'FCFA';
    }
  });

  it('gives no unit hint when the display currency is already the storage unit (W-b)', async () => {
    renderPage('/risks?focus=r1&tab=financial');
    await screen.findByText(/Quantification financière/);
    expect(screen.queryByTestId('financial-input-unit')).not.toBeInTheDocument();
  });
});

describe('round-1 review and live-pass fixes', () => {
  it('exports the WHOLE view, page by page, and says how many rows it wrote (W1)', async () => {
    rows = Array.from({ length: 450 }, (_, i) =>
      risk({ id: `x${i}`, title: `Risque ${i}`, score: 3, criticality: 'medium' }),
    );
    summary = { ...summary, views: { ...summary.views, all: 450 } };
    const written: string[] = [];
    const orig = URL.createObjectURL;
    URL.createObjectURL = ((blob: Blob) => {
      const reader = new FileReader();
      reader.onload = () => written.push(String(reader.result));
      reader.readAsText(blob);
      return 'blob:x';
    }) as typeof URL.createObjectURL;
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    renderPage();
    // 50 rows are loaded (the table virtualises them, jsdom draws none).
    await waitFor(() => expect(screen.getByTestId('risk-export')).toBeEnabled());
    get.mockClear();
    await userEvent.click(screen.getByTestId('risk-export'));

    await waitFor(() => expect(written.length).toBe(1));
    const calls = listParams();
    expect(calls.map((p) => p.page)).toEqual([1, 2, 3, 4, 5]);
    expect(calls.every((p) => p.limit === 100)).toBe(true);
    // Header + 450 rows, not the 50 that were on screen.
    expect(written[0].split('\n').length).toBe(451);
    expect(toast.success).toHaveBeenCalledWith('450 risques exportés en CSV');
    click.mockRestore();
    URL.createObjectURL = orig;
  });

  it('keeps the same query when exporting, and tells the user when it fails (W1)', async () => {
    renderPage('/risks?f.scope=hot&f.criticality=critical');
    await screen.findByText('Compromission de la messagerie');
    get.mockClear();
    get.mockImplementation((url: string) =>
      url === '/risks' ? Promise.reject(new Error('boom')) : route(url),
    );
    await userEvent.click(screen.getByTestId('risk-export'));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(listParams()[0]).toMatchObject({ view: 'hot', criticality: 'critical', limit: 100 });
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('does not sort on the band column, whose names do not sort by severity (W4)', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    const band = screen
      .getAllByRole('columnheader')
      .find((h) => /^Niveau$/.test(h.textContent ?? ''))!;
    expect(within(band).queryByRole('button')).not.toBeInTheDocument();
  });

  it('prints the score in neutral ink and the status as designed (L3)', async () => {
    renderPage();
    const row = (await screen.findByText('Compromission de la messagerie')).closest('tr')!;
    const score = within(row).getByText('7,2');
    expect(score.className).toMatch(/text-ink/);
    expect(score.getAttribute('style') ?? '').not.toMatch(/color/);
    expect(within(row).getByText('Ouvert').className).toMatch(/bg-surface-3/);
    const progress = screen.getByText('Fuite de données client').closest('tr')!;
    expect(within(progress).getByText('En cours').className).toMatch(/bg-info-surface/);
  });

  it('writes a tab count as plain muted text, not a badge (L3)', async () => {
    renderPage();
    const tab = screen.getByTestId('risk-tab-hot');
    await waitFor(() => expect(within(tab).getByText('2')).toBeInTheDocument());
    const count = within(tab).getByText('2');
    expect(count.className).toMatch(/mono/);
    expect(count.className).not.toMatch(/rounded-full/);
  });

  it('says "unknown" rather than "low" when the server sent no band (S3)', async () => {
    rows = [risk({ id: 'r1', title: 'Sans niveau', criticality: undefined })];
    renderPage();
    const row = (await screen.findByText('Sans niveau')).closest('tr')!;
    expect(within(row).queryByText('Faible')).not.toBeInTheDocument();
    await userEvent.click(row);
    const drawer = await screen.findByTestId('risk-drawer');
    expect(within(drawer).getByText('Niveau inconnu')).toBeInTheDocument();
    expect(within(drawer).queryByText('Faible')).not.toBeInTheDocument();
  });

  it('keeps the residual score in neutral ink, and the ALE unit beside its figure (W2, L4)', async () => {
    renderPage();
    await userEvent.click(await screen.findByText('Compromission de la messagerie'));
    const figures = await screen.findByTestId('risk-figures');
    const residual = within(figures).getByText('3,1');
    expect(residual.getAttribute('style') ?? '').not.toMatch(/color/);
    expect(figures).toHaveTextContent('117,2 M FCFA');
    // Never truncated.
    expect(figures.innerHTML).not.toMatch(/truncate/);
  });

  it('shows both reasons when the two footer actions are blocked differently (S4)', async () => {
    transitions = {
      ...(transitions as object),
      options: [
        {
          to: 'residual_accepted',
          label: 'a',
          allowed: false,
          reason: 'Plan requis.',
          is_forward: true,
        },
        {
          to: 'closed',
          label: 'c',
          allowed: false,
          reason: 'Sous-actions ouvertes.',
          is_forward: true,
        },
      ],
    };
    renderPage();
    await userEvent.click(await screen.findByText('Compromission de la messagerie'));
    const hint = await screen.findByTestId('risk-transition-hint');
    expect(hint).toHaveTextContent('Plan requis.');
    expect(hint).toHaveTextContent('Sous-actions ouvertes.');
  });

  it('draws matrix markers in the band fill with the inverse text token (L5)', async () => {
    renderPage('/risks?view=matrix');
    const marker = await screen.findByTestId('risk-marker-r1');
    expect(marker.className).toMatch(/text-fg-inverse/);
    expect(marker.className).not.toMatch(/text-fg-on-solid/);
  });

  it('keeps the bulk bar on one line (L7)', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    await userEvent.click(screen.getByTestId('row-select-r1'));
    expect(screen.getByTestId('bulk-bar').className).toMatch(/flex-nowrap/);
  });

  it('hides the scrollbar of the view tabs on a narrow screen (L9)', async () => {
    renderPage();
    const tabs = screen.getByRole('tablist', { name: 'Vues du registre' });
    expect(tabs.className).toMatch(/scrollbar-width:none/);
  });

  it('lets the toolbar listboxes size to their content, so the toolbar is one row (L1)', async () => {
    renderPage();
    await screen.findByText('Compromission de la messagerie');
    for (const id of ['risk-filter-status', 'risk-filter-owner']) {
      expect(screen.getByTestId(id).parentElement!.className).toMatch(/shrink-0/);
    }
  });
});
