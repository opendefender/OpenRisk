// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The vulnerability queue (#909): figures, severity pills that count under the
// queue's own filters, the two switches, rows that open in place, and the
// status change from an open row.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import type { Vulnerability, VulnStats } from '../vulnerabilityService';
import { epssPercent, nextStatuses, prioritySignals, slaState } from '../vulnQueue';

const perms = { write: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({
      hasPermission: (p: string) =>
        p === 'vulnerabilities:update' || p === 'vulnerabilities:delete' ? perms.write : true,
    }),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

// The risk form has its own tests; here it only has to open with the finding.
vi.mock('../../risks/CreateRiskModal', () => ({
  CreateRiskModal: ({
    initialTitle,
    initialAssetIds,
  }: {
    initialTitle?: string;
    initialAssetIds?: string[];
  }) => (
    <div
      role="dialog"
      aria-label={`create-risk:${initialTitle}:${(initialAssetIds ?? []).join(',')}`}
    />
  ),
}));

const get = vi.fn();
const patch = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: {
    get: (...a: unknown[]) => get(...a),
    patch: (...a: unknown[]) => patch(...a),
    post: vi.fn(),
    delete: vi.fn(),
    defaults: { baseURL: '' },
  },
}));

import { VulnerabilitiesPage } from '../VulnerabilitiesPage';

const NOW = new Date();
const inDays = (n: number) => new Date(NOW.getTime() + n * 86_400_000).toISOString();

function vuln(over: Partial<Vulnerability>): Vulnerability {
  return {
    id: 'v1',
    cve_id: 'CVE-2026-21873',
    title: 'Exécution de code à distance — SSL-VPN',
    description: 'Un débordement dans le portail SSL-VPN.',
    cvss_score: 9.8,
    cvss_vector: 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H',
    severity: 'critical',
    epss: 0.94,
    kev: true,
    exploit_available: true,
    exploit_maturity: '',
    priority_score: 92,
    priority_tier: 'P1',
    asset_id: 'a1',
    asset_name: 'Pare-feu périmétrique',
    asset_criticality: 'CRITICAL',
    affected_assets_count: 1,
    source: 'nessus',
    external_id: '',
    status: 'open',
    remediation_hint: 'Appliquer le correctif éditeur.',
    first_seen: inDays(-6),
    last_seen: inDays(0),
    sla_due_at: inDays(1),
    remediated_at: null,
    created_at: inDays(-6),
    updated_at: inDays(0),
    match_confidence: 1,
    match_ambiguous: false,
    ...over,
  };
}

const stats: VulnStats = {
  total: 9,
  open: 6,
  by_severity: {},
  by_status: {},
  by_source: {},
  by_tier: {},
  kev_count: 3,
  kev_open: 3,
  exploit_count: 4,
  unresolved: 8,
  sla_breached: 1,
  mttr_critical_days: 3.4,
  mttr_critical_sample: 5,
  mttr_window_days: 90,
};

let rows: Vulnerability[];

function route(url: string, config?: { params?: Record<string, unknown> }) {
  if (url === '/vulnerabilities/stats') return Promise.resolve({ data: stats });
  if (url === '/vulnerabilities') {
    return Promise.resolve({
      data: {
        items: rows,
        total: rows.length,
        page: 1,
        limit: 50,
        severity_counts: { critical: 2, high: 3, medium: 3, low: 1 },
        _params: config?.params,
      },
    });
  }
  if (url.includes('capabilities'))
    return Promise.resolve({ data: { actions: ['change_status', 'delete'] } });
  if (url === '/scanner/configs') return Promise.resolve({ data: [] });
  if (url === '/vulnerability-connectors') return Promise.resolve({ data: { connectors: [] } });
  return Promise.resolve({ data: {} });
}

function renderPage(path = '/vulnerabilities') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <VulnerabilitiesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const listParams = () =>
  get.mock.calls
    .filter((c) => c[0] === '/vulnerabilities')
    .map((c) => (c[1] as { params: Record<string, unknown> }).params);

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  perms.write = true;
  rows = [
    vuln({}),
    vuln({
      id: 'v2',
      cve_id: 'CVE-2025-55241',
      title: 'Délégation Kerberos non contrainte',
      severity: 'high',
      cvss_score: 8.1,
      epss: 0.33,
      kev: true,
      asset_id: null,
      asset_name: 'Contrôleur de domaine AD',
      sla_due_at: inDays(-2),
    }),
    vuln({
      id: 'v3',
      cve_id: 'CVE-2026-17760',
      title: 'XSS stocké — portail RH',
      severity: 'medium',
      kev: false,
      status: 'accepted',
      epss: 0.04,
    }),
  ];
  get.mockReset();
  get.mockImplementation(route);
  patch.mockReset();
});

describe('VulnerabilitiesPage', () => {
  it('shows the four figures, MTTR with what it is measured on', async () => {
    renderPage();
    const kpis = await screen.findByTestId('vuln-kpis');
    await waitFor(() => expect(within(kpis).getByTestId('vuln-kpi-open')).toHaveTextContent('8'));
    expect(within(kpis).getByTestId('vuln-kpi-kev')).toHaveTextContent('3');
    expect(within(kpis).getByTestId('vuln-kpi-sla')).toHaveTextContent('1');
    expect(within(kpis).getByTestId('vuln-kpi-mttr')).toHaveTextContent('3,4 j');
    expect(within(kpis).getByTestId('vuln-kpi-mttr')).toHaveTextContent(
      'sur 5 critiques corrigées en 90 j',
    );
  });

  it('says there is no MTTR rather than showing zero days', async () => {
    stats.mttr_critical_days = null;
    stats.mttr_critical_sample = 0;
    try {
      renderPage();
      const tile = await screen.findByTestId('vuln-kpi-mttr');
      await waitFor(() => expect(tile).toHaveTextContent('Aucune critique corrigée en 90 j'));
      // "90 j" is the window, not a figure: only a standalone zero would be wrong.
      expect(tile).not.toHaveTextContent(/(^|\D)0 j/);
    } finally {
      stats.mttr_critical_days = 3.4;
      stats.mttr_critical_sample = 5;
    }
  });

  it('hides fixed findings by default and shows them on request', async () => {
    renderPage();
    await screen.findByText('Délégation Kerberos non contrainte');
    expect(listParams()[0]).toMatchObject({ status: 'open,triaged,in_remediation,accepted' });

    fireEvent.click(screen.getByRole('switch', { name: /Inclure corrigées/ }));
    await waitFor(() => expect(listParams().at(-1)?.status).toBeUndefined());
  });

  it('counts each severity pill and filters on it, KEV switch included', async () => {
    renderPage();
    const pill = await screen.findByTestId('vuln-sev-high');
    await waitFor(() => expect(pill).toHaveTextContent('Élevée3'));
    fireEvent.click(pill);
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ severity: 'high' }));
    expect(pill).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(screen.getByRole('switch', { name: /KEV uniquement/ }));
    await waitFor(() => expect(listParams().at(-1)).toMatchObject({ severity: 'high', kev: true }));
  });

  it('reads the SLA in days left or late, and not at all once accepted', async () => {
    renderPage();
    const late = (await screen.findByText('Délégation Kerberos non contrainte')).closest('tr')!;
    expect(late).toHaveTextContent('Dépassé de 2 j');
    const soon = screen.getByText('Exécution de code à distance — SSL-VPN').closest('tr')!;
    expect(soon).toHaveTextContent('1 j restant');
    const accepted = screen.getByText('XSS stocké — portail RH').closest('tr')!;
    expect(accepted).not.toHaveTextContent('restant');
    expect(accepted).toHaveTextContent('Acceptée');
  });

  it('opens a row in place with why it ranks and what to do', async () => {
    renderPage();
    await screen.findByText('Exécution de code à distance — SSL-VPN');
    const chevron = screen.getByRole('button', { name: 'Détails de CVE-2026-21873' });
    expect(chevron).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(chevron);
    expect(chevron).toHaveAttribute('aria-expanded', 'true');

    const detail = screen.getByTestId('vuln-detail');
    expect(detail).toHaveTextContent('Pourquoi P1 · 92');
    expect(detail).toHaveTextContent('Exploitée activement (CISA KEV)');
    expect(detail).toHaveTextContent('Une faille KEV passe toujours en P1.');
    expect(detail).toHaveTextContent('Actif de criticité critique');
    expect(detail).not.toHaveTextContent('CRITICAL');
    expect(detail).toHaveTextContent('Appliquer le correctif éditeur.');
    expect(within(detail).getByRole('button', { name: 'Prendre en charge' })).toBeInTheDocument();
    expect(within(detail).getByRole('button', { name: 'Accepter le risque' })).toBeInTheDocument();

    fireEvent.click(within(detail).getByRole('button', { name: 'Créer un risque' }));
    expect(
      screen.getByRole('dialog', {
        name: 'create-risk:Exécution de code à distance — SSL-VPN (CVE-2026-21873):a1',
      }),
    ).toBeInTheDocument();
  });

  it('changes the status at once and keeps it when the server agrees', async () => {
    // The server takes the change, so the refetch that follows reads it back.
    patch.mockImplementation(() => {
      rows[0] = { ...rows[0], status: 'in_remediation' };
      return Promise.resolve({ data: rows[0] });
    });
    renderPage();
    await screen.findByText('Exécution de code à distance — SSL-VPN');
    fireEvent.click(screen.getByRole('button', { name: 'Détails de CVE-2026-21873' }));
    fireEvent.click(
      within(screen.getByTestId('vuln-detail')).getByRole('button', { name: 'Prendre en charge' }),
    );

    const row = screen.getByText('Exécution de code à distance — SSL-VPN').closest('tr')!;
    await waitFor(() => expect(row).toHaveTextContent('En correction'));
    expect(patch).toHaveBeenCalledWith('/vulnerabilities/v1/status', { status: 'in_remediation' });
  });

  it('puts the status back when the server refuses', async () => {
    patch.mockRejectedValue(new Error('403'));
    renderPage();
    await screen.findByText('Exécution de code à distance — SSL-VPN');
    fireEvent.click(screen.getByRole('button', { name: 'Détails de CVE-2026-21873' }));
    fireEvent.click(
      within(screen.getByTestId('vuln-detail')).getByRole('button', { name: 'Marquer corrigée' }),
    );

    const row = screen.getByText('Exécution de code à distance — SSL-VPN').closest('tr')!;
    await waitFor(() => expect(patch).toHaveBeenCalled());
    await waitFor(() => expect(row).toHaveTextContent('Ouverte'));
  });

  it('offers no status change to a read-only member', async () => {
    perms.write = false;
    renderPage();
    await screen.findByText('Exécution de code à distance — SSL-VPN');
    fireEvent.click(screen.getByRole('button', { name: 'Détails de CVE-2026-21873' }));
    const detail = screen.getByTestId('vuln-detail');
    expect(within(detail).queryByRole('button', { name: 'Prendre en charge' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Importer' })).toBeNull();
  });

  it('offers a retry when the figures fail, and keeps the table', async () => {
    get.mockImplementation((url: string, config?: { params?: Record<string, unknown> }) =>
      url === '/vulnerabilities/stats' ? Promise.reject(new Error('down')) : route(url, config),
    );
    renderPage();
    await screen.findByText('Exécution de code à distance — SSL-VPN');
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});

describe('vulnQueue', () => {
  it('counts the SLA only while unresolved', () => {
    const now = new Date(2026, 9, 11, 15, 0);
    expect(
      slaState({ status: 'open', sla_due_at: new Date(2026, 9, 12, 9).toISOString() }, now),
    ).toEqual({ kind: 'left', days: 1 });
    expect(
      slaState({ status: 'triaged', sla_due_at: new Date(2026, 9, 11, 23).toISOString() }, now),
    ).toEqual({ kind: 'today' });
    expect(
      slaState(
        { status: 'in_remediation', sla_due_at: new Date(2026, 9, 9, 1).toISOString() },
        now,
      ),
    ).toEqual({ kind: 'late', days: 2 });
    expect(
      slaState({ status: 'accepted', sla_due_at: new Date(2026, 9, 1).toISOString() }, now),
    ).toEqual({ kind: 'none' });
    expect(slaState({ status: 'open', sla_due_at: null }, now)).toEqual({ kind: 'none' });
  });

  it('reads EPSS without inventing precision', () => {
    expect(epssPercent(0)).toBeNull();
    expect(epssPercent(0.0004)).toBe('<1');
    expect(epssPercent(0.412)).toBe('41');
  });

  it('offers reopening only once a finding is closed', () => {
    expect(nextStatuses('open')[0]).toBe('in_remediation');
    expect(nextStatuses('in_remediation')).not.toContain('in_remediation');
    expect(nextStatuses('remediated')).toEqual(['open']);
    expect(nextStatuses('accepted')).toEqual(['open']);
  });

  it('names KEV over EPSS, and the blast radius only when it is more than one', () => {
    const keys = prioritySignals(vuln({ affected_assets_count: 1 })).map((s) => s.key);
    expect(keys).toEqual(['cvss', 'kev', 'asset']);
    const plain = prioritySignals(
      vuln({ kev: false, exploit_available: false, epss: 0.18, affected_assets_count: 4 }),
    );
    expect(plain.map((s) => s.key)).toEqual(['cvss', 'epss', 'asset', 'affected']);
  });
});
