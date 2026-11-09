// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #1315: when the server refuses a channel URL (the outbound guard answers 400
// with the offending field), the Channels tab shows that reason on save and on
// test instead of a bare "Failed" / "The test could not run". The generic copy
// stays as the fallback when the server sent nothing to show.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, type AxiosResponse } from 'axios';
import { toast } from 'sonner';

import { useUIStore } from '../../../store/uiStore';

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

const svc = vi.hoisted(() => ({
  getChannels: vi.fn(),
  channelCatalogue: vi.fn(),
  saveChannels: vi.fn(),
  testChannel: vi.fn(),
}));
vi.mock('../automationService', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../automationService')>();
  return { ...actual, automationService: { ...actual.automationService, ...svc } };
});

import { ChannelsView } from '../AutomationPage';

const REFUSED = 'webhook_url: target address is not allowed (must be a public https host)';

function http400(data: Record<string, unknown>) {
  return new AxiosError('Request failed with status code 400', '400', undefined, undefined, {
    status: 400,
    data,
  } as AxiosResponse);
}

function renderView() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <ChannelsView canWrite />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'en' });
  svc.getChannels.mockReset().mockResolvedValue({});
  svc.channelCatalogue.mockReset().mockResolvedValue([]);
  svc.saveChannels.mockReset();
  svc.testChannel.mockReset();
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

describe('ChannelsView — refused channel URL (#1315)', () => {
  it('shows the server reason when saving is refused', async () => {
    svc.saveChannels.mockRejectedValue(http400({ error: REFUSED }));
    renderView();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(`Channels not saved: ${REFUSED}`));
  });

  it('shows the server reason when a test is refused, in French too', async () => {
    useUIStore.setState({ lang: 'fr' });
    svc.testChannel.mockRejectedValue(http400({ error: REFUSED }));
    renderView();
    fireEvent.click(screen.getAllByRole('button', { name: 'Tester' })[0]);
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(`Test refusé : ${REFUSED}`));
  });

  it('falls back to the generic copy when the server sent no message', async () => {
    svc.saveChannels.mockRejectedValue(new Error('network down'));
    svc.testChannel.mockRejectedValue(http400({}));
    renderView();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed'));
    fireEvent.click(screen.getAllByRole('button', { name: 'Test' })[0]);
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('The test could not run'));
  });
});
