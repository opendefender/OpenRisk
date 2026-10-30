// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// React Query hooks for the scan engine. The jobs query self-polls while any
// scan is in flight (queued/claimed/running) so the UI reflects agent pushes
// and cloud completions without a browser SSE connection (EventSource can't send
// the Bearer header the /scanner/events route requires).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  scannerService,
  type CreateScanConfigInput,
  type ImportSelection,
  type ScanConfig,
  type ScanJob,
  type ScannerAgent,
} from './scannerService';

const CONFIGS_KEY = ['scanner', 'configs'];
const AGENTS_KEY = ['scanner', 'agents'];
const JOBS_KEY = ['scanner', 'jobs'];

const IN_FLIGHT: ScanJob['status'][] = ['queued', 'claimed', 'running'];

export function useScanConfigs() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: CONFIGS_KEY, queryFn: scannerService.listConfigs });
  const invalidate = () => qc.invalidateQueries({ queryKey: ['scanner'] });

  const createConfig = useMutation({
    mutationFn: (input: CreateScanConfigInput) => scannerService.createConfig(input),
    onSettled: invalidate,
  });
  // The row leaves the list at once and comes back if the server refuses.
  const deleteConfig = useMutation({
    mutationFn: (id: string) => scannerService.deleteConfig(id),
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: CONFIGS_KEY });
      const previous = qc.getQueryData<ScanConfig[]>(CONFIGS_KEY);
      qc.setQueryData<ScanConfig[]>(CONFIGS_KEY, (list) => list?.filter((c) => c.id !== id));
      return { previous };
    },
    onError: (_e, _id, ctx) => qc.setQueryData(CONFIGS_KEY, ctx?.previous),
    onSettled: invalidate,
  });
  // The new job is in the response; put it at the head of the list so the
  // scans block shows it before the next poll.
  const triggerScan = useMutation({
    mutationFn: (id: string) => scannerService.triggerScan(id),
    onSuccess: (job) =>
      qc.setQueryData<ScanJob[]>(JOBS_KEY, (list) => [
        job,
        ...(list ?? []).filter((j) => j.id !== job.id),
      ]),
    onSettled: invalidate,
  });

  return {
    configs: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
    createConfig,
    deleteConfig,
    triggerScan,
  };
}

export function useScannerAgents() {
  const qc = useQueryClient();
  // Agents drift online/offline on heartbeat, so refresh on a gentle cadence.
  const query = useQuery({
    queryKey: AGENTS_KEY,
    queryFn: scannerService.listAgents,
    refetchInterval: 15000,
  });
  const revokeAgent = useMutation({
    mutationFn: (id: string) => scannerService.revokeAgent(id),
    onMutate: async (id: string) => {
      await qc.cancelQueries({ queryKey: AGENTS_KEY });
      const previous = qc.getQueryData<ScannerAgent[]>(AGENTS_KEY);
      qc.setQueryData<ScannerAgent[]>(AGENTS_KEY, (list) =>
        list?.map((a) => (a.id === id ? { ...a, status: 'revoked' } : a)),
      );
      return { previous };
    },
    onError: (_e, _id, ctx) => qc.setQueryData(AGENTS_KEY, ctx?.previous),
    onSettled: () => qc.invalidateQueries({ queryKey: AGENTS_KEY }),
  });
  return {
    agents: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
    revokeAgent,
  };
}

export function useScanJobs() {
  const query = useQuery({
    queryKey: JOBS_KEY,
    queryFn: scannerService.listJobs,
    // Poll fast while a scan is in flight, otherwise stop.
    refetchInterval: (q) => {
      const jobs = q.state.data ?? [];
      return jobs.some((j) => IN_FLIGHT.includes(j.status)) ? 4000 : false;
    },
  });
  return {
    jobs: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}

export function useScanPreview(jobId: string | undefined) {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: ['scanner', 'preview', jobId],
    queryFn: () => scannerService.getPreview(jobId as string),
    enabled: !!jobId,
    // A missing/expired preview (unknown job id) is an expected state, not a
    // transient failure — don't retry-spam the console (OR-BUG-006); the page
    // renders a clean "Preview unavailable" state.
    retry: false,
  });

  const importPreview = useMutation({
    mutationFn: (selections: ImportSelection[]) =>
      scannerService.importPreview(jobId as string, selections),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['scanner'] });
      qc.invalidateQueries({ queryKey: ['assets'] });
    },
  });
  const ignorePreview = useMutation({
    mutationFn: () => scannerService.ignorePreview(jobId as string),
    onSettled: () => qc.invalidateQueries({ queryKey: ['scanner'] }),
  });

  return {
    preview: query.data,
    isLoading: query.isLoading,
    error: query.error,
    importPreview,
    ignorePreview,
  };
}
