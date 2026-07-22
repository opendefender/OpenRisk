// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  smartScoreService,
  type FactorWeightsInput,
  type RiskScoringWeights,
} from './smartScoreService';

/** Multifactor smart score for one risk. `enabled` gates the fetch (drawer tab). */
export function useRiskSmartScore(riskId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ['smart-score', 'risk', riskId],
    queryFn: () => smartScoreService.getRiskSmartScore(riskId as string),
    enabled: Boolean(riskId) && enabled,
  });
}

/** Non-persisting weight simulation for the "tune the weighting live" preview. */
export function useSimulateSmartScore(riskId: string | undefined) {
  return useMutation({
    mutationFn: (weights: FactorWeightsInput) =>
      smartScoreService.simulate(riskId as string, weights),
  });
}

/** The tenant's smart-risk factor weights (custom or defaults). */
export function useRiskWeights() {
  return useQuery({
    queryKey: ['smart-score', 'weights'],
    queryFn: smartScoreService.getWeights,
  });
}

/** Persist the tenant's factor weights (admin). Invalidates cached weights + scores. */
export function useUpdateRiskWeights() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (weights: FactorWeightsInput) => smartScoreService.updateWeights(weights),
    onSuccess: (data: RiskScoringWeights) => {
      qc.setQueryData(['smart-score', 'weights'], data);
      // A weight change moves every risk's score → drop cached per-risk scores.
      qc.invalidateQueries({ queryKey: ['smart-score', 'risk'] });
    },
  });
}
