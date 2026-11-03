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
  // Bodies for the other routes; anything unlisted answers {}.
  bodies: {} as Record<string, unknown>,
}));
const calls = vi.hoisted(() => [] as string[]);

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../../lib/api', () => {
  const get = (url: string) => {
    calls.push(url);
    if (url === '/entitlements')
      return Promise.resolve({
        data: { plan: 'free', features: plan.features, limits: plan.limits },
      });
    return Promise.resolve({ data: plan.bodies[url] ?? {} });
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
import { financialSummary } from '../../financial/__tests__/fixtures';
import { PlanCapNotice } from '../PlanCapNotice';
import { planCapErrorMessage } from '../useEntitlements';
import { ThreatsPage } from '../../cti/ThreatsPage';
import { ThreatIntel } from '../../cti/ThreatIntel';
import { DrawerCTI } from '../../risks/RiskRegisterPage';
import type { UiRisk } from '../../risks/riskMap';

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
  plan.bodies = {};
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
    plan.bodies = { '/analytics/financial': financialSummary() };
    renderIt(<FinancialDashboard />);

    // The page renders the portfolio, so a crash in it fails here (#1140).
    expect(await screen.findByTestId('fin-kpi-inherent-value')).toHaveTextContent('117,2');
    expect(screen.getByTestId('fin-methodology')).toBeInTheDocument();
    expect(calls).toContain('/analytics/financial');
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

describe('#783 — threat intel below Business (criteria 15, 16)', () => {
  // The two screens, on the two plans that do not have threat intel.
  const screens: [string, () => ReactNode][] = [
    ['threats list', () => <ThreatsPage />],
    ['threat map', () => <ThreatIntel />],
  ];
  for (const planName of ['free', 'pro']) {
    for (const [name, Screen] of screens) {
      it(`${name} on ${planName}: blurred preview with the upsell, focus never enters it`, async () => {
        plan.features = { ...FREE, cti: { enabled: false, level: '', required_plan: 'business' } };
        renderIt(<Screen />);

        const cta = await screen.findByRole('button', { name: 'Passer au plan Business' });
        // `inert` is what keeps keyboard focus out of the preview in a browser
        // (jsdom does not apply it to Tab, so the attribute is asserted here and
        // the keyboard pass is done live). The way out sits outside it.
        const preview = document.querySelector('[inert]');
        expect(preview).not.toBeNull();
        expect(preview?.contains(cta)).toBe(false);
      });
    }
  }

  it('the risk drawer CTI tab on Pro explains the plan and never calls /cti', async () => {
    plan.features = {
      ...FREE,
      financial_quantification: on('on', 'free'),
      cti: { enabled: false, level: '', required_plan: 'business' },
    };
    const risk = { id: 'r-1', raw: { source_cve_id: 'CVE-2024-3094' } } as unknown as UiRisk;
    renderIt(<DrawerCTI r={risk} />);

    expect(
      await screen.findByText(/disponible à partir du plan Business/),
    ).toHaveTextContent('CVE-2024-3094');
    await waitFor(() => expect(calls).toContain('/entitlements'));
    expect(calls.filter((u) => u.startsWith('/cti'))).toEqual([]);
  });
});
