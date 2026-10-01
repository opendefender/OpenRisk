// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #755 — the import page reports exactly what the server answered. A refused
// file lists every error by line and column and never shows a success; a
// success toast appears only when the server created at least one risk.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';

const post = vi.fn();
const toastSuccess = vi.fn();
const fetchRisks = vi.fn();

vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a), defaults: { baseURL: '' } },
}));
vi.mock('../../../hooks/useToast', () => ({
  useToast: () => ({ success: toastSuccess, error: vi.fn(), promise: vi.fn() }),
}));
vi.mock('../../../hooks/useRiskStore', () => ({
  useRiskStore: () => ({ fetchRisks }),
}));

import { ImportRisksPage } from '../../../pages/ImportRisks';
import { importErrorMessage, importFileSchema } from '../importRisksSchema';
import { useUIStore } from '../../../store/uiStore';

const tr = (_fr: string, en: string) => en;

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

async function chooseAndImport(
  name = 'register.csv',
  content = 'title,probability,impact\nA,0.5,5\n',
) {
  const user = userEvent.setup();
  render(
    <MemoryRouter>
      <ImportRisksPage />
    </MemoryRouter>,
  );
  await user.upload(
    screen.getByTestId('import-file-input'),
    new File([content], name, { type: 'text/csv' }),
  );
  await user.click(screen.getByRole('button', { name: /^import$/i }));
}

describe('ImportRisksPage', () => {
  beforeEach(() => {
    useUIStore.getState().setLang('en');
    post.mockReset();
    toastSuccess.mockReset();
    fetchRisks.mockReset();
  });

  it('shows the count the server created and refreshes the register', async () => {
    post.mockResolvedValue({
      data: { created: 3, rejected: 0, risk_ids: ['a', 'b', 'c'], errors: [] },
    });
    await chooseAndImport();

    expect(await screen.findByText('3 risk(s) imported.')).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(fetchRisks).toHaveBeenCalledTimes(1);
  });

  it('lists every error by line and column and shows no success when the file is refused', async () => {
    post.mockRejectedValue(
      httpError(422, {
        error: 'validation_failed',
        created: 0,
        rejected: 2,
        risk_ids: [],
        errors: [
          {
            line: 3,
            column: 'probability',
            message: 'probability must be between 0 and 1 (got 3)',
          },
          { line: 5, column: 'title', message: 'title is required' },
        ],
      }),
    );
    await chooseAndImport();

    const panel = await screen.findByTestId('import-outcome-error');
    expect(panel).toHaveTextContent('Nothing was imported: 2 error(s) on 2 row(s).');
    expect(panel).toHaveTextContent('probability must be between 0 and 1 (got 3)');
    expect(screen.getByRole('cell', { name: '5' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'title' })).toBeInTheDocument();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(fetchRisks).not.toHaveBeenCalled();
  });

  it('explains a plan limit instead of a generic failure', async () => {
    post.mockRejectedValue(httpError(402, { code: 'limit_reached', requested: 40, remaining: 5 }));
    await chooseAndImport();

    const panel = await screen.findByTestId('import-outcome-error');
    expect(panel).toHaveTextContent('The file has 40 risk(s); your plan allows 5 more.');
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it('refuses a non-CSV file before sending anything', async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(
      <MemoryRouter>
        <ImportRisksPage />
      </MemoryRouter>,
    );
    await user.upload(screen.getByTestId('import-file-input'), new File(['x'], 'register.xlsx'));

    expect(await screen.findByRole('alert')).toHaveTextContent('Only CSV files are accepted.');
    expect(screen.queryByRole('button', { name: /^import$/i })).not.toBeInTheDocument();
    await waitFor(() => expect(post).not.toHaveBeenCalled());
  });
});

describe('importFileSchema', () => {
  it('accepts a CSV and refuses empty or oversized files', () => {
    const schema = importFileSchema(tr);
    expect(schema.safeParse(new File(['a'], 'r.CSV')).success).toBe(true);
    expect(schema.safeParse(new File([], 'r.csv')).success).toBe(false);
    expect(schema.safeParse(new File([new Uint8Array(2 * 1024 * 1024 + 1)], 'r.csv')).success).toBe(
      false,
    );
  });

  it('renders server error codes in the reader’s language and falls back to the server text', () => {
    const fr = (f: string) => f;
    expect(
      importErrorMessage(
        {
          line: 3,
          column: 'probability',
          code: 'out_of_range',
          params: { min: '0', max: '1', value: '3' },
          message: 'probability must be between 0 and 1 (got 3)',
        },
        fr,
      ),
    ).toBe('Doit être entre 0 et 1 (valeur : 3).');
    expect(
      importErrorMessage({ line: 2, code: 'some_future_code', message: 'server text' }, fr),
    ).toBe('server text');
  });
});
