// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// React Query hooks for the incident register. Mutations invalidate both the
// list and the stats so the KPI header and the table stay in sync.

import { useMemo } from 'react';
import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import {
  incidentService,
  type CreateIncidentInput,
  type IncidentListParams,
  type UpdateIncidentInput,
} from './incidentService';

const INCIDENTS_KEY = ['incidents'];
const STATS_KEY = ['incidents', 'stats'];

export function useIncidentStats() {
  return useQuery({
    queryKey: STATS_KEY,
    queryFn: () => incidentService.stats(),
  });
}

export function useIncidents(params: IncidentListParams = {}) {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: [...INCIDENTS_KEY, params],
    queryFn: () => incidentService.list(params),
    placeholderData: keepPreviousData,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: INCIDENTS_KEY });
    // STATS_KEY is a subkey of INCIDENTS_KEY, so the prefix match above already
    // covers it — kept explicit for readability.
    queryClient.invalidateQueries({ queryKey: STATS_KEY });
  };

  const createIncident = useMutation({
    mutationFn: (input: CreateIncidentInput) => incidentService.create(input),
    onSettled: invalidate,
  });

  const updateIncident = useMutation({
    mutationFn: ({ id, input }: { id: number; input: UpdateIncidentInput }) =>
      incidentService.update(id, input),
    onSettled: invalidate,
  });

  const deleteIncident = useMutation({
    mutationFn: (id: number) => incidentService.remove(id),
    onSettled: invalidate,
  });

  return useMemo(
    () => ({
      incidents: query.data?.incidents ?? [],
      total: query.data?.total ?? 0,
      isLoading: query.isLoading,
      error: query.error,
      createIncident,
      updateIncident,
      deleteIncident,
    }),
    [query, createIncident, updateIncident, deleteIncident]
  );
}
