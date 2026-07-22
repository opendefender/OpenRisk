// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks over the real /asset-dependencies endpoints. Backs the
// dependency cartography (Asset Universe) and the per-asset dependency editor.

import { useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { assetService } from '../../services/assetService';
import type { AssetDependency, CreateAssetDependencyInput } from '../../types/asset';

const DEPENDENCIES_QUERY_KEY = ['asset-dependencies'];

export function useAssetDependencies() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: DEPENDENCIES_QUERY_KEY,
    queryFn: () => assetService.listDependencies(),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: DEPENDENCIES_QUERY_KEY });

  const createDependency = useMutation({
    mutationFn: (payload: CreateAssetDependencyInput) => assetService.createDependency(payload),
    onSettled: invalidate,
  });

  const deleteDependency = useMutation({
    mutationFn: (id: string) => assetService.deleteDependency(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: DEPENDENCIES_QUERY_KEY });
      const previous = queryClient.getQueryData<AssetDependency[]>(DEPENDENCIES_QUERY_KEY);
      if (previous) {
        queryClient.setQueryData(
          DEPENDENCIES_QUERY_KEY,
          previous.filter((d) => d.id !== id)
        );
      }
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(DEPENDENCIES_QUERY_KEY, context.previous);
    },
    onSettled: invalidate,
  });

  return useMemo(
    () => ({
      dependencies: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      createDependency,
      deleteDependency,
    }),
    [query, createDependency, deleteDependency]
  );
}
