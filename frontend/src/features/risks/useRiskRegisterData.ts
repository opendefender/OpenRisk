// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Data hooks of the register that are not the list itself: the summary behind
// the subtitle, the tab counts and the band pills; the members behind the owner
// pickers; and the money display (tenant currency, never hard-coded).

import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { ownershipService, type Assignee } from '../../services/ownershipService';
import { useFeature, useFeatureAtLeast } from '../billing/useEntitlements';
import { useFinancialSummary } from '../financial/useFinancial';
import { currencyLabel, toDisplay } from '../financial/money';
import { riskRegisterService, type RiskSummaryQuery } from './riskRegisterService';

export const RISK_SUMMARY_KEY = ['risks', 'summary'] as const;

export function useRiskSummary(query: RiskSummaryQuery) {
  return useQuery({
    queryKey: [...RISK_SUMMARY_KEY, query.view, query.q],
    queryFn: ({ signal }) => riskRegisterService.summary(query, signal),
    // Counts stay on screen while the next ones load: no flicker per keystroke.
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

const NO_MEMBERS: Assignee[] = [];

/** The tenant's members — the list the API validates ownership writes against. */
export function useRegisterMembers() {
  const q = useQuery({
    queryKey: ['ownership', 'assignable', 'risks'],
    queryFn: ({ signal }) => ownershipService.listAssignable({}, signal),
    staleTime: 60_000,
  });
  return { members: q.data?.users ?? NO_MEMBERS, isLoading: q.isLoading, isError: q.isError };
}

export interface RegisterMoney {
  /** The ALE column and cards exist only with the financial entitlement. */
  entitled: boolean;
  /** Label of the currency amounts are shown in ("FCFA", "EUR"…). */
  currency: string;
  /** XAF (canonical) → display currency. */
  toDisplay: (xaf: number) => number;
}

/**
 * The tenant's display currency and rate come from the financial summary, which
 * opens at Pro. Below that the figures are the canonical XAF ones and are
 * labelled as such — an honest label, not an assumed currency.
 */
export function useRegisterMoney(): RegisterMoney {
  const feature = useFeature('financial_quantification');
  const deep = useFeatureAtLeast('financial_quantification', 'on', 'pro');
  const summary = useFinancialSummary(!deep.loading && deep.enabled);
  const data = deep.enabled ? summary.data : undefined;
  const rate = data?.fx_rate_xaf;
  return {
    entitled: !feature.loading && feature.enabled,
    currency: currencyLabel(data?.currency),
    toDisplay: (xaf) => toDisplay(xaf, rate),
  };
}
