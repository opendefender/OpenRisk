// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
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
  type PostMortemInput,
  type CreateIncidentActionInput,
  type IncidentActionStatus,
} from './incidentService';

const INCIDENTS_KEY = ['incidents'];
const STATS_KEY = ['incidents', 'stats'];

export function useIncidentStats() {
  return useQuery({
    queryKey: STATS_KEY,
    queryFn: () => incidentService.stats(),
  });
}

export function useIncident(id: number | undefined) {
  return useQuery({
    queryKey: [...INCIDENTS_KEY, 'detail', id],
    queryFn: () => incidentService.get(id as number),
    enabled: !!id,
  });
}

export function useIncidentTimeline(id: number | undefined) {
  return useQuery({
    queryKey: [...INCIDENTS_KEY, 'timeline', id],
    queryFn: () => incidentService.timeline(id as number),
    enabled: !!id,
  });
}

/**
 * The incident's response actions — the War Room's task board (W0-05 / D7).
 *
 * The endpoints are tenant-scoped server-side (the service checks ownsIncident
 * before every read and write), so this needs no guard of its own beyond the
 * incident id.
 */
export function useIncidentActions(id: number | undefined) {
  const qc = useQueryClient();
  const key = [...INCIDENTS_KEY, 'actions', id];

  const query = useQuery({
    queryKey: key,
    queryFn: () => incidentService.actions(id as number),
    enabled: !!id,
  });

  // Deliberately NOT optimistic. Everywhere else in the app an optimistic
  // update is right, because a failed mutation rolls back within a frame and
  // the user is still looking at the screen. A War Room task board is read
  // during an incident as the record of who is doing what — a row that appears
  // and silently vanishes on a failed write is the kind of thing people act on.
  // The list reflects what the server stored.
  const create = useMutation({
    mutationFn: (input: CreateIncidentActionInput) =>
      incidentService.createAction(id as number, input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: key });
    },
  });

  const setStatus = useMutation({
    mutationFn: ({ actionId, status }: { actionId: number; status: IncidentActionStatus }) =>
      incidentService.setActionStatus(id as number, actionId, status),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: key });
    },
  });

  return {
    actions: query.data ?? [],
    isLoading: query.isLoading,
    isError: query.isError,
    refetch: query.refetch,
    create,
    setStatus,
  };
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
      // Surfaced so the register can offer a retry instead of an empty state.
      isError: query.isError,
      refetch: query.refetch,
      createIncident,
      updateIncident,
      deleteIncident,
    }),
    [query, createIncident, updateIncident, deleteIncident],
  );
}

// ---------------------------------------------------------------------------
// Provenance + post-mortem
// ---------------------------------------------------------------------------

export function useIncidentOrigins() {
  return useQuery({
    queryKey: ['incidents', 'origins'],
    queryFn: incidentService.origins,
    staleTime: 5 * 60 * 1000,
  });
}

export function usePostMortem(id: number | null) {
  return useQuery({
    queryKey: ['incidents', 'post-mortem', id],
    queryFn: () => incidentService.getPostMortem(id as number),
    enabled: id != null,
  });
}

export function usePostMortemMutations(id: number | null) {
  const qc = useQueryClient();
  // Publishing can change whether the incident may be closed, so both the review
  // and the incident itself are refreshed.
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['incidents', 'post-mortem', id] });
    void qc.invalidateQueries({ queryKey: ['incidents'] });
  };

  const save = useMutation({
    mutationFn: (input: PostMortemInput) => incidentService.savePostMortem(id as number, input),
    onSettled: invalidate,
  });
  const publish = useMutation({
    mutationFn: () => incidentService.publishPostMortem(id as number),
    onSettled: invalidate,
  });
  return { save, publish };
}
