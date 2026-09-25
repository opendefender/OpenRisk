// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hook over a risk's score working. Keyed by the stored score too,
// so an edit that moves the score refetches instead of showing stale working.

import { useQuery } from '@tanstack/react-query';
import { scoreWorkingService } from './scoreWorkingService';

export function useScoreWorking(riskId: string | undefined, storedScore?: number) {
  return useQuery({
    queryKey: ['risk-score-working', riskId, storedScore],
    queryFn: () => scoreWorkingService.get(riskId as string),
    enabled: Boolean(riskId),
  });
}
