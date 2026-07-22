// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { assetService } from '../../services/assetService';
import type { Asset, CreateAssetInput, UpdateAssetInput } from '../../types/asset';

const ASSETS_QUERY_KEY = ['assets'];
const historyQueryKey = (assetId: string) => ['assets', assetId, 'history'];

export function useAssets() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ASSETS_QUERY_KEY,
    queryFn: () => assetService.listAssets(),
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ASSETS_QUERY_KEY });

  const createAsset = useMutation({
    mutationFn: (payload: CreateAssetInput) => assetService.createAsset(payload),
    onSettled: invalidate,
  });

  const updateAsset = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateAssetInput }) =>
      assetService.updateAsset(id, payload),
    onMutate: async ({ id, payload }) => {
      await queryClient.cancelQueries({ queryKey: ASSETS_QUERY_KEY });
      const previous = queryClient.getQueryData<Asset[]>(ASSETS_QUERY_KEY);
      if (previous) {
        queryClient.setQueryData(
          ASSETS_QUERY_KEY,
          previous.map((a) => (a.id === id ? { ...a, ...payload } : a))
        );
      }
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(ASSETS_QUERY_KEY, context.previous);
    },
    onSettled: invalidate,
  });

  const deleteAsset = useMutation({
    mutationFn: (id: string) => assetService.deleteAsset(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ASSETS_QUERY_KEY });
      const previous = queryClient.getQueryData<Asset[]>(ASSETS_QUERY_KEY);
      if (previous) {
        queryClient.setQueryData(
          ASSETS_QUERY_KEY,
          previous.filter((a) => a.id !== id)
        );
      }
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(ASSETS_QUERY_KEY, context.previous);
    },
    onSettled: invalidate,
  });

  return useMemo(
    () => ({
      assets: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      createAsset,
      updateAsset,
      deleteAsset,
    }),
    [query, createAsset, updateAsset, deleteAsset]
  );
}

export function useAssetHistory(assetId: string | undefined) {
  return useQuery({
    queryKey: assetId ? historyQueryKey(assetId) : ['assets', 'history', 'disabled'],
    queryFn: () => assetService.getAssetHistory(assetId as string),
    enabled: !!assetId,
  });
}
