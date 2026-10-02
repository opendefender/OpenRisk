// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #872 — the MFA code screen after #689. A wrong code still says "incorrect
// code". A sign-in that cannot take another code goes back to the password and
// says why. A locked account says how long to wait and stops taking codes.

import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';

import { useUIStore } from '../../../store/uiStore';
import { authCopy } from '../authStrings';

const navigate = vi.fn();
const login = vi.fn();
const adoptSession = vi.fn();
const challengeMFA = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return { ...actual, useNavigate: () => navigate };
});
vi.mock('../../../lib/api', () => ({
  api: { post: vi.fn(), defaults: { baseURL: '' } },
}));
vi.mock('../authService', () => ({
  setupMFA: vi.fn(),
  verifyMFA: vi.fn(),
  challengeMFA: (...a: unknown[]) => challengeMFA(...a),
}));
vi.mock('../../../hooks/useAuthStore', () => {
  const state = {
    user: undefined,
    login: (...a: unknown[]) => login(...a),
    adoptSession: (...a: unknown[]) => adoptSession(...a),
  };
  const useAuthStore = (sel?: (s: typeof state) => unknown) => (sel ? sel(state) : state);
  useAuthStore.getState = () => state;
  return { useAuthStore };
});

import { AuthScreen } from '../AuthScreen';

function refused(status: number, data: unknown): AxiosError {
  const response = {
    status,
    statusText: '',
    data,
    headers: {},
    config: { headers: new AxiosHeaders() },
  } as AxiosResponse;
  return new AxiosError('refused', String(status), response.config, null, response);
}

async function reachChallenge() {
  const user = userEvent.setup();
  login.mockResolvedValue({ status: 'mfa_required', mfa_token: 'challenge-token' });
  render(
    <MemoryRouter>
      <AuthScreen initialView="login" />
    </MemoryRouter>,
  );
  await user.type(screen.getByTestId('login-email'), 'alix@example.com');
  await user.type(screen.getByTestId('login-password'), 'MotDePasse2026!');
  await user.click(screen.getByTestId('login-submit'));
  await screen.findByTestId('mfa-code');
  return user;
}

async function submitCode(user: ReturnType<typeof userEvent.setup>, code = '123456') {
  await user.type(screen.getByTestId('mfa-code'), code);
  await user.click(screen.getByTestId('mfa-submit'));
}

const copy = () => authCopy(useUIStore.getState().lang);

beforeEach(() => {
  vi.clearAllMocks();
  useUIStore.getState().setLang('fr');
});

afterEach(() => {
  useUIStore.getState().setLang('fr');
});

describe('MFA challenge refusals (#872)', () => {
  it('keeps the code field for a wrong code', async () => {
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(400, { error: 'invalid MFA code' }));
    await submitCode(user);

    expect(await screen.findByTestId('auth-error')).toHaveTextContent(copy().mfaInvalid);
    expect(screen.getByTestId('mfa-code')).toBeEnabled();
  });

  it.each(['MFA_CHALLENGE_EXHAUSTED', 'TOKEN_REVOKED'])(
    'sends the user back to the password on %s, with the reason',
    async (code) => {
      const user = await reachChallenge();
      challengeMFA.mockRejectedValueOnce(refused(401, { code }));
      await submitCode(user);

      expect(await screen.findByTestId('login-password')).toHaveValue('MotDePasse2026!');
      expect(screen.queryByTestId('mfa-code')).toBeNull();
      expect(screen.getByTestId('auth-error')).toHaveTextContent(copy().mfaExhausted);
    },
  );

  it('says the step expired when the challenge token did', async () => {
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(401, { code: 'TOKEN_EXPIRED' }));
    await submitCode(user);

    await screen.findByTestId('login-password');
    expect(screen.getByTestId('auth-error')).toHaveTextContent(copy().mfaExpired);
  });

  it('starting over reaches the code step again with a fresh token', async () => {
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(401, { code: 'MFA_CHALLENGE_EXHAUSTED' }));
    await submitCode(user);
    await screen.findByTestId('login-password');

    login.mockResolvedValueOnce({ status: 'mfa_required', mfa_token: 'fresh-token' });
    await user.click(screen.getByTestId('login-submit'));
    await screen.findByTestId('mfa-code');
    challengeMFA.mockResolvedValueOnce({ token_pair: { access_token: 'a' } });
    await submitCode(user);

    await waitFor(() => expect(challengeMFA).toHaveBeenLastCalledWith('123456', 'fresh-token'));
  });

  it('says how long a locked account must wait and stops taking codes', async () => {
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(429, { code: 'MFA_LOCKED', retry_after: 900 }));
    await submitCode(user);

    expect(await screen.findByTestId('auth-error')).toHaveTextContent(copy().mfaLocked(15));
    expect(screen.getByTestId('mfa-code')).toBeDisabled();
    expect(screen.getByTestId('mfa-submit')).toBeDisabled();
  });

  it('gives the code field back when the lock ends', async () => {
    // Simulated clock: the lock lasts 15 minutes, and a real wait of even one
    // second is flaky when the suite runs in parallel.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const user = await reachChallenge();
      challengeMFA.mockRejectedValueOnce(refused(429, { code: 'MFA_LOCKED', retry_after: 900 }));
      await submitCode(user);
      expect(await screen.findByTestId('mfa-code')).toBeDisabled();

      act(() => {
        vi.advanceTimersByTime(899_000);
      });
      expect(screen.getByTestId('mfa-code')).toBeDisabled();

      act(() => {
        vi.advanceTimersByTime(2_000);
      });
      expect(screen.getByTestId('mfa-code')).toBeEnabled();
      expect(screen.queryByTestId('auth-error')).toBeNull();
      expect(screen.getByTestId('mfa-code')).toHaveFocus();
    } finally {
      vi.useRealTimers();
    }
  });

  it('reads the per-address limit as a wait of unknown length', async () => {
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(429, { error: true, msg: 'Rate limit exceeded' }));
    await submitCode(user);

    expect(await screen.findByTestId('auth-error')).toHaveTextContent(copy().mfaLockedUnknown);
    expect(screen.getByTestId('mfa-code')).toBeDisabled();
  });

  it('speaks English to an English session', async () => {
    useUIStore.getState().setLang('en');
    const user = await reachChallenge();
    challengeMFA.mockRejectedValueOnce(refused(401, { code: 'MFA_CHALLENGE_EXHAUSTED' }));
    await submitCode(user);

    expect(await screen.findByTestId('auth-error')).toHaveTextContent(
      'Too many wrong codes for this sign-in. Sign in again with your password.',
    );
  });
});
