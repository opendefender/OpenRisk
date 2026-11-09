// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Threat Intel (#911): threats that touch the tenant's assets, the relevance
// view, the explanation of each score, "Non pertinent" and its undo, and an
// IOC panel that claims nothing it cannot compute.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { toast } from 'sonner';

import { useUIStore } from '../../../store/uiStore';
import type { Threat, ThreatList } from '../ctiService';
import { relevanceTone, sourceLabels, threatTitle, visibleThreats } from '../threatMeta';

const perms = { write: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({
      hasPermission: (p: string) =>
        p === 'risks:update' || p === 'risks:create' ? perms.write : true,
    }),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

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
const put = vi.fn();
const del = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: {
    get: (...a: unknown[]) => get(...a),
    put: (...a: unknown[]) => put(...a),
    delete: (...a: unknown[]) => del(...a),
    post: vi.fn(),
    patch: vi.fn(),
    defaults: { baseURL: '' },
  },
}));

import { ThreatsPage } from '../ThreatsPage';

// Threat intel opens at Business (#783): these tests run on a plan that has it.
const BUSINESS = {
  plan: 'business',
  features: { cti: { enabled: true, required_plan: 'business' } },
};

function threat(over: Partial<Threat>): Threat {
  return {
    cve_id: 'CVE-2026-21873',
    title: 'Exécution de code à distance — SSL-VPN',
    description: 'A heap overflow in the SSL-VPN portal. More text.',
    severity: 'critical',
    cvss: 9.8,
    epss: 0.94,
    kev: true,
    published_at: '2026-10-04T00:00:00Z',
    techniques: ['T1190', 'T1133'],
    tactics: [],
    assets: [
      { id: 'a1', name: 'Pare-feu périmétrique', criticality: 'CRITICAL', via: ['scan', 'cpe'] },
    ],
    relevance: 100,
    factors: [
      { key: 'asset', points: 40, max: 40, value: 'CRITICAL' },
      { key: 'exploitation', points: 30, max: 30, value: 'kev' },
      { key: 'severity', points: 20, max: 20, value: '' },
      { key: 'evidence', points: 10, max: 10, value: 'scan' },
    ],
    sources: ['nvd', 'cisa_kev', 'mitre_attack', 'nessus'],
    dismissed_at: null,
    ...over,
  };
}

let body: ThreatList;

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/threat-intel']}>
        <ThreatsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  perms.write = true;
  body = {
    assets_with_cpes: 3,
    truncated: false,
    threats: [
      threat({}),
      threat({
        cve_id: 'CVE-2026-17760',
        title: '',
        description: 'Stored XSS in the HR portal. Details follow.',
        kev: false,
        cvss: 5.4,
        epss: 0,
        techniques: [],
        assets: [{ id: null, name: 'kora-rh.local', criticality: 'MEDIUM', via: ['scan'] }],
        relevance: 41,
        factors: [
          { key: 'asset', points: 20, max: 40, value: 'MEDIUM' },
          { key: 'exploitation', points: 0, max: 30, value: '' },
          { key: 'severity', points: 11, max: 20, value: '' },
          { key: 'evidence', points: 10, max: 10, value: 'scan' },
        ],
        sources: ['nvd'],
      }),
    ],
  };
  get.mockReset();
  get.mockImplementation((url: string) =>
    url === '/cti/threats'
      ? Promise.resolve({ data: body })
      : url === '/entitlements'
        ? Promise.resolve({ data: BUSINESS })
        : Promise.resolve({ data: {} }),
  );
  put.mockReset();
  del.mockReset();
  vi.mocked(toast.error).mockClear();
});

describe('ThreatsPage', () => {
  it('lists the threats with their source, date, techniques and assets', async () => {
    renderPage();
    const card = await screen.findByTestId('threat-CVE-2026-21873');
    expect(card).toHaveTextContent('CISA KEV · 4 oct. · CVE-2026-21873');
    expect(card).toHaveTextContent('Exécution de code à distance — SSL-VPN');
    expect(card).toHaveTextContent('T1190');
    const asset = within(card).getByRole('link', { name: /Pare-feu périmétrique/ });
    expect(asset).toHaveAttribute('href', '/assets?focus=a1');
    expect(asset).toHaveAccessibleName(/trouvée par un scanner, par ses CPE/);
    // A scanned host that matches no asset is named, not linked.
    const other = screen.getByTestId('threat-CVE-2026-17760');
    expect(within(other).queryByRole('link', { name: /kora-rh/ })).toBeNull();
    expect(other).toHaveTextContent('Stored XSS in the HR portal.');
  });

  it('keeps only relevance ≥ 70 on request', async () => {
    renderPage();
    await screen.findByTestId('threat-CVE-2026-17760');
    fireEvent.click(screen.getByTestId('threat-mode-high'));
    expect(screen.queryByTestId('threat-CVE-2026-17760')).toBeNull();
    expect(screen.getByTestId('threat-CVE-2026-21873')).toBeInTheDocument();
    expect(screen.getByTestId('threat-mode-high')).toHaveAttribute('aria-pressed', 'true');
  });

  it('explains a score part by part, sector and sources included', async () => {
    renderPage();
    fireEvent.click(await screen.findByTestId('threat-relevance-CVE-2026-21873'));
    const why = await screen.findByTestId('threat-why');
    expect(why).toHaveTextContent('Pourquoi 100/100');
    expect(why).toHaveTextContent('Actif le plus critique touché : critique40/40');
    expect(why).toHaveTextContent('Exploitée activement (CISA KEV)30/30');
    expect(why).toHaveTextContent('Gravité CVSS 9,820/20');
    expect(why).toHaveTextContent('Trouvée par un scanner sur vos actifs10/10');
    expect(why).toHaveTextContent('Secteur : non pris en compte');
    expect(why).toHaveTextContent('Sources : NVD, CISA KEV, MITRE ATT&CK, Nessus');
  });

  it('dims a threat at once on "Non pertinent" and keeps it when the server agrees', async () => {
    put.mockImplementation(() => {
      body = {
        ...body,
        threats: body.threats.map((t) =>
          t.cve_id === 'CVE-2026-21873' ? { ...t, dismissed_at: '2026-10-14T08:00:00Z' } : t,
        ),
      };
      return Promise.resolve({ data: null });
    });
    renderPage();
    fireEvent.click(await screen.findByTestId('threat-dismiss-CVE-2026-21873'));
    const card = screen.getByTestId('threat-CVE-2026-21873');
    await waitFor(() => expect(card).toHaveAttribute('data-dismissed', 'true'));
    expect(put).toHaveBeenCalledWith('/cti/threats/CVE-2026-21873/dismissal');
    await waitFor(() =>
      expect(within(card).getByRole('button', { name: 'Restaurer' })).toBeInTheDocument(),
    );
  });

  it('brings the threat back and says so when the server refuses', async () => {
    put.mockRejectedValue(new Error('403'));
    renderPage();
    fireEvent.click(await screen.findByTestId('threat-dismiss-CVE-2026-21873'));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    expect(screen.getByTestId('threat-CVE-2026-21873')).not.toHaveAttribute('data-dismissed');
    expect(put).toHaveBeenCalledTimes(1);
  });

  it('opens the risk form with the threat and its assets', async () => {
    renderPage();
    const card = await screen.findByTestId('threat-CVE-2026-21873');
    fireEvent.click(within(card).getByRole('button', { name: 'Créer un risque' }));
    expect(
      screen.getByRole('dialog', {
        name: 'create-risk:Exécution de code à distance — SSL-VPN (CVE-2026-21873):a1',
      }),
    ).toBeInTheDocument();
  });

  it('offers no action to a read-only member', async () => {
    perms.write = false;
    renderPage();
    const card = await screen.findByTestId('threat-CVE-2026-21873');
    expect(within(card).queryByRole('button', { name: 'Non pertinent' })).toBeNull();
    expect(within(card).queryByRole('button', { name: 'Créer un risque' })).toBeNull();
  });

  it('says why the list is empty when no asset has CPEs', async () => {
    body = { threats: [], assets_with_cpes: 0, truncated: false };
    renderPage();
    expect(await screen.findByText(/renseignez leurs CPE/)).toBeInTheDocument();
  });

  it('offers a retry when the list fails', async () => {
    get.mockImplementation((url: string) =>
      url === '/entitlements'
        ? Promise.resolve({ data: BUSINESS })
        : Promise.reject(new Error('down')),
    );
    renderPage();
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });

  it('claims no IOC match and keeps the CVE feed one click away', async () => {
    renderPage();
    const ioc = screen.getByTestId('ioc-panel');
    expect(ioc).toHaveTextContent('Correspondances indisponibles');
    expect(ioc).not.toHaveTextContent(/corresp\./);
    expect(screen.getByRole('link', { name: 'Flux CVE complet' })).toHaveAttribute(
      'href',
      '/threat-map',
    );
  });
});

describe('threatMeta', () => {
  it('titles a card from the scanner, else the first sentence, else the CVE', () => {
    expect(threatTitle({ title: 'Scanner title', description: 'x', cve_id: 'C' })).toBe(
      'Scanner title',
    );
    expect(threatTitle({ title: '', description: 'First one. Second.', cve_id: 'C' })).toBe(
      'First one.',
    );
    expect(threatTitle({ title: '', description: '', cve_id: 'CVE-1' })).toBe('CVE-1');
    expect(threatTitle({ title: '', description: 'a'.repeat(300), cve_id: 'C' })).toHaveLength(140);
  });

  it('tones and filters on the design thresholds', () => {
    expect(relevanceTone(85)).toBe('text-danger-text');
    expect(relevanceTone(60)).toBe('text-warning-text');
    expect(relevanceTone(59)).toBe('text-ink-soft');
    const list = [threat({ relevance: 70 }), threat({ cve_id: 'x', relevance: 69 })];
    expect(visibleThreats(list, 'high').map((t) => t.relevance)).toEqual([70]);
    expect(visibleThreats(list, 'all')).toHaveLength(2);
  });

  it('names the feeds and the scanners', () => {
    expect(sourceLabels(['nvd', 'cisa_kev', 'qualys', 'unknown'])).toEqual([
      'NVD',
      'CISA KEV',
      'Qualys',
      'unknown',
    ]);
  });
});
