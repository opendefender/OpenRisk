// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #784: a password stored under the retired SHA-256 hasher is refused after the
// cutoff with 403 password_reset_required. The person typed the right password;
// the screen must say so and send them to the reset flow, not call it wrong.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router';

const login = vi.fn();

vi.mock('../../../lib/api', () => ({
  api: { post: vi.fn(), defaults: { baseURL: '' } },
}));
vi.mock('../authService', () => ({
  challengeMFA: vi.fn(),
  setupMFA: vi.fn(),
  verifyMFA: vi.fn(),
  requestPasswordReset: vi.fn(),
}));
vi.mock('../../../hooks/useAuthStore', () => {
  const state = {
    user: undefined,
    login: (...a: unknown[]) => login(...a),
  };
  const useAuthStore = (sel?: (s: typeof state) => unknown) => (sel ? sel(state) : state);
  useAuthStore.getState = () => state;
  return { useAuthStore };
});

import { AuthScreen } from '../AuthScreen';
import { ForgotPasswordScreen } from '../ForgotPasswordScreen';

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route path="/login" element={<AuthScreen />} />
        <Route path="/forgot-password" element={<ForgotPasswordScreen />} />
      </Routes>
    </MemoryRouter>,
  );
}

async function signIn(address: string) {
  const user = userEvent.setup();
  await user.type(screen.getByTestId('login-email'), address);
  await user.type(screen.getByTestId('login-password'), 'Ancre-Vitrail7-Cobalt');
  await user.click(screen.getByTestId('login-submit'));
  return user;
}

// Shaped like an AxiosError: an Error carrying isAxiosError and a response.
function refused(status: number, data: Record<string, string>) {
  return () =>
    Promise.reject(
      Object.assign(new Error(`HTTP ${status}`), {
        isAxiosError: true,
        response: { status, data },
      }),
    );
}

describe('sign-in refused because the password must be reset', () => {
  beforeEach(() => vi.clearAllMocks());

  it('says the password must be reset and links to the reset flow with the address filled in', async () => {
    login.mockImplementation(
      refused(403, {
        error: 'Your password must be reset before you can sign in.',
        code: 'password_reset_required',
      }),
    );

    renderLogin();
    const user = await signIn('legacy@example.com');

    expect(await screen.findByText(/ancien format qui n’est plus accepté/i)).toBeInTheDocument();
    expect(screen.queryByText(/mot de passe incorrect/i)).not.toBeInTheDocument();

    await user.click(screen.getByTestId('reset-required-link'));
    expect(await screen.findByTestId('forgot-email')).toHaveValue('legacy@example.com');
  });

  it('keeps the generic message and offers no reset link for a wrong password', async () => {
    login.mockImplementation(refused(401, { error: 'Invalid credentials' }));

    renderLogin();
    await signIn('someone@example.com');

    expect(await screen.findByText(/mot de passe incorrect/i)).toBeInTheDocument();
    expect(screen.queryByTestId('reset-required-link')).not.toBeInTheDocument();
  });

  it('does not treat another 403 as a reset request', async () => {
    login.mockImplementation(refused(403, { error: 'account is disabled' }));

    renderLogin();
    await signIn('someone@example.com');

    expect(await screen.findByText(/mot de passe incorrect/i)).toBeInTheDocument();
    expect(screen.queryByTestId('reset-required-link')).not.toBeInTheDocument();
  });
});
