// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/
//
// Typed client for the Smart Risk Calculation API (spec §8). Mirrors
// pkg/scoring's SmartResult / FactorScore and domain.RiskScoringWeights. Zero `any`.

import { api } from '../../lib/api';

/** The eight factors of the multifactor model (stable API keys). */
export type FactorKey =
  | 'business_criticality'
  | 'internet_exposure'
  | 'vulnerabilities'
  | 'control_maturity'
  | 'incident_history'
  | 'exploitability'
  | 'financial_value'
  | 'threat_intel';

/** Ordered factor keys — matches pkg/scoring.FactorKeys. */
export const FACTOR_KEYS: FactorKey[] = [
  'business_criticality',
  'internet_exposure',
  'vulnerabilities',
  'control_maturity',
  'incident_history',
  'exploitability',
  'financial_value',
  'threat_intel',
];

/** One factor's contribution to the smart score. */
export interface SmartFactorScore {
  key: FactorKey;
  label: string;
  weight: number; // normalised weight applied [0,1]
  value: number; // risk contribution [0,1] (1 = worst)
  contribution: number; // points added (value × weight × 100)
  detail: string;
}

/** The multifactor smart score + its radar-ready breakdown. */
export interface SmartRiskScore {
  score: number; // 0–100
  criticality: 'low' | 'medium' | 'high' | 'critical';
  factors: SmartFactorScore[];
  explanation: string;
}

/** A tenant's persisted factor weights. */
export interface RiskScoringWeights {
  id?: string;
  tenant_id?: string;
  business_criticality: number;
  internet_exposure: number;
  vulnerabilities: number;
  control_maturity: number;
  incident_history: number;
  exploitability: number;
  financial_value: number;
  threat_intel: number;
  updated_by?: string;
  created_at?: string;
  updated_at?: string;
}

/** The eight weights as sent to the simulator / update endpoint. */
export type FactorWeightsInput = Record<FactorKey, number>;

/** Maps a weights row to the flat FactorKey→number shape the API accepts. */
export function weightsToInput(w: RiskScoringWeights): FactorWeightsInput {
  return {
    business_criticality: w.business_criticality,
    internet_exposure: w.internet_exposure,
    vulnerabilities: w.vulnerabilities,
    control_maturity: w.control_maturity,
    incident_history: w.incident_history,
    exploitability: w.exploitability,
    threat_intel: w.threat_intel,
    financial_value: w.financial_value,
  };
}

export const smartScoreService = {
  /** Compute (and cache) the multifactor smart score for one risk. */
  getRiskSmartScore: async (riskId: string): Promise<SmartRiskScore> => {
    const res = await api.get<SmartRiskScore>(`/risks/${riskId}/smart-score`);
    return res.data;
  },

  /** Preview the smart score with custom weights (non-persisting). */
  simulate: async (riskId: string, weights: FactorWeightsInput): Promise<SmartRiskScore> => {
    const res = await api.post<SmartRiskScore>(`/risks/${riskId}/smart-score/simulate`, weights);
    return res.data;
  },

  /** The tenant's effective factor weights (custom or defaults). */
  getWeights: async (): Promise<RiskScoringWeights> => {
    const res = await api.get<RiskScoringWeights>('/risk-scoring/weights');
    return res.data;
  },

  /** Persist the tenant's custom factor weights (admin). */
  updateWeights: async (weights: FactorWeightsInput): Promise<RiskScoringWeights> => {
    const res = await api.put<RiskScoringWeights>('/risk-scoring/weights', weights);
    return res.data;
  },
};
