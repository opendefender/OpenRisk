// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #739 — a read-only member was offered "Add plan", which the server refuses
// (mitigations:create). The control now follows the permission.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';

import { useAuthStore } from '../../../hooks/useAuthStore';
import { useUIStore } from '../../../store/uiStore';

vi.mock('../useMitigations', () => ({
  useMitigations: () => ({
    items: [],
    columns: [],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock('../useMitigationEvents', () => ({ useMitigationEvents: vi.fn() }));

import { MitigationsBoard } from '../MitigationsBoard';

function signInWith(permissions: string[]) {
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

function renderBoard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <MitigationsBoard />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('MitigationsBoard — create control follows mitigations:create', () => {
  beforeEach(() => useUIStore.setState({ lang: 'en' }));

  it('TestAddPlan_Unauthorized — a read-only member is not offered it', () => {
    signInWith(['mitigations:read', 'risks:read']);
    renderBoard();
    expect(screen.queryByRole('button', { name: /Add plan/ })).toBeNull();
  });

  it('TestAddPlan_Success — a member who may create sees it', () => {
    signInWith(['mitigations:read', 'mitigations:create']);
    renderBoard();
    expect(screen.getByRole('button', { name: /Add plan/ })).toBeInTheDocument();
  });

  it('an administrator (wildcard) sees it', () => {
    signInWith(['*']);
    renderBoard();
    expect(screen.getByRole('button', { name: /Add plan/ })).toBeInTheDocument();
  });
});
