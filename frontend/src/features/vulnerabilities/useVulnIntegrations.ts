// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for vulnerability-integration + ticketing configuration.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  vulnIntegrationsService,
  type SaveIntegrationInput,
  type SaveTicketingInput,
} from './vulnIntegrationsService';

const INTEG_KEY = ['vuln-integrations'];
const TICKETING_KEY = ['vuln-ticketing'];

export function useVulnIntegrations() {
  return useQuery({ queryKey: INTEG_KEY, queryFn: vulnIntegrationsService.list });
}

export function useVulnTicketing() {
  return useQuery({ queryKey: TICKETING_KEY, queryFn: vulnIntegrationsService.getTicketing });
}

export function useVulnIntegrationMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: INTEG_KEY });

  const save = useMutation({
    mutationFn: (input: SaveIntegrationInput) => vulnIntegrationsService.save(input),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => vulnIntegrationsService.remove(id),
    onSettled: invalidate,
  });
  const pull = useMutation({
    mutationFn: (id: string) => vulnIntegrationsService.pull(id),
    onSettled: () => {
      invalidate();
      qc.invalidateQueries({ queryKey: ['vulnerabilities'] });
    },
  });
  return { save, remove, pull };
}

export function useVulnTicketingMutations() {
  const qc = useQueryClient();
  const invalidate = () => qc.invalidateQueries({ queryKey: TICKETING_KEY });

  const save = useMutation({
    mutationFn: (input: SaveTicketingInput) => vulnIntegrationsService.saveTicketing(input),
    onSettled: invalidate,
  });
  const remove = useMutation({
    mutationFn: () => vulnIntegrationsService.deleteTicketing(),
    onSettled: invalidate,
  });
  return { save, remove };
}
