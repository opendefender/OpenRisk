// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #803: an SSO callback leaves the session in HttpOnly cookies and redirects to
// /auth/sso/complete. The screen must turn those cookies into a signed-in SPA
// and move on, or send the user back to /login with the generic error.

import { StrictMode } from 'react';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';

const claimSSOSession = vi.fn<() => Promise<string>>();
const claimSSOMFAStep = vi.fn<() => Promise<{ token: string; enrolling: boolean }>>();
const adoptSession = vi.fn<(token: string) => Promise<void>>();

vi.mock('../authService', () => ({
  claimSSOSession: () => claimSSOSession(),
  claimSSOMFAStep: () => claimSSOMFAStep(),
}));
// The real challenge and enrolment screens are AuthScreen's and tested there;
// here only which one is shown, with which token, and where a restart goes.
vi.mock('../AuthScreen', () => ({
  MFAChallenge: ({ token, onRestart }: { token: string; onRestart: (m: string) => void }) => (
    <button data-testid="mfa-challenge" onClick={() => onRestart('expired')}>
      {token}
    </button>
  ),
  MFAEnrollment: ({ token }: { token: string }) => <p data-testid="mfa-enrolment">{token}</p>,
}));
vi.mock('../AuthLayout', () => ({
  AuthLayout: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock('../../../hooks/useAuthStore', () => {
  const state = {
    user: { business_role: '' },
    adoptSession: (token: string) => adoptSession(token),
  };
  const useAuthStore = (sel?: (s: typeof state) => unknown) => (sel ? sel(state) : state);
  useAuthStore.getState = () => state;
  return { useAuthStore };
});
vi.mock('../../../shared/navModel', () => ({
  landingForBusinessRole: () => '/dashboard',
}));

import { SSOCompleteScreen } from '../SSOCompleteScreen';
import { safeNextPath } from '../safeNextPath';

function Where() {
  const loc = useLocation();
  return <p data-testid="where">{loc.pathname + loc.search}</p>;
}

function renderAt(url: string) {
  return render(
    <StrictMode>
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route path="/auth/sso/complete" element={<SSOCompleteScreen />} />
          <Route path="*" element={<Where />} />
        </Routes>
      </MemoryRouter>
    </StrictMode>,
  );
}

beforeEach(() => {
  claimSSOSession.mockReset();
  claimSSOMFAStep.mockReset();
  adoptSession.mockReset();
});

describe('SSOCompleteScreen', () => {
  it('adopts the session from the cookies and lands on the usual page', async () => {
    claimSSOSession.mockResolvedValue('access-1');
    adoptSession.mockResolvedValue();

    renderAt('/auth/sso/complete');

    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(await screen.findByTestId('where')).toHaveTextContent('/dashboard');
    expect(adoptSession).toHaveBeenCalledWith('access-1');
  });

  it('claims the session once, even under StrictMode', async () => {
    claimSSOSession.mockResolvedValue('access-1');
    adoptSession.mockResolvedValue();

    renderAt('/auth/sso/complete');

    await screen.findByTestId('where');
    // The refresh token is single-use: a second claim would revoke the session.
    expect(claimSSOSession).toHaveBeenCalledTimes(1);
  });

  it('goes to the page the sign-in started from', async () => {
    claimSSOSession.mockResolvedValue('access-1');
    adoptSession.mockResolvedValue();

    renderAt('/auth/sso/complete?next=' + encodeURIComponent('/risks?focus=7'));

    expect(await screen.findByTestId('where')).toHaveTextContent('/risks?focus=7');
  });

  it('ignores an off-site next', async () => {
    claimSSOSession.mockResolvedValue('access-1');
    adoptSession.mockResolvedValue();

    renderAt('/auth/sso/complete?next=' + encodeURIComponent('//evil.example/x'));

    expect(await screen.findByTestId('where')).toHaveTextContent('/dashboard');
  });

  it('sends the user back to the login screen when the session cannot be loaded', async () => {
    claimSSOSession.mockRejectedValue(new Error('HTTP 401'));

    renderAt('/auth/sso/complete?next=/risks');

    expect(await screen.findByTestId('where')).toHaveTextContent('/login?error=internal');
    expect(adoptSession).not.toHaveBeenCalled();
  });
});

describe('SSOCompleteScreen with a second factor owed (GHSA-rv5c-pr27-gpxh)', () => {
  it('asks for the enrolled authenticator and claims no session', async () => {
    claimSSOMFAStep.mockResolvedValue({ token: 'mfa-tok', enrolling: false });

    renderAt('/auth/sso/complete?mfa=1&next=/risks');

    expect(await screen.findByTestId('mfa-challenge')).toHaveTextContent('mfa-tok');
    expect(claimSSOMFAStep).toHaveBeenCalledTimes(1);
    expect(claimSSOSession).not.toHaveBeenCalled();
    expect(adoptSession).not.toHaveBeenCalled();
  });

  it('shows enrolment when the role mandates MFA', async () => {
    claimSSOMFAStep.mockResolvedValue({ token: 'enrol-tok', enrolling: true });

    renderAt('/auth/sso/complete?mfa=1');

    expect(await screen.findByTestId('mfa-enrolment')).toHaveTextContent('enrol-tok');
    expect(claimSSOSession).not.toHaveBeenCalled();
  });

  it('sends a spent step back to the login screen', async () => {
    claimSSOMFAStep.mockResolvedValue({ token: 'mfa-tok', enrolling: false });

    renderAt('/auth/sso/complete?mfa=1');
    fireEvent.click(await screen.findByTestId('mfa-challenge'));

    expect(await screen.findByTestId('where')).toHaveTextContent('/login?error=mfa_restart');
  });

  it('falls back to the generic error when the step cannot be read', async () => {
    claimSSOMFAStep.mockRejectedValue(new Error('HTTP 404'));

    renderAt('/auth/sso/complete?mfa=1');

    expect(await screen.findByTestId('where')).toHaveTextContent('/login?error=internal');
    expect(claimSSOSession).not.toHaveBeenCalled();
  });
});

describe('safeNextPath', () => {
  it.each([
    ['/risks', '/risks'],
    ['/risks?x=1#y', '/risks?x=1#y'],
    ['//evil.example', null],
    ['/\\evil.example', null],
    ['/\t/evil.example', null],
    ['/\n/evil.example', null],
    ['/risks\\..\\x', null],
    ['/\x7f/evil.example', null],
    ['https://evil.example', null],
    ['evil.example', null],
    ['', null],
    [null, null],
  ])('%s → %s', (raw, want) => {
    expect(safeNextPath(raw)).toBe(want);
  });
});
