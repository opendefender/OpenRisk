// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The inventory's vulnerability columns (#906), from GET /assets/exposure.
// Only fetched for a member who may read vulnerabilities; for anyone else the
// columns say the figure is not theirs to see rather than showing zero.

import { useQuery } from '@tanstack/react-query';

import { api } from '../../lib/api';
import { useAuthStore } from '../../hooks/useAuthStore';
import type { components } from '../../types/openapi.generated';

export type AssetExposure = components['schemas']['AssetExposure'];

export function useAssetExposure() {
  const allowed = useAuthStore((s) => s.hasPermission('vulnerabilities:read'));
  const query = useQuery({
    queryKey: ['assets', 'exposure'],
    queryFn: async () =>
      (await api.get<{ items: AssetExposure[] }>('/assets/exposure')).data.items ?? [],
    enabled: allowed,
    staleTime: 60_000,
  });
  const byAsset = new Map((query.data ?? []).map((e) => [e.asset_id, e]));
  return { allowed, byAsset, isLoading: allowed && query.isLoading, isError: query.isError };
}
