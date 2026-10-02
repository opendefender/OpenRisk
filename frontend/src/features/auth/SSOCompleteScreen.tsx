// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router';

import { useAuthStore } from '../../hooks/useAuthStore';
import { landingForBusinessRole } from '../../shared/navModel';
import { Skeleton } from '../../shared/ds/States';
import { useUIStore } from '../../store/uiStore';
import { claimSSOSession } from './authService';
import { authCopy } from './authStrings';
import { safeNextPath } from './safeNextPath';

/**
 * Where an SSO sign-in lands (#803).
 *
 * The callback already set the session cookies. This screen loads the profile
 * and permissions from them, then moves on to `next` or to the user's usual
 * landing page. Any failure goes back to the login screen with the generic
 * error the login screen already knows how to say.
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

  useEffect(() => {
    if (claimed.current) return;
    claimed.current = true;

    const next = safeNextPath(params.get('next'));
    void (async () => {
      try {
        await adoptSession(await claimSSOSession());
        navigate(next ?? landingForBusinessRole(useAuthStore.getState().user?.business_role), {
          replace: true,
        });
      } catch {
        navigate('/login?error=internal', { replace: true });
      }
    })();
  }, [adoptSession, navigate, params]);

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
