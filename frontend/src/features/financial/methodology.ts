// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The methodology shown for the portfolio figures of the financial page,
// built from the summary itself (spec §4).

import type { FinancialSummary, Methodology } from './financialService';

/** The methodology of the portfolio figures, built from the summary itself. */
export function portfolioMethodology(d: FinancialSummary, t: (key: string) => string): Methodology {
  const b = d.portfolio_loss;
  return {
    formula_version: d.formula_version || b.formula_version,
    model: t('financial.methodology.modelValue'),
    iterations: d.iterations || b.iterations,
    // `seed` is the Monte Carlo RNG seed the API returns so a run can be
    // reproduced; it is provenance from the server, not invented data.
    // eslint-disable-next-line openrisk/no-mock-data
    seed: b.seed,
    computed_at: d.computed_at,
    doc_url: '/docs/financial-quantification.md',
    currency: d.currency || 'XAF',
    fx_as_of: d.fx_as_of,
    fx_rate_xaf: d.fx_rate_xaf,
    inputs: [
      {
        key: 'risks',
        label: t('financial.methodology.risks'),
        value: d.total_risks,
        unit: t('financial.methodology.unitRisks'),
        source: 'derived',
      },
      {
        key: 'quantified',
        label: t('financial.methodology.quantified'),
        value: d.quantified_risks,
        unit: t('financial.methodology.unitRisks'),
        source: 'risk-input',
      },
    ],
    assumptions: [
      t('financial.methodology.a1'),
      t('financial.methodology.a2'),
      t('financial.methodology.a3'),
    ],
  };
}
