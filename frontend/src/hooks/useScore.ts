// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// THE hook. Every surface that shows a score calls this one — the dashboard hero,
// the sidebar footer, the dedicated score page, the risk drawer, the asset
// drawer.
//
// Why one hook rather than "each screen fetches what it needs": the query key is
// what makes them agree. Two surfaces asking for the same (scope, id) share a
// single TanStack Query cache entry, so they render the same object, from the
// same fetch, at the same instant. It is no longer possible for the dashboard and
// the sidebar to hold two different answers — not because we remembered to keep
// them in sync, but because there is only one answer in memory.

import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  scoreService,
  type Score,
  type ScorePreviewInput,
  type ScoreScope,
} from '../services/scoreService';

/** Shared key: same (scope, id) → same cache entry → same rendered number. */
export const scoreQueryKey = (scope: ScoreScope, id?: string) => ['score', scope, id ?? ''];

export const SCORE_MODEL_QUERY_KEY = ['score', 'model'];

/**
 * Read the canonical score.
 *
 * `id` is required for the risk and asset scopes; the query stays disabled until
 * it arrives, so a drawer that has not chosen a row yet does not fetch a score
 * for `undefined`.
 */
export function useScore(scope: ScoreScope, id?: string) {
  return useQuery({
    queryKey: scoreQueryKey(scope, id),
    queryFn: ({ signal }) => scoreService.get(scope, id, signal),
    enabled: scope === 'tenant' || !!id,
    // Long enough that a dashboard full of widgets makes one request, short
    // enough that the number is not stale after a mutation elsewhere.
    staleTime: 30_000,
  });
}

/**
 * Invalidate scores after a mutation that can move one (creating a risk,
 * completing a mitigation, importing a framework, closing an incident).
 *
 * Invalidating the whole `['score']` family is deliberate: a new risk changes the
 * tenant score AND its own, and asking each call site to work out which keys it
 * touched is how caches drift.
 */
export function useInvalidateScores() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['score'] });
  };
}

/** The model's self-description — scale, bands, weights — for the explainer. */
export function useScoreModel() {
  return useQuery({
    queryKey: SCORE_MODEL_QUERY_KEY,
    queryFn: () => scoreService.model(),
    // The model changes only when the formula version does.
    staleTime: 60 * 60_000,
  });
}

/**
 * Live score preview for a form, debounced at 300 ms.
 *
 * It calls the SAME model as the persisted score, on the server. The alternative
 * — a quick multiplication in the component — is precisely the class of code this
 * change exists to delete: it would drift from the real formula the first time a
 * weight changed, and it would drift silently, while the user was deciding.
 *
 * Returns the previous score while a new one is in flight, so the number does not
 * flicker to zero on every keystroke.
 */
export function useScorePreview(input: ScorePreviewInput, enabled = true) {
  const [debounced, setDebounced] = useState(input);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(input), 300);
    return () => window.clearTimeout(timer);
    // Compared by value: the caller rebuilds the object on every render, so a
    // reference check would re-arm the timer forever and never fire.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [input.probability, input.impact, input.asset_criticality, input.mitigation_effectiveness, input.scope]);

  return useQuery<Score>({
    queryKey: ['score', 'preview', debounced],
    queryFn: ({ signal }) => scoreService.preview(debounced, signal),
    enabled,
    // Keep the last good number on screen while the next one loads, so the score
    // does not flicker to "—" on every keystroke.
    placeholderData: keepPreviousData,
    staleTime: 0,
  });
}
