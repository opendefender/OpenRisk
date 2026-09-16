// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #656 — the sidebar said "Enterprise" under every organisation's name,
// whatever its plan.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import type { Entitlements } from '../../../services/entitlementService';
import { useUIStore } from '../../../store/uiStore';

const getEntitlements = vi.fn();

vi.mock('../../../services/entitlementService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/entitlementService')>(
    '../../../services/entitlementService',
  );
  return {
    ...actual,
    entitlementService: {
      ...actual.entitlementService,
      get: (...a: unknown[]) => getEntitlements(...a),
    },
  };
});

import { OrgPlanLabel } from '../OrgPlanLabel';

function renderLabel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <OrgPlanLabel />
    </QueryClientProvider>,
  );
}

const withPlan = (plan: Entitlements['plan']) => ({ plan }) as Entitlements;

describe('OrgPlanLabel', () => {
  beforeEach(() => {
    getEntitlements.mockReset();
    useUIStore.setState({ lang: 'en' });
  });

  it('shows Free for a Free organisation, never Enterprise', async () => {
    getEntitlements.mockResolvedValue(withPlan('free'));
    renderLabel();
    await waitFor(() => expect(screen.getByTestId('org-plan')).toHaveTextContent('Free plan'));
    expect(screen.queryByText(/Enterprise/)).toBeNull();
  });

  it('shows Pro for a Pro organisation, localised', async () => {
    useUIStore.setState({ lang: 'fr' });
    getEntitlements.mockResolvedValue(withPlan('pro'));
    renderLabel();
    await waitFor(() => expect(screen.getByTestId('org-plan')).toHaveTextContent('Plan Pro'));
  });

  it('shows a skeleton while loading, not a guessed plan', () => {
    getEntitlements.mockReturnValue(new Promise(() => {}));
    renderLabel();
    expect(screen.getByTestId('org-plan-loading')).toBeInTheDocument();
    expect(screen.queryByTestId('org-plan')).toBeNull();
  });

  it('omits the line when entitlements fail to load', async () => {
    getEntitlements.mockRejectedValue(new Error('500'));
    renderLabel();
    await waitFor(() => expect(screen.queryByTestId('org-plan-loading')).toBeNull());
    expect(screen.queryByTestId('org-plan')).toBeNull();
  });
});
