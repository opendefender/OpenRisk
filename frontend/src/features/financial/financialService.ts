// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).
//
// Typed client for the Financial Risk Quantification API (spec §9). Mirrors
// pkg/crq's FinancialAssessment / FinancialSummary. Zero `any`.

import { api } from '../../lib/api';

/** An amount in both currencies (XAF is canonical, USD derived). */
export interface Money {
  xaf: number;
  usd: number;
}

/** Currency-aware amount: canonical XAF, USD, and the tenant display currency. */
export interface Amount {
  xaf: number;
  usd: number;
  value: number; // amount in `currency`
  currency: string; // tenant display currency (ISO code)
}

/** Supported tenant display currencies (spec §3). */
export const SUPPORTED_CURRENCIES = [
  'XAF',
  'XOF',
  'EUR',
  'USD',
  'NGN',
  'MAD',
  'GHS',
  'ZAR',
] as const;
export type CurrencyCode = (typeof SUPPORTED_CURRENCIES)[number];

/** FAIR-lite loss distribution: P10 / P50 (median) / P90 band + run metadata. */
export interface DistributionAmounts {
  p10: Amount;
  p50: Amount;
  p90: Amount;
  mean: Amount;
  iterations: number;
  seed: number;
  formula_version: string;
  lef: number;
}

/** One intrant surfaced in the methodology panel. */
export interface MethodologyInput {
  key: string;
  label: string;
  value: number;
  unit: string;
  source: string;
}

/** Explainability payload for a quantified figure (spec §4). */
export interface Methodology {
  formula_version: string;
  model: string;
  iterations: number;
  seed: number;
  computed_at: string;
  doc_url: string;
  currency: string;
  fx_as_of: string;
  fx_rate_xaf: number;
  inputs: MethodologyInput[];
  assumptions: string[];
}

export type SLEBasis = 'explicit' | 'composed' | 'reference';
export type ALEBasis = 'explicit' | 'reference';

/** Full monetary view of a single risk. */
export interface FinancialAssessment {
  distribution?: DistributionAmounts;
  methodology?: Methodology;
  sle: Money;
  sle_average: Money;
  sle_worst: Money;
  downtime_cost: Money;
  sle_basis: SLEBasis;
  aro: number;
  ale: Money;
  ale_average: Money;
  ale_worst: Money;
  ale_basis: ALEBasis;
  remediation_cost: Money;
  mitigation_effectiveness: number;
  ale_after: Money;
  risk_reduction: Money;
  rosi: number;
  rosi_computable: boolean;
}

export interface CriticalityBucket {
  criticality: string;
  count: number;
  ale: Money;
}

export interface TopRiskFinancial {
  id: string;
  title: string;
  criticality: string;
  ale: Money;
  ale_worst: Money;
  rosi: number;
  rosi_computable: boolean;
}

/** Tenant-wide financial posture for the CFO/CISO dashboard. */
export interface FinancialSummary {
  currency: string;
  fx_rate_xaf: number;
  fx_as_of: string;
  xaf_per_usd: number;
  computed_at: string;
  formula_version: string;
  iterations: number;
  total_risks: number;
  quantified_risks: number;
  /** Headline P10/P50/P90 band of total annual exposure (never one number). */
  portfolio_loss: DistributionAmounts;
  total_ale: Money;
  total_ale_worst: Money;
  total_ale_after: Money;
  total_risk_reduction: Money;
  total_remediation: Money;
  portfolio_rosi: number;
  portfolio_rosi_computable: boolean;
  by_criticality: CriticalityBucket[];
  top_risks: TopRiskFinancial[];
}

/** Per-field overrides for a what-if investment scenario (all optional). */
export interface SimulateInput {
  sle_xaf?: number;
  aro?: number;
  downtime_hours?: number;
  hourly_downtime_cost_xaf?: number;
  data_loss_cost_xaf?: number;
  fines_xaf?: number;
  other_direct_cost_xaf?: number;
  remediation_cost_xaf?: number;
  mitigation_effectiveness?: number;
}

export const financialService = {
  /** Portfolio-wide financial summary for the caller's tenant. */
  getSummary: async (): Promise<FinancialSummary> => {
    const res = await api.get<FinancialSummary>('/analytics/financial');
    return res.data;
  },

  /** Full financial assessment for one risk from its stored drivers. */
  getRiskFinancial: async (riskId: string): Promise<FinancialAssessment> => {
    const res = await api.get<FinancialAssessment>(`/risks/${riskId}/financial`);
    return res.data;
  },

  /** What-if assessment with overrides layered on the risk's stored drivers. */
  simulate: async (riskId: string, overrides: SimulateInput): Promise<FinancialAssessment> => {
    const res = await api.post<FinancialAssessment>(`/risks/${riskId}/simulate`, overrides);
    return res.data;
  },

  /** Change the tenant display currency (admin). Returns the normalized code. */
  setCurrency: async (currency: CurrencyCode): Promise<{ currency: string }> => {
    const res = await api.put<{ currency: string }>('/analytics/financial/currency', { currency });
    return res.data;
  },
};
