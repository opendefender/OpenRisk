// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  financialService,
  type CurrencyCode,
  type FinancialSummary,
  type SimulateInput,
} from './financialService';

/** Shared query key so mutations can invalidate the summary (real recompute). */
export const FINANCIAL_SUMMARY_KEY = ['financial', 'summary'] as const;

/** Tenant-wide financial summary (CFO/CISO dashboard). */
export function useFinancialSummary() {
  return useQuery({
    queryKey: FINANCIAL_SUMMARY_KEY,
    queryFn: financialService.getSummary,
  });
}

/** Change the tenant display currency, then invalidate the summary so every
 * figure re-converts (spec §3: currency changeable). */
export function useSetCurrency() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (currency: CurrencyCode) => financialService.setCurrency(currency),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: FINANCIAL_SUMMARY_KEY });
    },
  });
}

/**
 * Save the risk appetite (#904). Optimistic: the summary carries the new value
 * at once, and goes back to the stored one if the server refuses. No retry —
 * a 403 for a non-admin is final.
 */
export function useSetAppetite() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (appetiteXaf: number) => financialService.setAppetite(appetiteXaf),
    retry: false,
    onMutate: async (appetiteXaf: number) => {
      await qc.cancelQueries({ queryKey: FINANCIAL_SUMMARY_KEY });
      const prev = qc.getQueryData<FinancialSummary>(FINANCIAL_SUMMARY_KEY);
      if (prev)
        qc.setQueryData<FinancialSummary>(FINANCIAL_SUMMARY_KEY, {
          ...prev,
          risk_appetite_xaf: appetiteXaf,
        });
      return { prev };
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(FINANCIAL_SUMMARY_KEY, ctx.prev);
    },
  });
}

/** Full financial assessment for one risk. `enabled` gates the fetch. */
export function useRiskFinancial(riskId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ['financial', 'risk', riskId],
    queryFn: () => financialService.getRiskFinancial(riskId as string),
    enabled: Boolean(riskId) && enabled,
  });
}

/** What-if simulation mutation (non-persisting). */
export function useSimulateFinancial(riskId: string | undefined) {
  return useMutation({
    mutationFn: (overrides: SimulateInput) =>
      financialService.simulate(riskId as string, overrides),
  });
}
