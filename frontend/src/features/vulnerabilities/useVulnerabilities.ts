// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// React Query hooks for the vulnerability register.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  vulnerabilityService,
  type VulnQueryParams,
  type VulnStatus,
  type IngestInput,
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
  return useQuery({ queryKey: [...VULNS_KEY, 'connectors'], queryFn: vulnerabilityService.connectors });
}

export function useVulnMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: VULNS_KEY });

  const ingest = useMutation({
    mutationFn: (input: IngestInput) => vulnerabilityService.ingest(input),
    onSettled: invalidate,
  });
  const updateStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: VulnStatus }) =>
      vulnerabilityService.updateStatus(id, status),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => vulnerabilityService.remove(id),
    onSettled: invalidate,
  });

  return { ingest, updateStatus, remove };
}
