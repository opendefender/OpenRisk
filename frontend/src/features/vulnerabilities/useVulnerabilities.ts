// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the vulnerability register.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  vulnerabilityService,
  type VulnQueryParams,
  type VulnStatus,
  type IngestInput,
  type VulnListResponse,
} from './vulnerabilityService';

const VULNS_KEY = ['vulnerabilities'];

export function useVulnerabilities(params: VulnQueryParams) {
  return useQuery({
    queryKey: [...VULNS_KEY, 'list', params],
    queryFn: () => vulnerabilityService.list(params),
  });
}

export function useVulnStats() {
  return useQuery({ queryKey: [...VULNS_KEY, 'stats'], queryFn: vulnerabilityService.stats });
}

export function useVulnConnectors() {
  return useQuery({
    queryKey: [...VULNS_KEY, 'connectors'],
    queryFn: vulnerabilityService.connectors,
  });
}

export function useVulnMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: VULNS_KEY });

  const ingest = useMutation({
    mutationFn: (input: IngestInput) => vulnerabilityService.ingest(input),
    onSettled: invalidate,
  });
  // Optimistic (ABSOLUTE RULE 10): the row shows its new status at once and
  // goes back to the old one if the server refuses.
  const updateStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: VulnStatus }) =>
      vulnerabilityService.updateStatus(id, status),
    onMutate: async ({ id, status }) => {
      const key = [...VULNS_KEY, 'list'];
      await qc.cancelQueries({ queryKey: key });
      const before = qc.getQueriesData<VulnListResponse>({ queryKey: key });
      qc.setQueriesData<VulnListResponse>({ queryKey: key }, (page) =>
        page?.items
          ? { ...page, items: page.items.map((v) => (v.id === id ? { ...v, status } : v)) }
          : page,
      );
      return { before };
    },
    onError: (_e, _v, ctx) => {
      for (const [key, data] of ctx?.before ?? []) qc.setQueryData(key, data);
    },
    onSettled: invalidate,
  });
  const createTicket = useMutation({
    mutationFn: (id: string) => vulnerabilityService.createTicket(id),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => vulnerabilityService.remove(id),
    onSettled: invalidate,
  });

  return { ingest, updateStatus, createTicket, remove };
}
