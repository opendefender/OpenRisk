// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #287 — on an empty tenant, every surface that shows a security score must say
// the same thing. The audit found four: home 0/100, sidebar 25/100, executive
// board F 36/100, and "A/100" before import.
//
// This renders the REAL surfaces — the sidebar, both executive dashboards, the
// score page and the gauge the home and viewer dashboards use — against an API
// that answers like an empty tenant, and asserts that:
//   - none of them prints a number or a grade;
//   - every one of them says "not measured";
//   - they are all fed by ONE /score request (one source, not four).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import type { UnmeasuredScore } from '../../../services/scoreService';
import type { ExecutiveDashboard as ExecData } from '../../analytics/executiveService';
import { useUIStore } from '../../../store/uiStore';

const EMPTY_SCORE: UnmeasuredScore = {
  scope: 'tenant',
  measured: false,
  reason_i18n_key: 'score.unmeasured.no_data',
  value: null,
  band: null,
  band_label_i18n_key: null,
  inherent: null,
  inherent_band: null,
  residual: null,
  residual_band: null,
  mitigation_effectiveness: 0,
  computed_at: '2026-09-25T10:00:00Z',
  formula_version: '2.2',
  inputs: { total_risks: 0 },
  breakdown: [
    {
      factor: 'risk_exposure',
      weight: 0,
      raw: 0,
      contribution: 0,
      label_i18n_key: 'score.factor.risk_exposure',
      available: false,
    },
  ],
};

const EMPTY_EXEC: ExecData = {
  generated_at: '2026-09-25T10:00:00Z',
  currency: 'XAF',
  xaf_per_usd: 600,
  financial: {
    total_ale: { xaf: 0, usd: 0 },
    total_ale_worst: { xaf: 0, usd: 0 },
    total_risks: 0,
    quantified_risks: 0,
  },
  kris: [],
  top_risks: [],
  risk_trend: [],
  risk_distribution: [],
  compliance: [],
  incident_trend: [],
};

const scoreCalls = vi.fn();

// The HTTP boundary, answering like a brand-new tenant. Anything a surface asks
// for that is not about the score gets an empty-but-valid body.
vi.mock('../../../lib/api', () => {
  const get = (url: string) => {
    if (url === '/score') {
      scoreCalls();
      return Promise.resolve({ data: EMPTY_SCORE });
    }
    if (url === '/analytics/executive') return Promise.resolve({ data: EMPTY_EXEC });
    return Promise.resolve({ data: {} });
  };
  return {
    api: {
      get,
      post: () => Promise.resolve({ data: {} }),
      interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
    },
    onMFARequired: () => () => undefined,
  };
});

import { Sidebar } from '../../../components/layout/Sidebar';
import { ExecDashboard } from '../../dashboard/ExecDashboard';
import { ExecutiveDashboard } from '../../analytics/ExecutiveDashboard';
import { ScorePage } from '../ScorePage';
import { ScoreGauge } from '../../../shared/ScoreGauge';
import { useScore } from '../../../hooks/useScore';

/** The home and viewer dashboards render exactly this: the shared gauge fed by useScore. */
function HomeHero() {
  const { data, isLoading, isError } = useScore('tenant');
  return (
    <div data-testid="surface-home">
      <ScoreGauge score={data} loading={isLoading} error={isError} title="Score" />
    </div>
  );
}

function renderSurfaces() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <div data-testid="surface-sidebar">
          <Sidebar />
        </div>
        <HomeHero />
        <div data-testid="surface-exec-persona">
          <ExecDashboard />
        </div>
        <div data-testid="surface-exec-analytics">
          <ExecutiveDashboard />
        </div>
        <div data-testid="surface-score-page">
          <ScorePage />
        </div>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  scoreCalls.mockReset();
  useUIStore.setState({ lang: 'fr', sidebarCollapsed: false });
});

describe('#287 — empty tenant, every score surface agrees', () => {
  it('no surface prints a number or a grade; all say "non mesuré", from one /score request', async () => {
    renderSurfaces();

    const gauges = [
      'surface-home',
      'surface-exec-persona',
      'surface-exec-analytics',
      'surface-score-page',
    ];

    await waitFor(() => {
      for (const id of gauges) {
        expect(within(screen.getByTestId(id)).getByTestId('score-state')).toHaveTextContent(
          'non mesuré',
        );
      }
      expect(screen.getByTestId('sidebar-score-value')).toHaveTextContent('—');
    });

    for (const id of gauges) {
      const surface = within(screen.getByTestId(id));
      expect(surface.getByTestId('score-value')).toHaveTextContent('—');
      expect(surface.queryByTestId('score-band')).toBeNull();
      expect(surface.getByTestId('score-unmeasured-reason')).toHaveTextContent(
        'Ajoutez des risques pour calculer votre score',
      );
    }

    // The sidebar says it in words too, never "0/100" or "25/100".
    const sidebar = screen.getByTestId('surface-sidebar');
    expect(sidebar).toHaveTextContent('Non mesuré');
    expect(sidebar.textContent).not.toMatch(/\d+\s*\/\s*100/);

    // No surface falls back to a grade or a default score anywhere on the page.
    const page = document.body.textContent ?? '';
    expect(page).not.toMatch(/\b\d+\s*\/\s*100\b/);
    expect(page).not.toMatch(/Cyber score/i);

    // One source: five surfaces, one request.
    expect(scoreCalls).toHaveBeenCalledTimes(1);
  });
});
