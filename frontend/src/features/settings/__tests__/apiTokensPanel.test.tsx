// #782 — Settings › API tokens.
//
// The screen used to type the create answer as `{ token?: string }` while the
// server sent an object, so the clipboard received "[object Object]" and the
// toast still announced success. These tests pin what reaches the clipboard and
// that nothing claims a success that did not happen.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';

import { ApiTokensPanel } from '../ApiTokensPanel';

const get = vi.fn();
const post = vi.fn();
const del = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: {
    get: (...a: unknown[]) => get(...a),
    post: (...a: unknown[]) => post(...a),
    delete: (...a: unknown[]) => del(...a),
    defaults: { baseURL: '' },
  },
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock('sonner', () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

const writeText = vi.fn();

const SECRET = 'orsk_a1b2c3d4_' + 'f'.repeat(64);
const en = (_fr: string, english: string) => english;

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter>
      <QueryClientProvider client={qc}>
        <ApiTokensPanel tr={en} lang="en" />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

async function generate(name = 'CI/CD') {
  const user = userEvent.setup();
  // userEvent installs its own clipboard; ours must be the one the panel sees.
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
  });
  await user.type(screen.getByLabelText('Token name'), name);
  await user.click(screen.getByRole('button', { name: 'Generate token' }));
}

beforeEach(() => {
  vi.clearAllMocks();
  get.mockResolvedValue({ data: { tokens: [] } });
  writeText.mockResolvedValue(undefined);
});

describe('ApiTokensPanel', () => {
  it('creates a personal access token and puts the orsk_ secret on the clipboard', async () => {
    post.mockResolvedValue({
      data: {
        id: 't1',
        name: 'CI/CD',
        token_prefix: 'a1b2c3d4',
        created_at: '2026-10-01T00:00:00Z',
        token: SECRET,
      },
    });
    renderPanel();
    await generate();

    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    expect(post).toHaveBeenCalledWith('/auth/pat', { name: 'CI/CD', scopes: ['*'] });

    const copied = writeText.mock.calls[0][0];
    expect(typeof copied).toBe('string');
    expect(copied).toMatch(/^orsk_/);
    expect(copied).not.toContain('[object Object]');
    expect(copied).toBe(SECRET);

    expect(toastSuccess).toHaveBeenCalledWith('Token created and copied to clipboard');
    // Shown once, so the user can still copy it if they missed the clipboard.
    expect(screen.getByLabelText('Token value')).toHaveValue(SECRET);
  });

  it('never copies an object nor announces success when no secret string comes back', async () => {
    // The pre-#782 answer shape: the secret nested in an object.
    post.mockResolvedValue({ data: { token: { id: 't1', value: SECRET } } });
    renderPanel();
    await generate();

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(writeText).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(screen.queryByLabelText('Token value')).not.toBeInTheDocument();
  });

  it('does not claim the token was copied when the clipboard refuses', async () => {
    post.mockResolvedValue({
      data: { id: 't1', name: 'CI/CD', token_prefix: 'a1b2c3d4', created_at: '', token: SECRET },
    });
    writeText.mockRejectedValue(new Error('denied'));
    renderPanel();
    await generate();

    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    expect(toastSuccess).not.toHaveBeenCalledWith('Token created and copied to clipboard');
    expect(toastSuccess).toHaveBeenCalledWith('Token created — copy it below');
    expect(screen.getByLabelText('Token value')).toHaveValue(SECRET);
  });

  it('announces nothing but the failure when the server refuses', async () => {
    post.mockRejectedValue(new Error('500'));
    renderPanel();
    await generate();

    await waitFor(() => expect(toastError).toHaveBeenCalledWith('Creation failed'));
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(writeText).not.toHaveBeenCalled();
  });

  it('rejects an over-long name client-side without calling the server', async () => {
    renderPanel();
    await generate('x'.repeat(101));

    expect(await screen.findByRole('alert')).toHaveTextContent('100 characters or fewer');
    expect(post).not.toHaveBeenCalled();
  });
});
