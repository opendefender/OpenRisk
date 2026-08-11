// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The risk lifecycle, read from the server.
//
// There is deliberately NO client-side copy of the state graph or of the
// guards. The last version had one, the server had another, and the two
// disagreed — which is how a risk reached "traité" with open sub-actions. The
// stepper renders exactly what GET /risks/:id/transitions says, including the
// blocked options and their reasons.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';

/** The single canonical lifecycle (backend domain.RiskState). */
export type RiskState =
  | 'draft'
  | 'identified'
  | 'assessed'
  | 'treatment_planned'
  | 'in_treatment'
  | 'residual_accepted'
  | 'mitigated'
  | 'closed'
  | 'reopened';

/** Which precondition a blocked transition failed. Drives the "way out" CTA. */
export type TransitionGuard = 'active_mitigation' | 'subactions_complete' | 'governance_approval';

export interface TransitionOption {
  to: RiskState;
  label: string;
  allowed: boolean;
  /** Empty when allowed; otherwise the concrete blocker, in the user's language. */
  reason?: string;
  guard?: TransitionGuard;
  is_forward: boolean;
}

export interface TransitionsView {
  current: RiskState;
  current_label: string;
  next?: RiskState;
  next_label?: string;
  blocked_reason?: string;
  step_index: number;
  step_count: number;
  options: TransitionOption[];
}

export const lifecycleKey = (riskId: string) => ['risks', riskId, 'transitions'] as const;

export function useRiskTransitions(riskId: string, enabled = true) {
  return useQuery({
    queryKey: lifecycleKey(riskId),
    enabled: enabled && Boolean(riskId),
    queryFn: async (): Promise<TransitionsView> => {
      const { data } = await api.get<TransitionsView>(`/risks/${riskId}/transitions`);
      return data;
    },
  });
}

export interface TransitionInput {
  to: RiskState;
  comment?: string;
}

export function useTransitionRisk(riskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: TransitionInput) => {
      const { data } = await api.post(`/risks/${riskId}/transition`, input);
      return data;
    },
    onSuccess: () => {
      // The transition changes the risk, its available next steps, and the
      // register row. Invalidate all three rather than patching one and letting
      // the others go stale — a stale stepper is how the old one started lying.
      void qc.invalidateQueries({ queryKey: lifecycleKey(riskId) });
      void qc.invalidateQueries({ queryKey: ['risks'] });
      void qc.invalidateQueries({ queryKey: ['mitigations'] });
    },
  });
}
