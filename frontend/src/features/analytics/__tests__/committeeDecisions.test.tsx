// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Décisions attendues du comité" on the executive view (#903).
//
// What the live pass caught and these pin down:
//   - Reporter shows the dated note at once and rolls back if refused;
//   - a refused signature is sent once, not four times through the app-wide
//     mutation retry, and the row gets its buttons back;
//   - the requester's own summary is the sub-line when there is one.
//
// The server is mocked at the HTTP boundary.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import type { ApprovalRequest } from '../../governance/governanceService';

const posts = vi.fn<(url: string) => Promise<{ data: unknown }>>();
const toastError = vi.hoisted(() => vi.fn());

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: toastError } }));

function forbidden() {
  return Promise.reject(
    Object.assign(new Error('forbidden'), {
      isAxiosError: true,
      response: { status: 403, data: {} },
    }),
  );
}

function request(id: string, title: string, description = ''): ApprovalRequest {
  return {
    id,
    tenant_id: 't',
    entity_type: 'risk_acceptance',
    title,
    description,
    status: 'pending',
    current_step: 0,
    steps: [],
    decisions: [],
    deferrals: [],
    requested_by: 'u',
    requested_by_email: 'rssi@example.com',
    workflow_name: 'Acceptation de risque',
    mode: 'sequential',
    created_at: '2026-10-01T09:00:00Z',
  };
}

let pending: ApprovalRequest[] = [];

vi.mock('../../../lib/api', () => ({
  api: {
    get: (url: string) => {
      if (url.startsWith('/governance/approvals')) return Promise.resolve({ data: pending });
      return Promise.resolve({ data: {} });
    },
    post: (url: string) => posts(url),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

import { ExecutiveDashboard } from '../ExecutiveDashboard';

function renderView() {
  // The app's own mutation policy (main.tsx): three retries.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 3, retryDelay: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ExecutiveDashboard />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

async function row(title: string) {
  const rows = await screen.findAllByTestId('exec-decision');
  const r = rows.find((el) => el.textContent?.includes(title));
  if (!r) throw new Error(`no row ${title}`);
  return within(r);
}

beforeEach(() => {
  posts.mockReset();
  toastError.mockReset();
  useUIStore.setState({ lang: 'fr' });
  pending = [
    request(
      'a1',
      'Acceptation du risque R-0115',
      'Exposition 2,1 M FCFA/an, sous le seuil de tolérance',
    ),
    request('b2', 'Budget du PRA (M-284)'),
  ];
});

describe('#903 — committee decisions', () => {
  it('uses the requester summary as the sub-line, else type and requester', async () => {
    renderView();
    expect(
      (await row('R-0115')).getByText('Exposition 2,1 M FCFA/an, sous le seuil de tolérance'),
    ).toBeInTheDocument();
    expect(
      (await row('M-284')).getByText(/Acceptation de risque · demandée par rssi@example.com/),
    ).toBeInTheDocument();
  });

  it('Reporter shows the dated note at once', async () => {
    let resolve: (v: { data: unknown }) => void = () => undefined;
    posts.mockImplementation(() => new Promise((r) => (resolve = r)));
    renderView();
    fireEvent.click((await row('M-284')).getByRole('button', { name: 'Reporter' }));

    await waitFor(async () =>
      expect((await row('M-284')).getByText(/Reportée le/)).toBeInTheDocument(),
    );
    expect(posts).toHaveBeenCalledWith('/governance/approvals/b2/defer');
    resolve({ data: {} });
  });

  it('a refused deferral rolls back and is sent once', async () => {
    posts.mockImplementation(forbidden);
    renderView();
    fireEvent.click((await row('M-284')).getByRole('button', { name: 'Reporter' }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith('Vous ne pouvez pas signer cette étape.'),
    );
    expect((await row('M-284')).getByRole('button', { name: 'Reporter' })).toBeInTheDocument();
    expect(posts).toHaveBeenCalledTimes(1);
  });

  it('a refused signature is sent once and the buttons come back', async () => {
    posts.mockImplementation(forbidden);
    renderView();
    fireEvent.click((await row('R-0115')).getByRole('button', { name: 'Approuver' }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith('Vous ne pouvez pas signer cette étape.'),
    );
    const r = await row('R-0115');
    expect(r.getByRole('button', { name: 'Approuver' })).toBeInTheDocument();
    expect(r.queryByText('Approuvé')).toBeNull();
    expect(posts).toHaveBeenCalledTimes(1);
    expect(posts).toHaveBeenCalledWith('/governance/approvals/a1/decide');
  });

  it('an accepted signature marks the row approved', async () => {
    posts.mockResolvedValue({ data: {} });
    renderView();
    fireEvent.click((await row('R-0115')).getByRole('button', { name: 'Approuver' }));
    expect(await (await row('R-0115')).findByText('Approuvé')).toBeInTheDocument();
    expect(posts).toHaveBeenCalledTimes(1);
  });
});
