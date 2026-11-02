// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { useAuthStore } from '../../hooks/useAuthStore';
import { landingForBusinessRole } from '../../shared/navModel';
import { Skeleton } from '../../shared/ds/States';
import { useUIStore } from '../../store/uiStore';
import { AuthLayout } from './AuthLayout';
import { MFAChallenge, MFAEnrollment } from './AuthScreen';
import { claimSSOMFAStep, claimSSOSession, type SSOMFAStep } from './authService';
import { authCopy } from './authStrings';
import { safeNextPath } from './safeNextPath';

/**
 * Where an SSO sign-in lands (#803).
 *
 * The callback already set the session cookies. This screen loads the profile
 * and permissions from them, then moves on to `next` or to the user's usual
 * landing page. Any failure goes back to the login screen with the generic
 * error the login screen already knows how to say.
 *
 * With `mfa=1` there is no session yet (GHSA-rv5c-pr27-gpxh): the account has
 * an authenticator, or a role that mandates one, and the identity provider only
 * proved the first factor. The screen collects the pending step and shows the
 * same challenge or enrolment as a password sign-in.
 */
export function SSOCompleteScreen() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const adoptSession = useAuthStore((s) => s.adoptSession);
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);

  // The refresh token is single-use. A second claim with the same cookie (the
  // StrictMode double effect in development) would look like token reuse and
  // revoke the whole session, so the claim runs once per mount.
  const claimed = useRef(false);
  const [mfa, setMfa] = useState<SSOMFAStep | null>(null);

  useEffect(() => {
    if (claimed.current) return;
    claimed.current = true;

    const next = safeNextPath(params.get('next'));
    void (async () => {
      try {
        // The MFA step is read once and the cookie spent, so it obeys the same
        // once-per-mount rule as the refresh token below.
        if (params.get('mfa') === '1') {
          setMfa(await claimSSOMFAStep());
          return;
        }
        await adoptSession(await claimSSOSession());
        navigate(next ?? landingForBusinessRole(useAuthStore.getState().user?.business_role), {
          replace: true,
        });
      } catch {
        navigate('/login?error=internal', { replace: true });
      }
    })();
  }, [adoptSession, navigate, params]);

  // A spent or refused step can only be replaced by a new sign-in. Stable, so
  // the enrolment screen's setup effect does not run again on a re-render.
  const restart = useCallback(
    () => navigate('/login?error=mfa_restart', { replace: true }),
    [navigate],
  );

  if (mfa) {
    const next = safeNextPath(params.get('next'));
    return (
      <AuthLayout>
        {mfa.enrolling ? (
          <MFAEnrollment token={mfa.token} next={next} onRestart={restart} />
        ) : (
          <MFAChallenge
            token={mfa.token}
            next={next}
            onCancel={() => navigate('/login', { replace: true })}
            onRestart={restart}
          />
        )}
      </AuthLayout>
    );
  }

  return (
    <div
      className="min-h-screen flex items-center justify-center px-4"
      style={{ background: 'var(--bg-app)' }}
    >
      <div className="w-full max-w-sm flex flex-col gap-3" role="status" aria-live="polite">
        <span className="text-sm text-fg-secondary">{copy.ssoCompleting}</span>
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    </div>
  );
}
