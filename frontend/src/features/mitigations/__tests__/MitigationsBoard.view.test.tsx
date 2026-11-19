// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #591 — the board's view (Kanban / Table / Gantt) lives in the URL, so
// /risks/mitigations?view=table opens the table and a view can be linked.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router';

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

function LocationProbe() {
  const loc = useLocation();
  return <output data-testid="search">{loc.search}</output>;
}

function renderAt(url: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[url]}>
        <MitigationsBoard />
        <LocationProbe />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const pressed = (v: string) => screen.getByTestId(`miti-view-${v}`).getAttribute('aria-pressed');

describe('MitigationsBoard — view follows ?view=', () => {
  beforeEach(() => {
    useUIStore.setState({ lang: 'en' });
    useAuthStore.setState({
      user: {
        id: 'u',
        email: 'u@example.test',
        username: 'u',
        full_name: 'U',
        role: '',
        tenant_id: 'org-a',
        permissions: ['mitigations:read'],
      },
    });
  });

  it('defaults to Kanban without a parameter', () => {
    renderAt('/risks/mitigations');
    expect(pressed('kanban')).toBe('true');
    expect(pressed('table')).toBe('false');
  });

  it('?view=table selects the table', () => {
    renderAt('/risks/mitigations?view=table');
    expect(pressed('table')).toBe('true');
    expect(pressed('kanban')).toBe('false');
  });

  it('an unknown value falls back to Kanban', () => {
    renderAt('/risks/mitigations?view=bogus');
    expect(pressed('kanban')).toBe('true');
  });

  it('choosing a view writes it to the URL, and Kanban clears it', () => {
    renderAt('/risks/mitigations?q=x');
    fireEvent.click(screen.getByTestId('miti-view-gantt'));
    expect(pressed('gantt')).toBe('true');
    expect(screen.getByTestId('search').textContent).toBe('?q=x&view=gantt');

    fireEvent.click(screen.getByTestId('miti-view-kanban'));
    expect(pressed('kanban')).toBe('true');
    expect(screen.getByTestId('search').textContent).toBe('?q=x');
  });
});
