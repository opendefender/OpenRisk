// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The route guard (spec §4): the app is unreachable until the signup wizard is
// finished, and the wizard is unreachable once it is.
//
// The authority is the SERVER's onboarding.completed, never a local flag — the
// same principle as the activation checklist. Two consequences worth stating:
//
//   • While the state is loading we render NOTHING but a quiet placeholder. A
//     guard that guesses "not completed" during the first fetch would bounce
//     every returning user through the wizard on every cold load.
//   • If the state cannot be fetched at all, the guard FAILS OPEN. Locking an
//     existing customer out of their own GRC platform because an onboarding
//     endpoint is down would be a far worse failure than showing the app to
//     someone who has not finished a wizard.

import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';

import { useOnboardingState } from './useActivation';

/** Wraps the protected app shell. */
export function OnboardingGuard({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { data, isLoading, isError } = useOnboardingState();

  if (isLoading && !data) return <GuardPlaceholder />;
  if (isError || !data) return <>{children}</>; // fail open, on purpose

  if (!data.completed) {
    const target = `/onboarding/${data.current_step || 'organization'}`;
    if (pathname !== target) return <Navigate to={target} replace />;
  }
  return <>{children}</>;
}

/**
 * Wraps the wizard itself: someone who has already finished has no business
 * being sent back through it (a bookmark, a back button, a stale tab).
 */
export function OnboardingCompletedRedirect({ children }: { children: ReactNode }) {
  const { data, isLoading } = useOnboardingState();

  if (isLoading && !data) return <GuardPlaceholder />;
  if (data?.completed) return <Navigate to={data.landing || '/'} replace />;
  return <>{children}</>;
}

function GuardPlaceholder() {
  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ background: 'var(--bg-primary)' }}
    >
      <div
        className="h-8 w-8 rounded-full animate-spin"
        style={{ border: '3px solid var(--border-subtle)', borderTopColor: 'var(--accent, #2e6be6)' }}
        role="status"
        aria-label="Chargement…"
      />
    </div>
  );
}
