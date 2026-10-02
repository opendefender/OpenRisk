// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #688: a throttled sign-in or sign-up used to read "incorrect password" or
// "registration failed", which sent people to retry and only extended the wait.
// The screen now says it was throttled and for how long, in both languages.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';

const login = vi.fn();
const post = vi.fn();

vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a), defaults: { baseURL: '' } },
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
import { useUIStore } from '../../../store/uiStore';

// Shaped like an AxiosError: an Error carrying isAxiosError and a response.
function throttled(data: Record<string, unknown>, headers: Record<string, string> = {}) {
  return Object.assign(new Error('HTTP 429'), {
    isAxiosError: true,
    response: { status: 429, data, headers },
  });
}

async function signIn() {
  render(
    <MemoryRouter>
      <AuthScreen />
    </MemoryRouter>,
  );
  const user = userEvent.setup();
  await user.type(screen.getByTestId('login-email'), 'awa.diallo@example.com');
  await user.type(screen.getByTestId('login-password'), 'Ancre-Vitrail7-Cobalt');
  await user.click(screen.getByTestId('login-submit'));
}

async function signUp() {
  render(
    <MemoryRouter>
      <AuthScreen initialView="register" />
    </MemoryRouter>,
  );
  const user = userEvent.setup();
  await user.type(screen.getByTestId('register-name'), 'Alix Mensah');
  await user.type(screen.getByTestId('register-email'), 'alix@example.com');
  await user.type(screen.getByTestId('register-password'), 'MotDePasse2026!');
  await user.click(screen.getByTestId('register-submit'));
}

describe('throttled sign-in and sign-up', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useUIStore.setState({ lang: 'fr' });
    post.mockResolvedValue({ data: {} });
  });

  it('sign-in locked for the address: says how long, in French', async () => {
    login.mockRejectedValue(throttled({ code: 'LOGIN_LOCKED', retry_after: 900 }));
    await signIn();
    expect(
      await screen.findByText('Trop de tentatives. Réessayez dans 15 minutes.'),
    ).toBeInTheDocument();
    expect(screen.queryByText(/mot de passe incorrect/i)).not.toBeInTheDocument();
  });

  it('sign-in throttled per IP: says how long, in English, rounding up', async () => {
    useUIStore.setState({ lang: 'en' });
    login.mockRejectedValue(throttled({ code: 'RATE_LIMITED', retry_after: 61 }));
    await signIn();
    expect(
      await screen.findByText('Too many attempts. Try again in 2 minutes.'),
    ).toBeInTheDocument();
  });

  it('reads the Retry-After header when the body has no figure', async () => {
    login.mockRejectedValue(throttled({ error: true }, { 'retry-after': '30' }));
    await signIn();
    expect(
      await screen.findByText('Trop de tentatives. Réessayez dans 1 minute.'),
    ).toBeInTheDocument();
  });

  it('stays vague rather than guess when the server gives no figure', async () => {
    useUIStore.setState({ lang: 'en' });
    login.mockRejectedValue(throttled({ error: true }));
    await signIn();
    expect(
      await screen.findByText('Too many attempts. Try again in a few minutes.'),
    ).toBeInTheDocument();
  });

  it('sign-up throttled: no longer "registration failed", in French and English', async () => {
    post.mockImplementation((url: string) =>
      url === '/auth/register'
        ? Promise.reject(throttled({ code: 'RATE_LIMITED', retry_after: 600 }))
        : Promise.resolve({ data: {} }),
    );
    await signUp();
    expect(
      await screen.findByText('Trop de tentatives. Réessayez dans 10 minutes.'),
    ).toBeInTheDocument();
    expect(screen.queryByText(/création du compte a échoué/i)).not.toBeInTheDocument();
  });

  it('sign-up throttled in English', async () => {
    useUIStore.setState({ lang: 'en' });
    post.mockImplementation((url: string) =>
      url === '/auth/register'
        ? Promise.reject(throttled({ code: 'RATE_LIMITED', retry_after: 600 }))
        : Promise.resolve({ data: {} }),
    );
    await signUp();
    expect(
      await screen.findByText('Too many attempts. Try again in 10 minutes.'),
    ).toBeInTheDocument();
  });
});
