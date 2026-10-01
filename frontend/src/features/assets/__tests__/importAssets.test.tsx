// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #861 — the asset import page reports exactly what the server answered, with
// the same all-or-nothing contract as the risk import.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';

const post = vi.fn();
const toastSuccess = vi.fn();

vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a), defaults: { baseURL: '' } },
}));
vi.mock('../../../hooks/useToast', () => ({
  useToast: () => ({ success: toastSuccess, error: vi.fn(), promise: vi.fn() }),
}));

import { ImportAssetsPage } from '../ImportAssetsPage';
import { useUIStore } from '../../../store/uiStore';

function httpError(status: number, data: unknown): AxiosError {
  const response = {
    status,
    data,
    statusText: '',
    headers: {},
    config: { headers: new AxiosHeaders() },
  } as AxiosResponse;
  return new AxiosError('request failed', String(status), undefined, undefined, response);
}

async function chooseAndImport() {
  const user = userEvent.setup();
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ImportAssetsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  await user.upload(
    screen.getByTestId('import-file-input'),
    new File(['name\nA\n'], 'inventory.csv', { type: 'text/csv' }),
  );
  await user.click(screen.getByRole('button', { name: /^(import|importer)$/i }));
  return invalidate;
}

describe('ImportAssetsPage', () => {
  beforeEach(() => {
    useUIStore.getState().setLang('en');
    post.mockReset();
    toastSuccess.mockReset();
  });

  it('posts to /assets/import, shows the count and refreshes the inventory', async () => {
    post.mockResolvedValue({
      data: { created: 2, rejected: 0, asset_ids: ['a', 'b'], errors: [] },
    });
    const invalidate = await chooseAndImport();

    expect(await screen.findByText('2 asset(s) imported.')).toBeInTheDocument();
    expect(post.mock.calls[0]?.[0]).toBe('/assets/import');
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['assets'] });
  });

  it('lists asset-specific errors in French and shows no success', async () => {
    useUIStore.getState().setLang('fr');
    post.mockRejectedValue(
      httpError(422, {
        created: 0,
        rejected: 2,
        errors: [
          {
            line: 2,
            column: 'name',
            code: 'asset_exists',
            params: { value: 'Kiosk' },
            message: 'an asset named "Kiosk" is already in the inventory',
          },
          {
            line: 3,
            column: 'criticality',
            code: 'invalid_criticality',
            params: { value: 'urgent' },
            message: '"urgent" is not a criticality',
          },
        ],
      }),
    );
    await chooseAndImport();

    const panel = await screen.findByTestId('import-outcome-error');
    expect(panel).toHaveTextContent('Un actif « Kiosk » existe déjà dans l’inventaire.');
    expect(panel).toHaveTextContent('« urgent » n’est pas une criticité');
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it('explains the plan’s asset limit', async () => {
    post.mockRejectedValue(httpError(402, { code: 'limit_reached', requested: 40, remaining: 5 }));
    await chooseAndImport();

    const panel = await screen.findByTestId('import-outcome-error');
    expect(panel).toHaveTextContent('The file has 40 asset(s); your plan allows 5 more.');
  });
});
