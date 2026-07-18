// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
