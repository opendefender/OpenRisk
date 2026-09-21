// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #695 — the sidebar labelled every founder and administrator "Membre": it read
// an org role /auth/me never sends and defaulted to member.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import type { RBACCatalog } from '../../../features/rbac/rbacService';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { useUIStore } from '../../../store/uiStore';

const getCatalog = vi.fn();

vi.mock('../../../features/rbac/rbacService', async () => {
  const actual = await vi.importActual<typeof import('../../../features/rbac/rbacService')>(
    '../../../features/rbac/rbacService',
  );
  return { ...actual, rbacService: { getCatalog: (...a: unknown[]) => getCatalog(...a) } };
});

import { SidebarRoleLabel } from '../SidebarRoleLabel';

const ORG = 'org-1';

const catalog: RBACCatalog = {
  permissions: [],
  business_roles: [
    {
      key: 'auditor',
      label_fr: 'Auditeur',
      label_en: 'Auditor',
      description_fr: '',
      description_en: '',
      permissions: [],
      default_landing: '/',
    },
  ],
};

type StoreUser = NonNullable<ReturnType<typeof useAuthStore.getState>['user']>;

function signIn(overrides: Partial<StoreUser>) {
  useAuthStore.setState({
    user: {
      id: 'u-1',
      email: 'a@example.test',
      username: 'a',
      full_name: 'A',
      role: '',
      tenant_id: ORG,
      business_role: '',
      ...overrides,
    } as StoreUser,
  });
}

function renderLabel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SidebarRoleLabel />
    </QueryClientProvider>,
  );
}

describe('SidebarRoleLabel', () => {
  beforeEach(() => {
    getCatalog.mockReset();
    getCatalog.mockResolvedValue(catalog);
    useUIStore.setState({ lang: 'en' });
  });

  it('calls the founder Owner, not Member — the role comes from the token, not /auth/me', () => {
    // `role` is what /auth/me leaves empty; the org role is in org_roles.
    signIn({ role: '', org_roles: { [ORG]: 'root' } });
    renderLabel();
    expect(screen.getByTestId('sidebar-role')).toHaveTextContent('Owner');
  });

  it('calls an administrator Administrator, in French too', () => {
    useUIStore.setState({ lang: 'fr' });
    signIn({ org_roles: { [ORG]: 'admin' } });
    renderLabel();
    expect(screen.getByTestId('sidebar-role')).toHaveTextContent('Administrateur');
  });

  it('reads the role for the ACTIVE organisation only', () => {
    signIn({ org_roles: { 'other-org': 'root', [ORG]: 'user' } });
    renderLabel();
    expect(screen.getByTestId('sidebar-role')).toHaveTextContent('Member');
  });

  it('prefers the business role, labelled by the server catalogue', async () => {
    signIn({ business_role: 'auditor', org_roles: { [ORG]: 'user' } });
    renderLabel();
    await waitFor(() => expect(screen.getByTestId('sidebar-role')).toHaveTextContent('Auditor'));
  });

  it('omits the line when the role is unknown, rather than defaulting to Member', () => {
    signIn({ org_roles: {} });
    renderLabel();
    expect(screen.queryByTestId('sidebar-role')).toBeNull();
    expect(screen.queryByText(/Member|Membre/)).toBeNull();
  });
});
