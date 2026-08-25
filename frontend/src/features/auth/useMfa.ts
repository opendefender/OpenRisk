// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// OR26-03 — MFA state and policy hooks.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchMFAStatus, fetchMFAPolicy, saveMFAPolicy } from './mfaPolicyService';

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
