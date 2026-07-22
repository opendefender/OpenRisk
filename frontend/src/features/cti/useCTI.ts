// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the CTI / Intel Threat engine. Sync + match mutations
// invalidate the feed and stats so the page stays live.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ctiService, type CTIListParams } from './ctiService';

const CTI_KEY = ['cti'];

export function useCTIStats() {
  return useQuery({ queryKey: [...CTI_KEY, 'stats'], queryFn: () => ctiService.stats() });
}

export function useCTIVulnerabilities(params: CTIListParams = {}) {
  return useQuery({
    queryKey: [...CTI_KEY, 'list', params],
    queryFn: () => ctiService.list(params),
  });
}

export function useCTISync() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => ctiService.sync(),
    onSuccess: () => qc.invalidateQueries({ queryKey: CTI_KEY }),
  });
}

export function useCTIMatch() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => ctiService.match(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: CTI_KEY });
      qc.invalidateQueries({ queryKey: ['risks'] });
    },
  });
}
