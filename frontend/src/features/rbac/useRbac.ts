// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the RBAC business-role admin screen.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { rbacService, type AssignBusinessRoleInput } from './rbacService';

const CATALOG_KEY = ['rbac', 'catalog'] as const;
const MEMBERS_KEY = ['rbac', 'members'] as const;

export function useRbacCatalog() {
  return useQuery({
    queryKey: CATALOG_KEY,
    queryFn: () => rbacService.getCatalog(),
    staleTime: 5 * 60 * 1000, // presets are static
  });
}

export function useRbacMembers() {
  return useQuery({
    queryKey: MEMBERS_KEY,
    queryFn: () => rbacService.listMembers(),
  });
}

export function useAssignBusinessRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, input }: { userId: string; input: AssignBusinessRoleInput }) =>
      rbacService.assignBusinessRole(userId, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: MEMBERS_KEY });
    },
  });
}
