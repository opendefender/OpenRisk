// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// OR26-03 — MFA state and policy hooks.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchMFAStatus, fetchMFAPolicy, saveMFAPolicy, type MFAStatus } from './mfaPolicyService';
import { disableMFA } from './authService';
import type { Lang } from '../../store/uiStore';

/** Shared key so any flow that changes MFA state can invalidate the banner. */
export const MFA_STATUS_KEY = ['auth', 'mfa-status'] as const;
export const MFA_POLICY_KEY = ['auth', 'mfa-policy'] as const;

/**
 * The caller's MFA state.
 *
 * Long staleTime on purpose: this changes a handful of times in an account's
 * life, and the flows that change it (enrolling, an admin moving the window)
 * invalidate the key explicitly. Refetching it per navigation would be a request
 * per page for a value that is almost always identical — and enforcement does
 * not depend on this query in any case, the server guard does.
 */
export function useMFAStatus() {
  return useQuery({
    queryKey: MFA_STATUS_KEY,
    queryFn: fetchMFAStatus,
    staleTime: 5 * 60_000,
    // One retry: a transient failure should not paint an error banner over an
    // account that is perfectly fine.
    retry: 1,
  });
}

export function useMFAPolicy() {
  return useQuery({ queryKey: MFA_POLICY_KEY, queryFn: fetchMFAPolicy, staleTime: 60_000 });
}

export function useSaveMFAPolicy() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (graceDays: number) => saveMFAPolicy(graceDays),
    onSuccess: (policy) => {
      qc.setQueryData(MFA_POLICY_KEY, policy);
      // A shorter window can make the caller's own state change, so re-ask.
      void qc.invalidateQueries({ queryKey: MFA_STATUS_KEY });
    },
  });
}

/** Call after a successful enrolment so the banner disappears immediately. */
export function useInvalidateMFAStatus() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: MFA_STATUS_KEY });
}

/**
 * Turns MFA off (#754).
 *
 * Not optimistic: the server has to check the password first, and showing
 * "MFA off" before it agrees would tell the user an unprotected account is
 * what they have when it is not. Once the server says yes, the panel changes
 * at once and a refetch confirms it.
 */
export function useDisableMFA() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ password, locale }: { password: string; locale: Lang }) =>
      disableMFA(password, locale),
    // Never replay: every request is a password guess the server counts against
    // a five-per-quarter-hour budget. The app-wide retry of 3 would spend four
    // of them on one wrong password and lock the user out on their second try.
    retry: false,
    onSuccess: () => {
      qc.setQueryData<MFAStatus | null>(MFA_STATUS_KEY, (prev) =>
        prev ? { ...prev, state: 'recommended', configured: false } : prev,
      );
      void qc.invalidateQueries({ queryKey: MFA_STATUS_KEY });
    },
  });
}
