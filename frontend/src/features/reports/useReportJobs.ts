// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { reportJobService, type CreateReportJobInput, type ReportJob } from './reportJobService';

const JOBS_KEY = ['reports', 'jobs'];

export function useReportJobs(limit = 25) {
  const { data, isLoading, isError } = useQuery({
    queryKey: [...JOBS_KEY, limit],
    queryFn: () => reportJobService.list(limit),
  });
  return { jobs: data ?? [], isLoading, isError };
}

export function useReportJob(id: string | undefined) {
  const { data, isLoading, isError } = useQuery({
    queryKey: [...JOBS_KEY, id],
    queryFn: () => reportJobService.get(id as string),
    enabled: !!id,
    // Poll only while the job is still moving. A terminal job never changes, so
    // polling it forever would be a permanent background request per open tab.
    refetchInterval: (q) => {
      const s = (q.state.data as ReportJob | undefined)?.status;
      return s === 'queued' || s === 'running' ? 1500 : false;
    },
  });
  return { job: data, isLoading, isError };
}

/**
 * Requests a report and navigates to it.
 *
 * The navigation is the point: generating a report used to bounce the user
 * between Compliance and Reports without producing anything. Every caller goes
 * through here so no screen can reintroduce the round trip.
 */
export function useGenerateReport() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: (input: CreateReportJobInput) => reportJobService.create(input),
    onSuccess: (job) => {
      qc.invalidateQueries({ queryKey: JOBS_KEY });
      qc.setQueryData([...JOBS_KEY, job.id], job);
      navigate(`/reports/jobs/${job.id}`);
    },
  });
}
