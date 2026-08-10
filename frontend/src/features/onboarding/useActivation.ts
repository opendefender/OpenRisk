// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// TanStack Query hooks over the activation & onboarding endpoints, plus the
// celebration effect.
//
// Invalidation is the point of this file. The old checklist stopped ticking
// because nothing told it to look again after a mutation; here every mutation
// that can complete a step invalidates ACTIVATION_QUERY_KEY, and the key is
// exported so any feature that creates a risk / imports a framework / invites a
// teammate can do the same.

import { useEffect, useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import {
  activationService,
  i18n,
  type ActivationState,
  type ActivationStep,
  type Lang,
  type OnboardingState,
  type OnboardingStepKey,
} from '../../services/activationService';
import { confetti } from '../../shared/celebrate';

/** Shared key — invalidate it after any mutation that can complete a step. */
export const ACTIVATION_QUERY_KEY = ['activation', 'state'];
export const ONBOARDING_QUERY_KEY = ['onboarding', 'state'];
export const ONBOARDING_SUGGESTIONS_KEY = ['onboarding', 'suggestions'];

/** The activation checklist, straight from the server. */
export function useActivationState(enabled = true) {
  return useQuery({
    queryKey: ACTIVATION_QUERY_KEY,
    queryFn: () => activationService.getState(),
    enabled,
    staleTime: 15_000,
  });
}

/**
 * Invalidate activation state. Call after creating a risk, importing a
 * framework, adding an asset, planning a mitigation, inviting a teammate or
 * generating a report — the panel then re-reads the server rather than guessing.
 */
export function useInvalidateActivation() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ACTIVATION_QUERY_KEY });
  };
}

/**
 * Fires the celebration for steps the SERVER says to celebrate, then reports it
 * so it never fires again for this user.
 *
 * Idempotence has three layers, because this is exactly where the old
 * implementation went wrong:
 *   1. the server only sets `celebrate` while the acknowledgement is missing;
 *   2. this hook keeps an in-flight set so a re-render during the round-trip
 *      cannot fire twice;
 *   3. the acknowledgement endpoint is idempotent.
 *
 * Under prefers-reduced-motion there is no burst — the milestone is announced as
 * a toast instead, so the feedback is never simply lost.
 */
export function useCelebrateActivation(state: ActivationState | undefined, lang: Lang) {
  const qc = useQueryClient();
  const inFlight = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (!state) return;
    const pending = state.steps.filter((s) => s.celebrate && !inFlight.current.has(s.key));
    if (pending.length === 0) return;

    const reduced =
      typeof window !== 'undefined' &&
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true;

    pending.forEach((step) => {
      inFlight.current.add(step.key);

      if (reduced) {
        toast.success(
          `${lang === 'fr' ? 'Étape terminée' : 'Step complete'} — ${i18n(step.label_i18n, lang)}`,
        );
      } else {
        // A slightly bigger burst for the step that IS the product's promise.
        confetti(step.primary ? 90 : 55);
      }

      activationService
        .markCelebrated(step.key)
        .then(() => qc.invalidateQueries({ queryKey: ACTIVATION_QUERY_KEY }))
        .catch(() => {
          // Let it be retried on the next load rather than swallowing the
          // milestone: an unacknowledged step simply celebrates again later.
          inFlight.current.delete(step.key);
        });
    });
  }, [state, lang, qc]);
}

/** The wizard's resumable state; also the source of truth for the route guard. */
export function useOnboardingState(enabled = true) {
  return useQuery({
    queryKey: ONBOARDING_QUERY_KEY,
    queryFn: () => activationService.getOnboardingState(),
    enabled,
    // The guard reads this on every protected navigation: keep it fresh but not
    // chatty.
    staleTime: 30_000,
    retry: 1,
  });
}

export function useSaveOnboardingStep() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      step,
      answers,
      next,
    }: {
      step: OnboardingStepKey;
      answers: Record<string, unknown>;
      next?: OnboardingStepKey;
    }) => activationService.saveStep(step, answers, next),
    onSuccess: (state: OnboardingState) => {
      qc.setQueryData(ONBOARDING_QUERY_KEY, state);
      // The profile step completes a checklist row server-side.
      void qc.invalidateQueries({ queryKey: ACTIVATION_QUERY_KEY });
      void qc.invalidateQueries({ queryKey: ONBOARDING_SUGGESTIONS_KEY });
    },
  });
}

export function useCompleteOnboarding() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => activationService.complete(),
    onSuccess: (state: OnboardingState) => {
      qc.setQueryData(ONBOARDING_QUERY_KEY, state);
      void qc.invalidateQueries({ queryKey: ACTIVATION_QUERY_KEY });
    },
  });
}

/** Sector/goal-driven suggestions; overrides preview a choice before saving it. */
export function useOnboardingSuggestions(params?: {
  industry?: string;
  country?: string;
  goal?: string;
}) {
  return useQuery({
    queryKey: [...ONBOARDING_SUGGESTIONS_KEY, params?.industry ?? '', params?.country ?? '', params?.goal ?? ''],
    queryFn: () => activationService.getSuggestions(params),
    staleTime: 5 * 60_000,
  });
}

/** Convenience: the next step the user should act on, or undefined when done. */
export function nextActionableStep(state: ActivationState | undefined): ActivationStep | undefined {
  if (!state) return undefined;
  // The primary step first if it is still open — it is the one that proves the
  // product. Otherwise the first incomplete step in order.
  const primary = state.steps.find((s) => s.primary && !s.completed);
  return primary ?? state.steps.find((s) => !s.completed);
}
