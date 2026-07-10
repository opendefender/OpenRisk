// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { boardService } from '../../services/boardService';
import type {
  BoardReport,
  GenerateBoardReportInput,
  UpdateBoardReportInput,
} from '../../types/board';

const BOARD_REPORTS_KEY = ['reports', 'board'];
const boardReportKey = (id: string) => ['reports', 'board', id];

// useBoardReports lists a tenant's board reports and exposes the generate/delete
// mutations. Generating can take several seconds when the Claude advisor is on, so
// the mutation's isPending drives the button's "generating…" state.
export function useBoardReports() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: BOARD_REPORTS_KEY,
    queryFn: () => boardService.list(),
    staleTime: 1000 * 30,
  });

  const generate = useMutation({
    mutationFn: (payload: GenerateBoardReportInput) => boardService.generate(payload),
    onSuccess: (report) => {
      queryClient.setQueryData<BoardReport[]>(BOARD_REPORTS_KEY, (prev) =>
        prev ? [report, ...prev] : [report]
      );
      queryClient.setQueryData(boardReportKey(report.id), report);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: BOARD_REPORTS_KEY }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => boardService.remove(id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: BOARD_REPORTS_KEY }),
  });

  return useMemo(
    () => ({
      reports: query.data ?? [],
      isLoading: query.isLoading,
      error: query.error,
      refetch: query.refetch,
      generate,
      remove,
    }),
    [query, generate, remove]
  );
}

// useBoardReport loads a single report and exposes the update/approve/download
// actions. update/approve optimistically refresh the list and detail caches.
export function useBoardReport(id: string | null) {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: id ? boardReportKey(id) : ['reports', 'board', 'none'],
    queryFn: () => boardService.get(id as string),
    enabled: !!id,
  });

  const sync = (report: BoardReport) => {
    queryClient.setQueryData(boardReportKey(report.id), report);
    queryClient.setQueryData<BoardReport[]>(BOARD_REPORTS_KEY, (prev) =>
      prev ? prev.map((r) => (r.id === report.id ? report : r)) : prev
    );
  };

  const update = useMutation({
    mutationFn: (payload: UpdateBoardReportInput) => boardService.update(id as string, payload),
    onSuccess: sync,
  });

  const approve = useMutation({
    mutationFn: () => boardService.approve(id as string),
    onSuccess: sync,
  });

  const download = useMutation({
    mutationFn: () => boardService.downloadPDF(id as string),
  });

  return useMemo(
    () => ({
      report: query.data ?? null,
      isLoading: query.isLoading,
      error: query.error,
      update,
      approve,
      download,
    }),
    [query.data, query.isLoading, query.error, update, approve, download]
  );
}
