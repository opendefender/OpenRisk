// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useMemo } from 'react';
import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { complianceService } from '../../services/complianceService';
import type {
  ComplianceControl,
  ComplianceFramework,
  ComplianceCatalogSummary,
  ControlEvidence,
  CreateControlInput,
  CreateFrameworkInput,
  UpdateControlInput,
  ImportCatalogInput,
  CreateAuditInput,
  UpdateAuditInput,
  CreateRemediationInput,
  UpdateRemediationInput,
  RemediationFilter,
  CreateControlMappingInput,
} from '../../types/compliance';

const FRAMEWORKS_QUERY_KEY = ['compliance', 'frameworks'];
const CATALOGS_QUERY_KEY = ['compliance', 'catalogs'];
// Shared with complianceOverview.ts's useComplianceOverview — the combined
// frameworks+progress query that feeds the Compliance grid and the framework
// detail header. Kept here so mutations can invalidate it in one place.
export const OVERVIEW_QUERY_KEY = ['compliance', 'overview'];
const controlsQueryKey = (frameworkId: string) => ['compliance', 'frameworks', frameworkId, 'controls'];
const evidencesQueryKey = (controlId: string) => ['compliance', 'controls', controlId, 'evidences'];
const progressQueryKey = (frameworkId: string) => ['compliance', 'frameworks', frameworkId, 'progress'];

export function useCatalogs() {
  return useQuery({
    queryKey: CATALOGS_QUERY_KEY,
    queryFn: () => complianceService.listCatalogs(),
    staleTime: 1000 * 60 * 5,
  });
}

export function useFrameworks() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: FRAMEWORKS_QUERY_KEY,
    queryFn: () => complianceService.listFrameworks(),
    staleTime: 1000 * 60 * 5,
  });

  const invalidateFrameworkLists = () => {
    queryClient.invalidateQueries({ queryKey: FRAMEWORKS_QUERY_KEY });
    queryClient.invalidateQueries({ queryKey: OVERVIEW_QUERY_KEY });
  };

  const createFramework = useMutation({
    mutationFn: (payload: CreateFrameworkInput) => complianceService.createFramework(payload),
    onSettled: invalidateFrameworkLists,
  });

  const deleteFramework = useMutation({
    mutationFn: (frameworkId: string) => complianceService.deleteFramework(frameworkId),
    onSettled: invalidateFrameworkLists,
  });

  return useMemo(
    () => ({
      frameworks: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      refetch: query.refetch,
      createFramework,
      deleteFramework,
    }),
    [query, createFramework, deleteFramework]
  );
}

// useComplianceReport downloads the official compliance report (PDF) for a
// framework. It's a mutation (a user-triggered side effect), exposing isPending
// so the button can show a generating state.
export function useComplianceReport() {
  return useMutation({
    mutationFn: ({ frameworkId, locale }: { frameworkId: string; locale: string }) =>
      complianceService.downloadReport(frameworkId, locale),
  });
}

// useImportCatalogAsFramework turns a regulatory catalog into its OWN selectable
// framework: it reuses an existing framework matching the catalog's name+version
// (so re-importing is idempotent and never duplicates), otherwise creates one,
// then imports the catalog's controls into it. This is what makes each imported
// catalog show up as its own entry in the rail instead of piling every catalog's
// controls into whichever framework happened to be selected.
export function useImportCatalogAsFramework() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (catalog: ComplianceCatalogSummary) => {
      const frameworks =
        queryClient.getQueryData<ComplianceFramework[]>(FRAMEWORKS_QUERY_KEY) ??
        (await complianceService.listFrameworks());
      const version = catalog.version ?? '';
      let framework = frameworks.find(
        (f) => f.name === catalog.name && (f.version ?? '') === version
      );
      if (!framework) {
        framework = await complianceService.createFramework({
          name: catalog.name,
          version: catalog.version,
          description: catalog.description,
        });
      }
      const result = await complianceService.importCatalog(framework.id, {
        catalog_key: catalog.key,
      });
      return { framework, result };
    },
    onSuccess: ({ framework }) => {
      queryClient.invalidateQueries({ queryKey: FRAMEWORKS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: controlsQueryKey(framework.id) });
      queryClient.invalidateQueries({ queryKey: progressQueryKey(framework.id) });
      queryClient.invalidateQueries({ queryKey: OVERVIEW_QUERY_KEY });
    },
  });
}

// useGapAnalysis fetches the tenant's open compliance gaps (all frameworks, or a
// single one). Shares the ['compliance','overview'] invalidation family so a
// status change on a control refreshes the gap list too.
export function useGapAnalysis(frameworkId?: string) {
  return useQuery({
    queryKey: ['compliance', 'gap-analysis', frameworkId ?? 'all'],
    queryFn: () => complianceService.getGapAnalysis(frameworkId),
    staleTime: 1000 * 30,
  });
}

// --- Audits ------------------------------------------------------------------
const AUDITS_QUERY_KEY = ['compliance', 'audits'];

export function useAudits() {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: AUDITS_QUERY_KEY, queryFn: () => complianceService.listAudits() });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: AUDITS_QUERY_KEY });

  const createAudit = useMutation({
    mutationFn: (payload: CreateAuditInput) => complianceService.createAudit(payload),
    onSettled: invalidate,
  });
  const updateAudit = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateAuditInput }) => complianceService.updateAudit(id, payload),
    onSettled: invalidate,
  });
  const deleteAudit = useMutation({
    mutationFn: (id: string) => complianceService.deleteAudit(id),
    onSettled: invalidate,
  });
  const generateRemediations = useMutation({
    mutationFn: (auditId: string) => complianceService.generateRemediations(auditId),
    // A generated batch shows up under the remediation lists.
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['compliance', 'remediations'] }),
  });

  return useMemo(
    () => ({ audits: query.data ?? [], isLoading: query.isLoading, error: query.error, refetch: query.refetch, createAudit, updateAudit, deleteAudit, generateRemediations }),
    [query, createAudit, updateAudit, deleteAudit, generateRemediations]
  );
}

// --- Remediation plans -------------------------------------------------------
const remediationsQueryKey = (filter?: RemediationFilter) => ['compliance', 'remediations', filter ?? {}];

export function useRemediations(filter?: RemediationFilter) {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: remediationsQueryKey(filter), queryFn: () => complianceService.listRemediations(filter) });
  // Invalidate every remediation list (any filter) after a mutation.
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['compliance', 'remediations'] });

  const createRemediation = useMutation({
    mutationFn: (payload: CreateRemediationInput) => complianceService.createRemediation(payload),
    onSettled: invalidate,
  });
  const updateRemediation = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateRemediationInput }) => complianceService.updateRemediation(id, payload),
    onSettled: invalidate,
  });
  const deleteRemediation = useMutation({
    mutationFn: (id: string) => complianceService.deleteRemediation(id),
    onSettled: invalidate,
  });

  return useMemo(
    () => ({ remediations: query.data ?? [], isLoading: query.isLoading, error: query.error, refetch: query.refetch, createRemediation, updateRemediation, deleteRemediation }),
    [query, createRemediation, updateRemediation, deleteRemediation]
  );
}

export function useComplianceProgress(frameworkId: string | undefined) {
  return useQuery({
    queryKey: frameworkId ? progressQueryKey(frameworkId) : ['compliance', 'progress', 'disabled'],
    queryFn: () => complianceService.getProgress(frameworkId as string),
    enabled: !!frameworkId,
  });
}

export function useControls(frameworkId: string | undefined) {
  const queryClient = useQueryClient();
  const queryKey = frameworkId ? controlsQueryKey(frameworkId) : ['compliance', 'controls', 'disabled'];

  const query = useQuery({
    queryKey,
    queryFn: () => complianceService.listControls(frameworkId as string),
    placeholderData: keepPreviousData,
    enabled: !!frameworkId,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey });
    if (frameworkId) queryClient.invalidateQueries({ queryKey: progressQueryKey(frameworkId) });
    // The Compliance grid + framework-detail header gauge read from the combined
    // ['compliance','overview'] query, not from these per-framework keys — invalidate
    // it too so progress reflects a status change in real time (not on next reload).
    queryClient.invalidateQueries({ queryKey: OVERVIEW_QUERY_KEY });
  };

  const createControl = useMutation({
    mutationFn: (payload: CreateControlInput) => complianceService.createControl(frameworkId as string, payload),
    onSettled: invalidate,
  });

  const importCatalog = useMutation({
    mutationFn: (payload: ImportCatalogInput) => complianceService.importCatalog(frameworkId as string, payload),
    onSettled: invalidate,
  });

  const updateControl = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateControlInput }) =>
      complianceService.updateControl(id, payload),
    onMutate: async ({ id, payload }) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<ComplianceControl[]>(queryKey);
      if (previous) {
        queryClient.setQueryData(
          queryKey,
          previous.map((c) => (c.id === id ? { ...c, ...payload } : c))
        );
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: invalidate,
  });

  const deleteControl = useMutation({
    mutationFn: (id: string) => complianceService.deleteControl(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<ComplianceControl[]>(queryKey);
      if (previous) {
        queryClient.setQueryData(
          queryKey,
          previous.filter((c) => c.id !== id)
        );
      }
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: invalidate,
  });

  return useMemo(
    () => ({
      controls: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      refetch: query.refetch,
      createControl,
      updateControl,
      deleteControl,
      importCatalog,
    }),
    [query, createControl, updateControl, deleteControl, importCatalog]
  );
}

export function useEvidences(controlId: string | undefined) {
  const queryClient = useQueryClient();
  const queryKey = controlId ? evidencesQueryKey(controlId) : ['compliance', 'evidences', 'disabled'];

  const query = useQuery({
    queryKey,
    queryFn: () => complianceService.listEvidences(controlId as string),
    enabled: !!controlId,
  });

  // Adding/removing evidence changes a control's evidence_count, which the
  // controls table badges and which gates the "implemented" transition. Refresh
  // the evidence list AND the per-framework controls + combined overview so the
  // badge and the strict-mode lock update the moment a proof is attached.
  const invalidateAfterEvidenceChange = () => {
    queryClient.invalidateQueries({ queryKey });
    queryClient.invalidateQueries({ queryKey: FRAMEWORKS_QUERY_KEY });
    queryClient.invalidateQueries({ queryKey: OVERVIEW_QUERY_KEY });
  };

  const createEvidence = useMutation({
    mutationFn: ({ file, description }: { file: File; description?: string }) =>
      complianceService.createEvidence(controlId as string, file, description),
    onSettled: invalidateAfterEvidenceChange,
  });

  const deleteEvidence = useMutation({
    mutationFn: (id: string) => complianceService.deleteEvidence(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<ControlEvidence[]>(queryKey);
      if (previous) {
        queryClient.setQueryData(
          queryKey,
          previous.filter((e) => e.id !== id)
        );
      }
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: invalidateAfterEvidenceChange,
  });

  const downloadEvidence = useMutation({
    mutationFn: ({ id, filename }: { id: string; filename: string }) =>
      complianceService.downloadEvidence(id, filename),
  });

  return useMemo(
    () => ({
      evidences: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      createEvidence,
      deleteEvidence,
      downloadEvidence,
    }),
    [query, createEvidence, deleteEvidence, downloadEvidence]
  );
}

// useControlMappings — the tenant's cross-framework crosswalks for one control
// (both directions). Powers the "Correspondances" section of the control drawer.
export function useControlMappings(controlId: string | undefined) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: ['compliance', 'control-mappings', controlId ?? 'none'],
    queryFn: () => complianceService.listControlMappings(controlId),
    enabled: !!controlId,
  });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['compliance', 'control-mappings'] });

  const create = useMutation({
    mutationFn: (payload: CreateControlMappingInput) => complianceService.createControlMapping(payload),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => complianceService.deleteControlMapping(id),
    onSettled: invalidate,
  });

  return useMemo(
    () => ({ mappings: query.data ?? [], isLoading: query.isLoading, error: query.error, create, remove }),
    [query, create, remove]
  );
}
