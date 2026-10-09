// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The financial page of the October 2026 redesign (#904), against the API at
// its HTTP boundary. The admin gate is the real one (auth store permissions).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import type { FinancialSummary } from '../financialService';

const puts = vi.fn<(url: string, body: unknown) => Promise<{ data: unknown }>>();
const toastError = vi.hoisted(() => vi.fn());
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: toastError } }));

const M = 1_000_000;
const money = (xaf: number) => ({ xaf, usd: xaf / 600 });
const amount = (xaf: number) => ({ xaf, usd: xaf / 600, value: xaf, currency: 'XAF' });

function summary(over: Partial<FinancialSummary> = {}): FinancialSummary {
  return {
    currency: 'XAF',
    fx_rate_xaf: 1,
    fx_as_of: '2026-10-01T00:00:00Z',
    xaf_per_usd: 600,
    computed_at: '2026-10-09T08:00:00Z',
    formula_version: 'fair-lite-1.0.0',
    iterations: 10000,
    total_risks: 9,
    quantified_risks: 9,
    portfolio_loss: {
      p10: amount(60 * M),
      p50: amount(100 * M),
      p90: amount(180 * M),
      p95: amount(190 * M),
      mean: amount(100 * M),
      iterations: 10000,
      seed: 1,
      formula_version: 'fair-lite-1.0.0',
      lef: 1,
    },
    total_ale: money(117.2 * M),
    total_ale_worst: money(200 * M),
    total_ale_after: money(48.5 * M),
    total_risk_reduction: money(68.7 * M),
    total_remediation: money(62.2 * M),
    portfolio_rosi: 0.1,
    portfolio_rosi_computable: true,
    by_criticality: [],
    top_risks: [
      {
        id: 'aaaaaaaa-0000-4000-8000-000000000001',
        title: 'Rançongiciel sur le SI cœur bancaire',
        criticality: 'critical',
        ale: money(42 * M),
        ale_worst: money(80 * M),
        ale_after: money(18.4 * M),
        rosi: 1,
        rosi_computable: true,
      },
      {
        id: 'bbbbbbbb-0000-4000-8000-000000000002',
        title: 'Fraude interne sur les virements',
        criticality: 'high',
        ale: money(8.4 * M),
        ale_worst: money(16 * M),
        ale_after: money(3 * M),
        rosi: 1,
        rosi_computable: true,
      },
    ],
    portfolio_rosi_3y: 1.85,
    portfolio_rosi_3y_computable: true,
    treated_risks: 10,
    // Uniform years: inherent 0..200 M, after plan 0..50 M.
    loss_exceedance: {
      inherent: Array.from({ length: 101 }, (_, p) => 2 * p * M),
      residual: Array.from({ length: 101 }, (_, p) => 0.5 * p * M),
    },
    treatments: [
      {
        risk_id: 'cccccccc-0000-4000-8000-000000000003',
        title: 'Double validation des virements',
        cost: money(2 * M),
        reduction: money(4.8 * M),
        payback_months: 5,
      },
      {
        risk_id: 'dddddddd-0000-4000-8000-000000000004',
        title: 'Second bureau de service SWIFT',
        cost: money(5 * M),
        reduction: money(1.4 * M),
        payback_months: 42.9,
      },
    ],
    risk_appetite_xaf: 80 * M,
    ...over,
  };
}

let current: FinancialSummary | Error = summary();

vi.mock('../../../lib/api', () => ({
  api: {
    get: (url: string) => {
      if (url === '/entitlements')
        return Promise.resolve({
          data: {
            features: { financial_quantification: { enabled: true, required_plan: 'business' } },
          },
        });
      if (url === '/analytics/financial')
        return current instanceof Error
          ? Promise.reject(current)
          : Promise.resolve({ data: current });
      return Promise.resolve({ data: {} });
    },
    put: (url: string, body: unknown) => puts(url, body),
    post: () => Promise.resolve({ data: {} }),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

import { FinancialDashboard } from '../FinancialDashboard';

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

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 3, retryDelay: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <FinancialDashboard />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  current = summary();
  puts.mockReset();
  toastError.mockReset();
  useUIStore.setState({ lang: 'fr' });
  signIn(['*']);
});

describe('#904 — financial page', () => {
  it('shows the four headline figures', async () => {
    renderPage();
    expect(await screen.findByTestId('fin-kpi-inherent-value')).toHaveTextContent('117,2');
    expect(screen.getByTestId('fin-kpi-residual-value')).toHaveTextContent('48,5');
    expect(screen.getByTestId('fin-kpi-plan-value')).toHaveTextContent('62,2');
    expect(screen.getByTestId('fin-kpi-rosi-value')).toHaveTextContent('+185 %');
    expect(screen.getByTestId('fin-kpi-plan')).toHaveTextContent('FCFA, 10 risques traités');
  });

  it('reads both exceedance probabilities at the stored appetite', async () => {
    renderPage();
    const legend = await screen.findByTestId('fin-lec-legend');
    // P(uniform 0..200 M > 80 M) = 60 %; after plan the worst year is 50 M.
    expect(legend).toHaveTextContent('Inhérent : 60 % de dépasser 80 M');
    expect(legend).toHaveTextContent('Après plan : 0 %');
    expect(screen.getByTestId('fin-appetite-value')).toHaveTextContent('80 M FCFA');
    expect(screen.getByText('Perte annuelle (M FCFA)')).toBeInTheDocument();
    expect(screen.getByText('Probabilité de dépassement')).toBeInTheDocument();
  });

  it('saves a moved appetite in XAF, once, and updates the legend at once', async () => {
    puts.mockResolvedValue({ data: { appetite_xaf: 120 * M } });
    renderPage();
    fireEvent.change(await screen.findByTestId('fin-appetite'), { target: { value: '120' } });
    expect(screen.getByTestId('fin-lec-legend')).toHaveTextContent(
      'Inhérent : 40 % de dépasser 120 M',
    );
    await waitFor(() =>
      expect(puts).toHaveBeenCalledWith('/analytics/financial/appetite', { appetite_xaf: 120 * M }),
    );
    expect(puts).toHaveBeenCalledTimes(1);
  });

  it('rolls the appetite back when the server refuses, without retrying', async () => {
    puts.mockRejectedValue(
      Object.assign(new Error('forbidden'), {
        isAxiosError: true,
        response: { status: 403, data: {} },
      }),
    );
    renderPage();
    fireEvent.change(await screen.findByTestId('fin-appetite'), { target: { value: '120' } });
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Seul un administrateur peut fixer l'appétence."),
    );
    expect(puts).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(screen.getByTestId('fin-appetite-value')).toHaveTextContent('80 M FCFA'),
    );
  });

  it('a member without admin rights sees the appetite but cannot move it', async () => {
    signIn(['risks:read']);
    renderPage();
    expect(await screen.findByTestId('fin-appetite')).toBeDisabled();
    expect(screen.queryByTestId('fin-currency')).toBeNull();
  });

  it('says when no appetite is set yet', async () => {
    current = summary({ risk_appetite_xaf: null });
    renderPage();
    expect(await screen.findByTestId('fin-appetite-value')).toHaveTextContent('non fixée');
  });

  it('ranks plans by payback and links each to its risk mitigations', async () => {
    renderPage();
    const rows = await screen.findAllByTestId('fin-payback-row');
    expect(rows.map((r) => within(r).getByText(/mois/).textContent)).toEqual(['5 mois', '43 mois']);
    expect(rows[0]).toHaveAttribute(
      'href',
      '/risks?focus=cccccccc-0000-4000-8000-000000000003&tab=miti',
    );
    expect(rows[0]).toHaveTextContent('coût 2,0 · −4,8/an');
  });

  it('lists exposure by risk with the reduction share', async () => {
    renderPage();
    const rows = await screen.findAllByTestId('fin-exposure-row');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('42,0');
    expect(rows[0]).toHaveTextContent('18,4');
    expect(rows[0]).toHaveTextContent('−56%');
  });

  it('opens the methodology from the subtitle', async () => {
    renderPage();
    fireEvent.click(await screen.findByTestId('fin-methodology'));
    expect(await screen.findByRole('dialog')).toHaveTextContent('fair-lite-1.0.0');
  });

  it('empty states: no plan, no quantified risk, empty register', async () => {
    current = summary({ treatments: [], top_risks: [] });
    const { unmount } = renderPage();
    expect(await screen.findByText(/Aucun plan de traitement chiffré/)).toBeInTheDocument();
    expect(screen.getByText(/Aucun risque chiffré/)).toBeInTheDocument();
    unmount();

    current = summary({ total_risks: 0, loss_exceedance: null, top_risks: [], treatments: [] });
    renderPage();
    expect(await screen.findByText(/Aucun risque à quantifier/)).toBeInTheDocument();
  });

  it('a failed load offers a retry', async () => {
    current = new Error('boom');
    renderPage();
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});
