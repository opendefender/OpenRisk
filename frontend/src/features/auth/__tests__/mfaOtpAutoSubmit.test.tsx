// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 5 — the MFA enrolment field's `onComplete` submits the form
// instead of waiting for a separate button press, guarded so a fast
// paste/edit while the request is in flight cannot fire it twice, and the
// stale error clears the moment the user edits the code rather than lingering
// until the next submit.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';

const navigate = vi.fn();
const login = vi.fn();
const adoptSession = vi.fn();
const post = vi.fn();
const setupMFA = vi.fn();
const verifyMFA = vi.fn();
const challengeMFA = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return { ...actual, useNavigate: () => navigate };
});
vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a), defaults: { baseURL: '' } },
}));
vi.mock('../authService', () => ({
  setupMFA: (...a: unknown[]) => setupMFA(...a),
  verifyMFA: (...a: unknown[]) => verifyMFA(...a),
  challengeMFA: (...a: unknown[]) => challengeMFA(...a),
  listSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
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

function renderAt(initialView: 'login' | 'register') {
  return render(
    <MemoryRouter>
      <AuthScreen initialView={initialView} />
    </MemoryRouter>,
  );
}

/** Drives a registration all the way to the MFA enrolment field. */
async function reachEnrolment() {
  const user = userEvent.setup();
  login.mockResolvedValue({ status: 'mfa_enrollment_required', mfa_token: 'enrol-token' });
  renderAt('register');
  await user.type(screen.getByTestId('register-name'), 'Alix Mensah');
  await user.type(screen.getByTestId('register-email'), 'alix@example.com');
  await user.type(screen.getByTestId('register-password'), 'MotDePasse2026!');
  await user.click(screen.getByTestId('register-submit'));
  await waitFor(() => expect(setupMFA).toHaveBeenCalled());
  return { user, input: await screen.findByTestId('mfa-enrol-code') };
}

beforeEach(() => {
  vi.clearAllMocks();
  post.mockResolvedValue({ data: {} });
  setupMFA.mockResolvedValue({
    secret: 'ABCDEF',
    qr_code: '/9j/rawbase64',
    backup_codes: ['ABCDEFGH2345'],
  });
});

describe('the MFA enrolment field auto-submits on completion (#751 phase 5)', () => {
  it('submits once the sixth digit lands, with no button press', async () => {
    verifyMFA.mockResolvedValue({});
    const { user, input } = await reachEnrolment();

    await user.click(input);
    await user.paste('654321');

    await waitFor(() => expect(verifyMFA).toHaveBeenCalledTimes(1));
    expect(verifyMFA).toHaveBeenCalledWith('654321', 'enrol-token');
  });

  it('does not fire a second submit while the first is still in flight', async () => {
    let resolveVerify!: (v: unknown) => void;
    verifyMFA.mockReturnValue(new Promise((resolve) => (resolveVerify = resolve)));
    const { user, input } = await reachEnrolment();

    await user.click(input);
    await user.paste('654321');
    await waitFor(() => expect(verifyMFA).toHaveBeenCalledTimes(1));

    // A re-render (or a caret re-focus firing another onValueChange with the
    // unchanged value) must not queue a second request while `busy` is true.
    await user.click(input);
    await user.paste('654321');

    resolveVerify({});
    await waitFor(() => expect(navigate).toHaveBeenCalled());
    expect(verifyMFA).toHaveBeenCalledTimes(1);
  });

  it('clears a rejected code’s error as soon as the user edits it', async () => {
    verifyMFA.mockRejectedValueOnce(new Error('invalid'));
    const { user, input } = await reachEnrolment();

    await user.click(input);
    await user.paste('111111');

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());

    // Shake remounts its children on a new errorKey (see Shake.tsx), so the
    // field just submitted is a detached node now — re-query the live one.
    // jsdom also has no real layout, so a click-then-Backspace can land the
    // caret anywhere; clearing is the unambiguous way to trigger onValueChange.
    await user.clear(screen.getByTestId('mfa-enrol-code'));

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
