// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  entitlementService,
  telemetryService,
  orgDeletionService,
  type PlanKey,
  type RegionKey,
} from '../../services/entitlementService';

export const ENTITLEMENTS_KEY = ['entitlements'] as const;
export const BILLING_KEY = ['billing'] as const;

/** The resolved plan/features/limits/usage snapshot. Cached 60s; drives the whole
 *  paywall UX (grey + explain). */
export function useEntitlements() {
  return useQuery({
    queryKey: ENTITLEMENTS_KEY,
    queryFn: entitlementService.get,
    staleTime: 60_000,
  });
}

/** Convenience: is a feature enabled on the current plan, and which plan unlocks
 *  it. Returns a safe default (locked) while loading. */
export function useFeature(feature: string): { enabled: boolean; requiredPlan: PlanKey; loading: boolean } {
  const { data, isLoading } = useEntitlements();
  const f = data?.features?.[feature];
  return {
    enabled: f?.enabled ?? false,
    requiredPlan: (f?.required_plan ?? 'pro') as PlanKey,
    loading: isLoading,
  };
}

export function useBilling() {
  return useQuery({ queryKey: BILLING_KEY, queryFn: entitlementService.getBilling });
}

function useBillingMutation<TArgs>(fn: (a: TArgs) => Promise<unknown>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: BILLING_KEY });
      qc.invalidateQueries({ queryKey: ENTITLEMENTS_KEY });
    },
  });
}

export function useStartTrial() {
  return useBillingMutation((v: { plan: PlanKey; region: RegionKey }) =>
    entitlementService.startTrial(v.plan, v.region),
  );
}

export function useCheckout() {
  return useMutation({
    mutationFn: (v: { plan: PlanKey; region: RegionKey; provider?: string }) =>
      entitlementService.checkout(v.plan, v.region, v.provider),
  });
}

export function useChangePlan() {
  return useBillingMutation((v: { plan: PlanKey; region: RegionKey }) =>
    entitlementService.changePlan(v.plan, v.region),
  );
}

export function useCancelSubscription() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => entitlementService.cancel(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: BILLING_KEY });
      qc.invalidateQueries({ queryKey: ENTITLEMENTS_KEY });
    },
  });
}

// Telemetry ------------------------------------------------------------------
export const TELEMETRY_KEY = ['telemetry'] as const;
export function useTelemetry() {
  return useQuery({ queryKey: TELEMETRY_KEY, queryFn: telemetryService.get });
}
export function useSetTelemetry() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => telemetryService.set(enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: TELEMETRY_KEY }),
  });
}

// Danger zone ----------------------------------------------------------------
export const ORG_DELETION_KEY = ['org-deletion'] as const;
export function useOrgDeletion() {
  return useQuery({ queryKey: ORG_DELETION_KEY, queryFn: orgDeletionService.get });
}
export function useRequestOrgDeletion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { confirmName: string; mfaCode: string; reason: string }) =>
      orgDeletionService.request(v.confirmName, v.mfaCode, v.reason),
    onSuccess: () => qc.invalidateQueries({ queryKey: ORG_DELETION_KEY }),
  });
}
export function useCancelOrgDeletion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => orgDeletionService.cancel(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ORG_DELETION_KEY }),
  });
}
