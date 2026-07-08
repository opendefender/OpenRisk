// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useMemo } from 'react';
import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { complianceService } from '../../services/complianceService';
import type {
  ComplianceControl,
  ControlEvidence,
  CreateControlInput,
  CreateFrameworkInput,
  UpdateControlInput,
} from '../../types/compliance';

const FRAMEWORKS_QUERY_KEY = ['compliance', 'frameworks'];
const controlsQueryKey = (frameworkId: string) => ['compliance', 'frameworks', frameworkId, 'controls'];
const evidencesQueryKey = (controlId: string) => ['compliance', 'controls', controlId, 'evidences'];
const progressQueryKey = (frameworkId: string) => ['compliance', 'frameworks', frameworkId, 'progress'];

export function useFrameworks() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: FRAMEWORKS_QUERY_KEY,
    queryFn: () => complianceService.listFrameworks(),
    staleTime: 1000 * 60 * 5,
  });

  const createFramework = useMutation({
    mutationFn: (payload: CreateFrameworkInput) => complianceService.createFramework(payload),
    onSettled: () => queryClient.invalidateQueries({ queryKey: FRAMEWORKS_QUERY_KEY }),
  });

  return useMemo(
    () => ({
      frameworks: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      refetch: query.refetch,
      createFramework,
    }),
    [query, createFramework]
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
  };

  const createControl = useMutation({
    mutationFn: (payload: CreateControlInput) => complianceService.createControl(frameworkId as string, payload),
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
    }),
    [query, createControl, updateControl, deleteControl]
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

  const createEvidence = useMutation({
    mutationFn: ({ file, description }: { file: File; description?: string }) =>
      complianceService.createEvidence(controlId as string, file, description),
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
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
    onSettled: () => queryClient.invalidateQueries({ queryKey }),
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
