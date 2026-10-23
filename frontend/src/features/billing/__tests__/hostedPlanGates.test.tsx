// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #783 — the hosted plan gates on the screens. The API answers 402 on a plan
// without the feature; these screens must show the upsell, or leave a block
// out, instead of calling the refused route and rendering its error.
//
// The gate is the real one: useFeature reads GET /entitlements through the
// mocked HTTP boundary, so a plan here is what the server would report.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';

const plan = vi.hoisted(() => ({
  features: {} as Record<string, { enabled: boolean; required_plan: string }>,
  limits: {} as Record<string, { limit: number; used: number }>,
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
import { ExecutiveDashboard } from '../../analytics/ExecutiveDashboard';
import { DiscoverButton } from '../../assets/DiscoverButton';
import { FrameworkCapNotice } from '../../compliance/FrameworkCapNotice';

function feature(enabled: boolean, required = 'pro') {
  return { enabled, required_plan: required };
}

function renderAt(node: ReactNode, path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/infrastructure" element={<div>infrastructure screen</div>} />
          <Route path="*" element={node} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  calls.length = 0;
  plan.features = {};
  plan.limits = {};
  window.history.replaceState(null, '', '/');
});

describe('#783 — automation', () => {
  it('Free: the SOAR tabs show the upsell and never call the gated routes', async () => {
    plan.features = { automation: feature(false, 'pro') };
    renderAt(<AutomationPage />);

    expect(await screen.findByText('Automatisation (SOAR)')).toBeInTheDocument();
    expect(screen.getByText(/Disponible à partir du plan Pro\./)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nouvelle règle' })).not.toBeInTheDocument();
    expect(calls).not.toContain('/automation/rules');
    expect(calls).not.toContain('/automation/sla/stats');
  });

  it('Free: the channels tab stays open, because incident alerts use it', async () => {
    plan.features = { automation: feature(false, 'pro') };
    window.history.replaceState(null, '', '/automation?tab=channels');
    renderAt(<AutomationPage />, '/automation?tab=channels');

    await waitFor(() => expect(calls).toContain('/automation/channels'));
    expect(screen.queryByText('Automatisation (SOAR)')).not.toBeInTheDocument();
  });

  it('Pro: the rules load and no upsell shows', async () => {
    plan.features = { automation: feature(true) };
    renderAt(<AutomationPage />);

    await waitFor(() => expect(calls).toContain('/automation/rules'));
    expect(screen.queryByText('Automatisation (SOAR)')).not.toBeInTheDocument();
  });

  it('English: the upsell reads in English and names the plan the snapshot reports', async () => {
    useUIStore.setState({ lang: 'en' });
    plan.features = { automation: feature(false, 'pro') };
    renderAt(<AutomationPage />);

    expect(await screen.findByText('Automation (SOAR)')).toBeInTheDocument();
    expect(screen.getByText(/Available from the Pro plan\./)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Upgrade to Pro' })).toBeInTheDocument();
  });
});

describe('#783 — executive dashboard on a plan without governance', () => {
  it('Pro: leaves the committee block out and never reads the approval inbox', async () => {
    plan.features = {
      executive_dashboard: feature(true),
      governance: feature(false, 'business'),
    };
    renderAt(<ExecutiveDashboard />);

    await waitFor(() => expect(calls).toContain('/entitlements'));
    await new Promise((r) => setTimeout(r, 20));
    expect(calls.some((u) => u.startsWith('/governance/'))).toBe(false);
  });

  it('Business: reads the approval inbox', async () => {
    plan.features = { executive_dashboard: feature(true), governance: feature(true, 'business') };
    renderAt(<ExecutiveDashboard />);

    await waitFor(() =>
      expect(calls.some((u) => u.startsWith('/governance/approvals'))).toBe(true),
    );
  });
});

describe('#783 — discover button on a plan without the scanner', () => {
  beforeEach(() => {
    useAuthStore.setState({
      hasPermission: () => true,
    } as unknown as Parameters<typeof useAuthStore.setState>[0]);
  });

  it('Free: does not list scan configs and leads to Infrastructure, where the upsell is', async () => {
    plan.features = { scanner: feature(false, 'pro') };
    renderAt(<DiscoverButton />);

    const button = await screen.findByTestId('inv-discover');
    await waitFor(() => expect(calls).toContain('/entitlements'));
    expect(calls).not.toContain('/scanner/configs');
    fireEvent.click(button);
    expect(await screen.findByText('infrastructure screen')).toBeInTheDocument();
  });

  it('Pro: lists the scan configs', async () => {
    plan.features = { scanner: feature(true) };
    renderAt(<DiscoverButton />);

    await waitFor(() => expect(calls).toContain('/scanner/configs'));
  });
});

describe('#783 — framework cap notice (D-076)', () => {
  it('says the cap with the numbers when the plan is full', async () => {
    plan.limits = { frameworks: { limit: 1, used: 1 } };
    renderAt(<FrameworkCapNotice />);

    expect(await screen.findByTestId('framework-cap')).toHaveTextContent(
      'Votre plan garde 1 référentiel actif à la fois (1/1).',
    );
  });

  it('stays silent under the cap and when uncapped', async () => {
    plan.limits = { frameworks: { limit: 3, used: 2 } };
    const { unmount } = renderAt(<FrameworkCapNotice />);
    await waitFor(() => expect(calls).toContain('/entitlements'));
    expect(screen.queryByTestId('framework-cap')).not.toBeInTheDocument();
    unmount();

    plan.limits = { frameworks: { limit: -1, used: 40 } };
    renderAt(<FrameworkCapNotice />);
    await waitFor(() => expect(calls.filter((u) => u === '/entitlements')).toHaveLength(2));
    expect(screen.queryByTestId('framework-cap')).not.toBeInTheDocument();
  });
});
