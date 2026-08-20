// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hook for the RBAC catalogue. Member listing and role assignment
// moved to features/organization/useOrganization with their endpoints (W0-04).

import { useQuery } from '@tanstack/react-query';
import { rbacService } from './rbacService';

const CATALOG_KEY = ['rbac', 'catalog'] as const;

export function useRbacCatalog() {
  return useQuery({
    queryKey: CATALOG_KEY,
    queryFn: () => rbacService.getCatalog(),
    staleTime: 5 * 60 * 1000, // presets are static
  });
}
