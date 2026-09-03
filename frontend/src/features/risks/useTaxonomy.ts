// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  taxonomyService,
  type RiskCategory,
  type RiskControlMapping,
} from '../../services/taxonomyService';

export const CATEGORIES_KEY = ['risk-categories'] as const;
export const IMPORTED_FRAMEWORKS_KEY = ['compliance', 'frameworks', 'imported'] as const;
export const mappingsKey = (riskId: string) => ['risks', riskId, 'control-mappings'] as const;
export const UNMAPPED_KEY = ['risks', 'unmapped'] as const;

/** The tenant's controlled vocabulary. Seeded server-side on first read. */
export function useRiskCategories(withCounts = false) {
  return useQuery({
    queryKey: [...CATEGORIES_KEY, withCounts],
    queryFn: () => taxonomyService.listCategories(withCounts),
    staleTime: 60_000, // a curated vocabulary does not churn
  });
}

/**
 * The frameworks that can be mapped to. Empty is a MEANINGFUL answer here — it
 * means the tenant has imported nothing yet, and the caller renders an inline
 * empty state with a way to fix it rather than an empty dropdown.
 */
export function useImportedFrameworks() {
  return useQuery({
    queryKey: IMPORTED_FRAMEWORKS_KEY,
    queryFn: () => taxonomyService.listImportedFrameworks(),
  });
}

export function useFrameworkControls(frameworkId: string | null) {
  return useQuery({
    queryKey: ['compliance', 'frameworks', frameworkId, 'controls'],
    enabled: Boolean(frameworkId),
    queryFn: () => taxonomyService.listFrameworkControls(frameworkId as string),
  });
}

export function useRiskMappings(riskId: string, enabled = true) {
  return useQuery({
    queryKey: mappingsKey(riskId),
    enabled: enabled && Boolean(riskId),
    queryFn: () => taxonomyService.listMappings(riskId),
  });
}

export function useCreateMapping(riskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { framework_id?: string; control_id?: string | null; note?: string }) =>
      taxonomyService.createMapping(riskId, input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: mappingsKey(riskId) });
      // The register's "Référentiel" column and the unmapped backlog both change.
      void qc.invalidateQueries({ queryKey: ['risks'] });
      void qc.invalidateQueries({ queryKey: UNMAPPED_KEY });
    },
  });
}

export function useDeleteMapping(riskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (mappingId: string) => taxonomyService.deleteMapping(riskId, mappingId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: mappingsKey(riskId) });
      void qc.invalidateQueries({ queryKey: ['risks'] });
      void qc.invalidateQueries({ queryKey: UNMAPPED_KEY });
    },
  });
}

/** The catch-up screen: risks nobody has mapped to a control yet. */
export function useUnmappedRisks() {
  return useQuery({
    queryKey: UNMAPPED_KEY,
    queryFn: () => taxonomyService.listUnmapped(),
  });
}

export function useSaveCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<RiskCategory> & { id?: string }) =>
      input.id
        ? taxonomyService.updateCategory(input.id, input)
        : taxonomyService.createCategory(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: CATEGORIES_KEY });
      void qc.invalidateQueries({ queryKey: ['risks'] });
    },
  });
}

export function useDeleteCategory() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => taxonomyService.deleteCategory(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: CATEGORIES_KEY });
      void qc.invalidateQueries({ queryKey: ['risks'] });
    },
  });
}

export type { RiskCategory, RiskControlMapping };
