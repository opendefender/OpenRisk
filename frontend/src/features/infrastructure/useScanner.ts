// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
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
  type ScanJob,
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
  const deleteConfig = useMutation({
    mutationFn: (id: string) => scannerService.deleteConfig(id),
    onSettled: invalidate,
  });
  const triggerScan = useMutation({
    mutationFn: (id: string) => scannerService.triggerScan(id),
    onSettled: invalidate,
  });

  return {
    configs: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error,
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
    onSettled: () => qc.invalidateQueries({ queryKey: AGENTS_KEY }),
  });
  return { agents: query.data ?? [], isLoading: query.isLoading, error: query.error, revokeAgent };
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
  return { jobs: query.data ?? [], isLoading: query.isLoading, error: query.error };
}

export function useScanPreview(jobId: string | undefined) {
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: ['scanner', 'preview', jobId],
    queryFn: () => scannerService.getPreview(jobId as string),
    enabled: !!jobId,
  });

  const importPreview = useMutation({
    mutationFn: (selections: ImportSelection[]) => scannerService.importPreview(jobId as string, selections),
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
