// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { assetSchemaService } from './schemaService';
import type { AssetCategory, AttributeDef, UpdateAssetTypeSchemaInput } from './schemaTypes';

export const ASSET_SCHEMAS_QUERY_KEY = ['attack-surface', 'schemas'];

/**
 * The tenant's attribute schemas. Cached for the session: they change only when
 * an admin edits one, and every asset form needs them.
 */
export function useAssetSchemas() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ASSET_SCHEMAS_QUERY_KEY,
    queryFn: () => assetSchemaService.list(),
    staleTime: 5 * 60 * 1000,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ASSET_SCHEMAS_QUERY_KEY });

  const updateSchema = useMutation({
    mutationFn: ({
      category,
      payload,
    }: {
      category: AssetCategory;
      payload: UpdateAssetTypeSchemaInput;
    }) => assetSchemaService.update(category, payload),
    onSettled: invalidate,
  });

  const resetSchema = useMutation({
    mutationFn: (category: AssetCategory) => assetSchemaService.reset(category),
    onSettled: invalidate,
  });

  /** The attribute definitions governing a category, or [] while loading. */
  const defsFor = (category: AssetCategory | '' | undefined): AttributeDef[] => {
    if (!category) return [];
    return query.data?.find((s) => s.category === category)?.attributes ?? [];
  };

  return {
    schemas: query.data ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
    defsFor,
    updateSchema,
    resetSchema,
  };
}
