// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useQuery, useMutation } from '@tanstack/react-query';
import { financialService, type SimulateInput } from './financialService';

/** Tenant-wide financial summary (CFO/CISO dashboard). */
export function useFinancialSummary() {
  return useQuery({
    queryKey: ['financial', 'summary'],
    queryFn: financialService.getSummary,
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
