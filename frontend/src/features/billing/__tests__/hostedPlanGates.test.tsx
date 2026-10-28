// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #783 under D-078 — Free does almost everything, in small quantities. The
// screens show the counter under a cap and the cap's sentence at it; a depth
// the plan does not reach (the Monte-Carlo portfolio on Free) shows the upsell
// and is never called; threat intel keeps its Business gate.
//
// The gate is the real one: useFeature / useLimit read GET /entitlements
// through the mocked HTTP boundary, so a plan here is what the server reports.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { useUIStore } from '../../../store/uiStore';

type Feat = { enabled: boolean; level: string; required_plan: string };
const plan = vi.hoisted(() => ({
  features: {} as Record<string, Feat>,
  limits: {} as Record<string, { limit: number; used: number }>,
}));
const calls = vi.hoisted(() => [] as string[]);
// An empty register, shaped as GET /analytics/financial returns it: the page
// reads portfolio_loss as soon as the summary lands.
const emptyPortfolio = vi.hoisted(() => {
  const zero = { xaf: 0, usd: 0, value: 0, currency: 'XAF' };
  const money = { xaf: 0, usd: 0 };
  return {
    currency: 'XAF',
    fx_rate_xaf: 1,
    fx_as_of: '2026-10-01T00:00:00Z',
    xaf_per_usd: 600,
    computed_at: '2026-10-09T08:00:00Z',
    formula_version: 'fair-lite-1.0.0',
    iterations: 10000,
    total_risks: 0,
    quantified_risks: 0,
    data_status: 'ok',
    min_quantified_risks: 1,
    portfolio_loss: {
      p10: zero,
      p50: zero,
      p90: zero,
      p95: zero,
      mean: zero,
      iterations: 10000,
      seed: 1,
      formula_version: 'fair-lite-1.0.0',
      lef: 0,
    },
    total_ale: money,
    total_ale_worst: money,
    total_ale_after: money,
    total_risk_reduction: money,
    total_remediation: money,
    portfolio_rosi: 0,
    portfolio_rosi_computable: false,
    by_criticality: [],
    top_risks: [],
    portfolio_rosi_3y: 0,
    portfolio_rosi_3y_computable: false,
    treated_risks: 0,
    loss_exceedance: null,
    treatments: [],
    risk_appetite_xaf: null,
  };
});

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../../lib/api', () => {
  const get = (url: string) => {
    calls.push(url);
    if (url === '/entitlements')
      return Promise.resolve({
        data: { plan: 'free', features: plan.features, limits: plan.limits },
      });
    if (url === '/analytics/financial') return Promise.resolve({ data: emptyPortfolio });
    return Promise.resolve({ data: {} });
  };
  return {
    api: {
      get,
      post: () => Promise.resolve({ data: {} }),
      put: () => Promise.resolve({ data: {} }),
      delete: () => Promise.resolve({ data: {} }),
      interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
    },
    onMFARequired: () => () => undefined,
  };
});

import { AutomationPage } from '../../automation/AutomationPage';
import { FinancialDashboard } from '../../financial/FinancialDashboard';
import { PlanCapNotice } from '../PlanCapNotice';
import { planCapErrorMessage } from '../useEntitlements';

const on = (level = 'on', required = 'free'): Feat => ({
  enabled: true,
  level,
  required_plan: required,
});

// The shipped Free row (D-078).
const FREE: Record<string, Feat> = {
  automation: on(),
  scanner: on(),
  governance: on(),
  smart_score: on(),
  executive_dashboard: on(),
  financial_quantification: on('basic'),
  cti: { enabled: false, level: '', required_plan: 'business' },
};

function renderIt(node: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{node}</MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  calls.length = 0;
  plan.features = FREE;
  plan.limits = {};
  window.history.replaceState(null, '', '/');
});

describe('#783 — automation on Free (D-078)', () => {
  it('is open: rules load, no upsell, and the counter shows the cap coming', async () => {
    plan.limits = { automation_rules: { limit: 2, used: 1 } };
    renderIt(<AutomationPage />);

    await waitFor(() => expect(calls).toContain('/automation/rules'));
    expect(await screen.findByTestId('plan-usage-automation_rules')).toHaveTextContent(
      'Règles d’automatisation actives : 1/2 sur votre plan',
    );
    expect(screen.queryByText(/Passer au plan/)).not.toBeInTheDocument();
  });

  it('at the cap, says so with the numbers and the way out', async () => {
    plan.limits = { automation_rules: { limit: 2, used: 2 } };
    renderIt(<AutomationPage />);

    expect(await screen.findByTestId('plan-cap-automation_rules')).toHaveTextContent(
      'Votre plan inclut 2 règles d’automatisation actives (2/2). Suspendez-en une ou passez à un plan supérieur pour en ajouter.',
    );
  });
});

describe('#783 — the Monte-Carlo portfolio on Free', () => {
  it('shows the upsell naming Pro and never calls the portfolio route', async () => {
    renderIt(<FinancialDashboard />);

    expect(await screen.findByRole('button', { name: 'Passer au plan Pro' })).toBeInTheDocument();
    expect(calls).not.toContain('/analytics/financial');
  });

  it('loads the portfolio on Pro', async () => {
    plan.features = { ...FREE, financial_quantification: on('on', 'free') };
    renderIt(<FinancialDashboard />);

    await waitFor(() => expect(calls).toContain('/analytics/financial'));
    expect(await screen.findByTestId('fin-methodology')).toBeInTheDocument();
    expect(screen.queryByText(/Passer au plan/)).not.toBeInTheDocument();
  });
});

describe('#783 — plan caps (D-078)', () => {
  it('names a monthly scan cap and when it resets', async () => {
    plan.limits = { scans_per_month: { limit: 4, used: 4 } };
    renderIt(<PlanCapNotice limitKey="scans_per_month" />);

    expect(await screen.findByTestId('plan-cap-scans_per_month')).toHaveTextContent(
      'Votre plan inclut 4 scans par mois, tous utilisés ce mois-ci (4/4). Le compteur repart le 1er du mois',
    );
  });

  it('says nothing under the cap or uncapped, unless asked for the counter', async () => {
    plan.limits = { frameworks: { limit: 3, used: 2 }, users: { limit: -1, used: 40 } };
    renderIt(
      <>
        <PlanCapNotice limitKey="frameworks" />
        <PlanCapNotice limitKey="users" showUsage />
      </>,
    );
    await waitFor(() => expect(calls).toContain('/entitlements'));
    expect(screen.queryByTestId('plan-cap-frameworks')).not.toBeInTheDocument();
    expect(screen.queryByTestId('plan-usage-users')).not.toBeInTheDocument();
  });

  it('turns a 402 limit_reached into the sentence, in English too', () => {
    const err = {
      response: {
        status: 402,
        data: { code: 'limit_reached', limit_key: 'approval_workflows', limit: 1, used: 1 },
      },
    };
    expect(planCapErrorMessage(err, 'en')).toBe(
      'Your plan includes 1 approval workflow (1/1). Delete one or upgrade to add more.',
    );
    expect(planCapErrorMessage({ response: { status: 500 } }, 'fr')).toBeNull();
    expect(
      planCapErrorMessage(
        { response: { status: 402, data: { code: 'upgrade_required', feature: 'cti' } } },
        'fr',
      ),
    ).toBeNull();
  });
});
