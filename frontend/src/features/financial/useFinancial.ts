// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
