// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #746 — merging the organization switcher (#734) dropped the branding (#718)
// from the sidebar: the block went back to the login's org_name and to plain
// initials, and nothing failed but an unused import.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useAuthStore } from '../../../hooks/useAuthStore';
import { useUIStore } from '../../../store/uiStore';

vi.mock('../../../features/organization/useOrganization', async () => {
  const actual = await vi.importActual<
    typeof import('../../../features/organization/useOrganization')
  >('../../../features/organization/useOrganization');
  return {
    ...actual,
    useOrganizationBranding: () => ({ data: { name: 'Banque Atlantique', has_logo: true } }),
    useOrganizationCounts: () => ({ data: undefined }),
  };
});

import { Sidebar } from '../Sidebar';

type StoreUser = NonNullable<ReturnType<typeof useAuthStore.getState>['user']>;

describe('Sidebar organization block', () => {
  beforeEach(() => {
    useUIStore.setState({ lang: 'en' });
    useAuthStore.setState({
      user: {
        id: 'u-1',
        email: 'a@example.test',
        username: 'a',
        full_name: 'A',
        role: '',
        tenant_id: 'org-1',
        org_name: 'Espace de A',
        business_role: '',
      } as StoreUser,
    });
  });

  it('shows the branded name and logo, not the login fallback', () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <Sidebar />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    const trigger = screen.getByRole('button', { name: /Banque Atlantique/ });
    expect(trigger).toBeInTheDocument();
    expect(screen.queryByText('Espace de A')).not.toBeInTheDocument();
    // The organization's logo badge, not the switcher's default initials.
    expect(trigger.querySelector('[data-testid="org-logo"]')).not.toBeNull();
  });
});
