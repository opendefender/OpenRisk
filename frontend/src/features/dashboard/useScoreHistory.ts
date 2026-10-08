// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The tenant exposure score month by month (#901), from GET /score/history.
// A month with no snapshot is absent from `points`, never zero.

import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { ScoreBand } from '../../services/scoreService';

export interface ScoreHistoryPoint {
  month: string; // YYYY-MM
  day: string;
  value: number;
  band: ScoreBand;
}

export interface ScoreHistory {
  months: number;
  points: ScoreHistoryPoint[];
  current: number | null;
  delta_30d: number | null;
  since: string | null;
}

export function useScoreHistory(months = 12) {
  return useQuery({
    // Under the ['score'] family, so invalidateScores() refreshes it too.
    queryKey: ['score', 'history', months],
    queryFn: async ({ signal }) =>
      (await api.get<ScoreHistory>('/score/history', { params: { months }, signal })).data,
    staleTime: 60_000,
  });
}
