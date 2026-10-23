// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #756 — the executive board must not pass the reference model's amounts off
// as the tenant's exposure. With no quantified risk, the exposure tile reads
// "—" and the top-exposures block says why; with one, the figures come back.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';

const finance = vi.hoisted(() => ({ status: 'ok', quantified: 1 }));

vi.mock('../../../lib/api', () => {
  const money = (xaf: number) => ({ xaf, usd: xaf / 600 });
  const get = (url: string) => {
    if (url === '/entitlements')
      return Promise.resolve({
        data: {
          features: {
            financial_quantification: { enabled: true, level: 'on', required_plan: 'pro' },
          },
        },
      });
    if (url === '/analytics/financial')
      return Promise.resolve({
        data: {
          currency: 'XAF',
          fx_rate_xaf: 1,
          total_risks: 2,
          quantified_risks: finance.quantified,
          data_status: finance.status,
          min_quantified_risks: 1,
          total_ale: money(6_000_000),
          top_risks: [
            {
              id: 'r1',
              title: 'Phishing ciblé',
              criticality: 'high',
              ale: money(3_000_000),
              ale_after: money(3_000_000),
            },
            {
              id: 'r2',
              title: 'Panne du SI',
              criticality: 'high',
              ale: money(3_000_000),
              ale_after: money(3_000_000),
            },
          ],
        },
      });
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

import { ExecutiveDashboard } from '../ExecutiveDashboard';

function renderBoard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ExecutiveDashboard />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  finance.status = 'ok';
  finance.quantified = 1;
});

describe('#756 — executive exposure', () => {
  it('shows no amount while no risk is quantified', async () => {
    finance.status = 'insufficient_data';
    finance.quantified = 0;
    renderBoard();
    await waitFor(() =>
      expect(screen.getByTestId('exec-kpi-ale')).toHaveTextContent('aucun risque chiffré'),
    );
    expect(screen.getByTestId('exec-kpi-ale-value')).toHaveTextContent('—');
    const top = screen.getByTestId('exec-top');
    expect(top).toHaveTextContent('Aucun risque quantifié');
    expect(within(top).queryByText('Phishing ciblé')).not.toBeInTheDocument();
  });

  it('shows the amounts once a risk is quantified', async () => {
    renderBoard();
    await waitFor(() => expect(screen.getByTestId('exec-kpi-ale-value')).toHaveTextContent('6,0'));
    expect(within(screen.getByTestId('exec-top')).getByText('Phishing ciblé')).toBeInTheDocument();
  });
});
