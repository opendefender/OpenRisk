// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #910: "Lancer un scan" asks the agents for their packages and says plainly
// why it did not start: no agent online, or one less than 10 minutes ago.
// An open finding from the inventory shows its package and how sure the match is.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, type AxiosResponse } from 'axios';
import { toast } from 'sonner';

import { useUIStore } from '../../../store/uiStore';

const perms = { scan: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({ hasPermission: (p: string) => (p === 'scanner:scan' ? perms.scan : true) }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const post = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a), get: vi.fn(), defaults: { baseURL: '' } },
}));

import { PackageScanButton } from '../PackageScanButton';

function httpError(status: number) {
  return new AxiosError('x', String(status), undefined, undefined, {
    status,
    data: {},
  } as AxiosResponse);
}

function renderButton() {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PackageScanButton />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  perms.scan = true;
  post.mockReset();
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
  vi.mocked(toast.info).mockClear();
});

describe('PackageScanButton', () => {
  it('asks the agents and says how many', async () => {
    post.mockResolvedValue({ data: { agents: 2 } });
    renderButton();
    fireEvent.click(screen.getByTestId('vuln-scan'));
    await waitFor(() =>
      expect(toast.success).toHaveBeenCalledWith(
        'Inventaire demandé à 2 agents. Les vulnérabilités arrivent dans quelques minutes.',
      ),
    );
    expect(post).toHaveBeenCalledWith('/vulnerabilities/scan');
  });

  it('says no agent is online, with the way to set one up', async () => {
    post.mockRejectedValue(httpError(404));
    renderButton();
    fireEvent.click(screen.getByTestId('vuln-scan'));
    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    const [msg, opts] = vi.mocked(toast.error).mock.calls[0];
    expect(msg).toMatch(/Aucun agent en ligne/);
    expect((opts as { action: { label: string } }).action.label).toBe('Infrastructure');
    expect(post).toHaveBeenCalledTimes(1);
  });

  it('says a scan already ran, without retrying', async () => {
    post.mockRejectedValue(httpError(429));
    renderButton();
    fireEvent.click(screen.getByTestId('vuln-scan'));
    await waitFor(() =>
      expect(toast.info).toHaveBeenCalledWith(
        'Un scan a déjà été lancé il y a moins de 10 minutes.',
      ),
    );
    expect(post).toHaveBeenCalledTimes(1);
  });

  it('is not offered without scanner:scan', () => {
    perms.scan = false;
    renderButton();
    expect(screen.queryByTestId('vuln-scan')).toBeNull();
  });
});
