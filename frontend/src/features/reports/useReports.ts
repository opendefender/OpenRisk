// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { reportService, type ReportListFilter } from '../../services/reportService';
import type { CreateReportInput, Report, ReportLifecycle } from '../../types/report';

export const REPORTS_QUERY_KEY = ['reports'];

export function useReportCatalogue(locale: string) {
  return useQuery({
    queryKey: [...REPORTS_QUERY_KEY, 'catalogue', locale],
    queryFn: () => reportService.catalogue(locale),
    staleTime: 1000 * 60 * 10,
  });
}

/**
 * The report list.
 *
 * Polls while anything is still rendering, and stops once everything is
 * terminal. An interval that keeps running would hammer the API for the rest of
 * the session; one that never runs would leave a queued report looking stuck.
 */
export function useReports(filter: ReportListFilter = {}) {
  return useQuery({
    queryKey: [...REPORTS_QUERY_KEY, 'list', filter],
    queryFn: () => reportService.list(filter),
    refetchInterval: (query) => {
      const data = query.state.data as { items: Report[] } | undefined;
      const pending = data?.items?.some(
        (r) => r.run_state === 'queued' || r.run_state === 'running',
      );
      return pending ? 1500 : false;
    },
  });
}

export function useReport(id: string | undefined) {
  return useQuery({
    queryKey: [...REPORTS_QUERY_KEY, 'one', id],
    queryFn: () => reportService.get(id!),
    enabled: Boolean(id),
    refetchInterval: (query) => {
      const data = query.state.data as Report | undefined;
      if (!data) return false;
      return data.run_state === 'queued' || data.run_state === 'running' ? 1000 : false;
    },
  });
}

export function useReportVersions(id: string | undefined) {
  return useQuery({
    queryKey: [...REPORTS_QUERY_KEY, 'versions', id],
    queryFn: () => reportService.versions(id!),
    enabled: Boolean(id),
  });
}

function useReportMutation<TVars, TData>(fn: (vars: TVars) => Promise<TData>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: REPORTS_QUERY_KEY }),
  });
}

export function useCreateReport() {
  return useReportMutation((input: CreateReportInput) => reportService.create(input));
}

export function useTransitionReport() {
  return useReportMutation(
    ({ id, to, comment }: { id: string; to: ReportLifecycle; comment?: string }) =>
      reportService.transition(id, to, comment),
  );
}

export function useDeleteReport() {
  return useReportMutation((id: string) => reportService.remove(id));
}

export function useVerifyReport() {
  return useMutation({ mutationFn: (id: string) => reportService.verify(id) });
}
