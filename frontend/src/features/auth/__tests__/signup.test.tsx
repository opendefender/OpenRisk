// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Regression fence around "on n'arrive pas à créer un compte".
//
// Signing up creates an organisation with you as its owner, and an owner's role
// mandates MFA — so login() after register ALWAYS stops short of a session and
// returns `mfa_enrollment_required`. The register form discarded that return,
// declared success and navigated, so the route guard bounced every brand-new
// user straight back to the login screen. Registration was broken for everyone,
// not in an edge case.

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

function renderSignup() {
  return render(
    <MemoryRouter>
      <AuthScreen initialView="register" />
    </MemoryRouter>,
  );
}

async function fillAndSubmit() {
  const user = userEvent.setup();
  await user.type(screen.getByTestId('register-name'), 'Alix Mensah');
  await user.type(screen.getByTestId('register-email'), 'alix@example.com');
  await user.type(screen.getByTestId('register-password'), 'MotDePasse2026!');
  await user.click(screen.getByTestId('register-submit'));
  return user;
}

describe('signing up', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    post.mockResolvedValue({ data: {} });
    setupMFA.mockResolvedValue({
      secret: 'ABCDEF',
      qr_code: '/9j/rawbase64',
      backup_codes: ['AAAA1111'],
    });
  });

  it('hands off to MFA enrolment instead of navigating with no session', async () => {
    // The only outcome a real signup produces.
    login.mockResolvedValue({ status: 'mfa_enrollment_required', mfa_token: 'enrol-token' });

    renderSignup();
    await fillAndSubmit();

    await waitFor(() => expect(setupMFA).toHaveBeenCalledWith('enrol-token'));
    // The user is on the enrolment screen…
    expect(await screen.findByText(/double authentification/i)).toBeInTheDocument();
    // …and was NOT sent into the app without a session, which is what the route
    // guard used to bounce straight back to /login.
    expect(navigate).not.toHaveBeenCalled();
  });

  it('renders the QR as a decodable image, not a relative URL', async () => {
    login.mockResolvedValue({ status: 'mfa_enrollment_required', mfa_token: 'enrol-token' });

    renderSignup();
    await fillAndSubmit();

    // An <img alt=""> is presentational, so it has no "img" role — select the
    // QR by the element itself.
    await screen.findByText(/scannez ce qr code/i);
    const img = document.querySelector('img[width="168"]') as HTMLImageElement;
    expect(img).not.toBeNull();
    // The backend returns raw base64; assigning it to src verbatim made the
    // browser fetch it as a path and show a broken image on the one screen a new
    // account cannot get past.
    expect(img.getAttribute('src')).toBe('data:image/jpeg;base64,/9j/rawbase64');
  });

  it('requests MFA setup exactly once per token', async () => {
    // POST /auth/mfa/setup is not idempotent — a second call answers 400
    // "duplicated key not allowed". React double-invokes effects in development,
    // and the failed second call painted "la création du compte a échoué" over a
    // screen that had loaded perfectly.
    login.mockResolvedValue({ status: 'mfa_enrollment_required', mfa_token: 'enrol-token' });

    renderSignup();
    await fillAndSubmit();

    await waitFor(() => expect(setupMFA).toHaveBeenCalled());
    expect(setupMFA).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/création du compte a échoué/i)).not.toBeInTheDocument();
  });

  it('never blames the sign-up when it is the second factor that failed', async () => {
    login.mockResolvedValue({ status: 'mfa_enrollment_required', mfa_token: 'enrol-token' });
    setupMFA.mockRejectedValue(new Error('boom'));

    renderSignup();
    await fillAndSubmit();

    // The account EXISTS at this point; telling the user their registration
    // failed sends them off to create it a second time.
    expect(await screen.findByText(/votre compte est créé/i)).toBeInTheDocument();
    expect(screen.queryByText(/création du compte a échoué/i)).not.toBeInTheDocument();
  });

  // #687: the form used to derive a username from the address's local part, a
  // field nobody saw. Two people who both write to contact@ collided, and the
  // second was told their email was already registered.
  it('sends no username, so people sharing an email local part can both sign up', async () => {
    login.mockResolvedValue({ status: 'signed_in' });

    renderSignup();
    await fillAndSubmit();

    await waitFor(() => expect(post).toHaveBeenCalled());
    const [url, body] = post.mock.calls[0] as [string, Record<string, unknown>];
    expect(url).toBe('/auth/register');
    expect(body).not.toHaveProperty('username');
    expect(body.email).toBe('alix@example.com');
  });

  // #716: the organisation used to be created as "<name> — espace", a name
  // nobody chose, which the danger zone then made the owner type to confirm.
  it('names the organisation after the person, with no invented suffix', async () => {
    login.mockResolvedValue({ status: 'signed_in' });

    renderSignup();
    await fillAndSubmit();

    await waitFor(() => expect(post).toHaveBeenCalled());
    const [, body] = post.mock.calls[0] as [string, Record<string, unknown>];
    expect(body.company_name).toBe('Alix Mensah');
  });

  it('never blames the email when the server refused the username', async () => {
    post.mockRejectedValue({
      isAxiosError: true,
      response: {
        status: 409,
        data: { error: 'This username is already taken', code: 'USERNAME_TAKEN' },
      },
    });

    renderSignup();
    await fillAndSubmit();

    expect(await screen.findByText(/username is already taken/i)).toBeInTheDocument();
    expect(screen.queryByText(/compte existe déjà avec cet e-mail/i)).not.toBeInTheDocument();
  });

  it('says so plainly when the address already has an account', async () => {
    post.mockRejectedValue({
      isAxiosError: true,
      response: {
        status: 409,
        data: {
          error: 'An account already exists for this email address',
          code: 'EMAIL_ALREADY_REGISTERED',
        },
      },
    });

    renderSignup();
    await fillAndSubmit();

    expect(await screen.findByText(/compte existe déjà avec cet e-mail/i)).toBeInTheDocument();
  });

  it('still goes straight in when no second factor is demanded', async () => {
    login.mockResolvedValue({ status: 'signed_in' });

    renderSignup();
    await fillAndSubmit();

    await waitFor(() => expect(navigate).toHaveBeenCalled());
    expect(setupMFA).not.toHaveBeenCalled();
  });
});
