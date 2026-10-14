// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for the CTI / Intel Threat engine. Sync + match mutations
// invalidate the feed and stats so the page stays live.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ctiService, type CTIListParams, type ThreatList } from './ctiService';

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

const THREATS_KEY = [...CTI_KEY, 'threats'];

export function useThreats() {
  return useQuery({ queryKey: THREATS_KEY, queryFn: () => ctiService.threats() });
}

/**
 * "Non pertinent" and its undo. Optimistic (ABSOLUTE RULE 10): the card dims
 * at once and comes back if the server refuses. A refused call is not retried:
 * a 4xx will not change on a second try.
 */
export function useThreatDismissal() {
  const qc = useQueryClient();
  return useMutation({
    retry: false,
    mutationFn: ({ cve, dismissed }: { cve: string; dismissed: boolean }) =>
      dismissed ? ctiService.dismiss(cve) : ctiService.restore(cve),
    onMutate: async ({ cve, dismissed }) => {
      await qc.cancelQueries({ queryKey: THREATS_KEY });
      const before = qc.getQueryData<ThreatList>(THREATS_KEY);
      if (before) {
        const at = dismissed ? new Date().toISOString() : null;
        qc.setQueryData<ThreatList>(THREATS_KEY, {
          ...before,
          threats: before.threats.map((th) =>
            th.cve_id === cve ? { ...th, dismissed_at: at } : th,
          ),
        });
      }
      return { before };
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.before) qc.setQueryData(THREATS_KEY, ctx.before);
    },
    onSettled: () => qc.invalidateQueries({ queryKey: THREATS_KEY }),
  });
}
