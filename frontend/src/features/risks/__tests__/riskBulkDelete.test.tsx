// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #1441 — a bulk delete on the risk register previews first and sends the
// preview's fingerprint, like the vulnerability and asset registers. The page
// is the real one; only the HTTP boundary is mocked, so what is asserted is the
// requests a user's clicks produce.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { useRiskStore } from '../../../hooks/useRiskStore';

const http = vi.hoisted(() => ({
  gets: [] as string[],
  posts: [] as { url: string; body: unknown }[],
  risks: [] as {
    id: string;
    title: string;
    score: number;
    probability: number;
    impact: number;
    status: string;
    tags: string[];
  }[],
  capabilities: ['delete'] as string[],
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('../../../lib/api', () => {
  const get = (url: string) => {
    http.gets.push(url);
    if (url === '/risks')
      return Promise.resolve({ data: { items: http.risks, total: http.risks.length } });
    if (url === '/risks/bulk/capabilities')
      return Promise.resolve({ data: { entity_type: 'risk', actions: http.capabilities } });
    return Promise.resolve({ data: {} });
  };
  const post = (url: string, body: unknown) => {
    http.posts.push({ url, body });
    if (url === '/risks/bulk/preview') {
      const ids = (body as { ids: string[] }).ids;
      return Promise.resolve({
        data: {
          action: 'delete',
          total: ids.length,
          affected: ids.length,
          unchanged: 0,
          missing: [],
          sample: [],
          fingerprint: 'fp-1441',
        },
      });
    }
    if (url === '/risks/bulk/delete') {
      const ids = (body as { ids: string[] }).ids;
      return Promise.resolve({
        data: { total: ids.length, applied: ids.length, ids, audited: ids.length },
      });
    }
    return Promise.resolve({ data: {} });
  };
  return {
    api: {
      get,
      post,
      put: () => Promise.resolve({ data: {} }),
      patch: () => Promise.resolve({ data: {} }),
      delete: () => Promise.resolve({ data: {} }),
      interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
    },
    onMFARequired: () => () => undefined,
  };
});

import { RiskRegisterPage } from '../RiskRegisterPage';

const risk = (id: string, title: string) => ({
  id,
  title,
  score: 5,
  probability: 0.5,
  impact: 5,
  status: 'OPEN',
  tags: [],
});

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/risks']}>
        <RiskRegisterPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function grant(permissions: string[]) {
  useAuthStore.setState((s) => ({
    ...s,
    user: { ...(s.user ?? {}), id: 'u1', tenant_id: 't1', permissions } as never,
  }));
}

async function selectTwo(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByTestId('row-select-r1'));
  await user.click(screen.getByTestId('row-select-r2'));
}

const posted = (url: string) => http.posts.filter((p) => p.url === url);

beforeEach(() => {
  useUIStore.setState({ lang: 'en' });
  useRiskStore.setState({ risks: [], total: 0, selectedIds: [] });
  http.posts.length = 0;
  http.gets.length = 0;
  http.risks = [risk('r1', 'Ransomware'), risk('r2', 'Phishing'), risk('r3', 'Insider')];
  http.capabilities = ['delete'];
  grant(['risks:read', 'risks:delete']);
  window.history.replaceState(null, '', '/risks');
});

describe('#1441 — risk register bulk delete goes through the preview gate', () => {
  it('previews first and deletes nothing before the confirm', async () => {
    const user = userEvent.setup();
    renderPage();
    await selectTwo(user);

    await user.click(await screen.findByTestId('bulk-action-delete'));

    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByTestId('bulk-preview-counts');
    expect(posted('/risks/bulk/preview')).toEqual([
      {
        url: '/risks/bulk/preview',
        body: { action: 'delete', ids: ['r1', 'r2'], status: undefined },
      },
    ]);
    expect(posted('/risks/bulk/delete')).toEqual([]);
    expect(screen.getByText('Ransomware')).toBeInTheDocument();
  });

  it('on confirm, sends ONE delete carrying the preview fingerprint', async () => {
    const user = userEvent.setup();
    renderPage();
    await selectTwo(user);
    await user.click(await screen.findByTestId('bulk-action-delete'));
    const dialog = await screen.findByRole('alertdialog');
    await within(dialog).findByTestId('bulk-preview-counts');

    http.risks = [risk('r3', 'Insider')];
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }));

    await waitFor(() => expect(posted('/risks/bulk/delete')).toHaveLength(1));
    expect(posted('/risks/bulk/delete')[0].body).toMatchObject({
      ids: ['r1', 'r2'],
      fingerprint: 'fp-1441',
    });
    // The legacy, ungoverned route is never used.
    expect(posted('/risks/bulk')).toEqual([]);
    await waitFor(() => expect(screen.queryByText('Ransomware')).not.toBeInTheDocument());
  });

  it('cancelling the preview sends no delete and keeps the selection', async () => {
    const user = userEvent.setup();
    renderPage();
    await selectTwo(user);
    await user.click(await screen.findByTestId('bulk-action-delete'));
    const dialog = await screen.findByRole('alertdialog');

    // The footer's Cancel, not the header's close icon.
    const cancel = within(dialog)
      .getAllByRole('button', { name: 'Cancel' })
      .find((b) => b.textContent === 'Cancel');
    await user.click(cancel as HTMLElement);

    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(posted('/risks/bulk/delete')).toEqual([]);
    expect(screen.getByTestId('bulk-count')).toHaveTextContent('2');
  });

  it('does not offer Delete to a user without risks:delete', async () => {
    grant(['risks:read']);
    const user = userEvent.setup();
    renderPage();
    await selectTwo(user);

    await waitFor(() => expect(http.gets).toContain('/risks/bulk/capabilities'));
    expect(screen.queryByTestId('bulk-action-delete')).not.toBeInTheDocument();
  });

  it('does not offer Delete when the server does not list it', async () => {
    http.capabilities = [];
    const user = userEvent.setup();
    renderPage();
    await selectTwo(user);

    // Wait for the capabilities answer to land before asserting the absence.
    await waitFor(() => expect(http.gets).toContain('/risks/bulk/capabilities'));
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.queryByTestId('bulk-action-delete')).not.toBeInTheDocument();
  });
});
