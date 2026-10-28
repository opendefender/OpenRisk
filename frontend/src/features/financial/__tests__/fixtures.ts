// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// A GET /analytics/financial body with every field the API always sends
// (FinancialSummary in internal/application/risk/financial_summary.go), so a
// test that renders the page never feeds it a shape the server cannot produce.

import type { FinancialSummary } from '../financialService';

export const M = 1_000_000;
const money = (xaf: number) => ({ xaf, usd: xaf / 600 });
const amount = (xaf: number) => ({ xaf, usd: xaf / 600, value: xaf, currency: 'XAF' });

export function financialSummary(over: Partial<FinancialSummary> = {}): FinancialSummary {
  return {
    currency: 'XAF',
    fx_rate_xaf: 1,
    fx_as_of: '2026-10-01T00:00:00Z',
    xaf_per_usd: 600,
    computed_at: '2026-10-09T08:00:00Z',
    formula_version: 'fair-lite-1.0.0',
    iterations: 10000,
    total_risks: 9,
    quantified_risks: 9,
    data_status: 'ok',
    min_quantified_risks: 1,
    portfolio_loss: {
      p10: amount(60 * M),
      p50: amount(100 * M),
      p90: amount(180 * M),
      p95: amount(190 * M),
      mean: amount(100 * M),
      iterations: 10000,
      seed: 1,
      formula_version: 'fair-lite-1.0.0',
      lef: 1,
    },
    total_ale: money(117.2 * M),
    total_ale_worst: money(200 * M),
    total_ale_after: money(48.5 * M),
    total_risk_reduction: money(68.7 * M),
    total_remediation: money(62.2 * M),
    portfolio_rosi: 0.1,
    portfolio_rosi_computable: true,
    by_criticality: [],
    top_risks: [
      {
        id: 'aaaaaaaa-0000-4000-8000-000000000001',
        title: 'Rançongiciel sur le SI cœur bancaire',
        criticality: 'critical',
        ale: money(42 * M),
        ale_worst: money(80 * M),
        ale_after: money(18.4 * M),
        rosi: 1,
        rosi_computable: true,
      },
      {
        id: 'bbbbbbbb-0000-4000-8000-000000000002',
        title: 'Fraude interne sur les virements',
        criticality: 'high',
        ale: money(8.4 * M),
        ale_worst: money(16 * M),
        ale_after: money(3 * M),
        rosi: 1,
        rosi_computable: true,
      },
    ],
    portfolio_rosi_3y: 1.85,
    portfolio_rosi_3y_computable: true,
    treated_risks: 10,
    // Uniform years: inherent 0..200 M, after plan 0..50 M.
    loss_exceedance: {
      inherent: Array.from({ length: 101 }, (_, p) => 2 * p * M),
      residual: Array.from({ length: 101 }, (_, p) => 0.5 * p * M),
    },
    treatments: [
      {
        risk_id: 'cccccccc-0000-4000-8000-000000000003',
        title: 'Double validation des virements',
        cost: money(2 * M),
        reduction: money(4.8 * M),
        payback_months: 5,
      },
      {
        risk_id: 'dddddddd-0000-4000-8000-000000000004',
        title: 'Second bureau de service SWIFT',
        cost: money(5 * M),
        reduction: money(1.4 * M),
        payback_months: 42.9,
      },
    ],
    risk_appetite_xaf: 80 * M,
    ...over,
  };
}
