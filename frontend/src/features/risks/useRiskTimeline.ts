// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hook over the risk timeline. `enabled` gates the fetch so it only
// runs when the drawer's "Timeline" tab is open (same pattern as useRiskSmartScore).

import { useQuery } from '@tanstack/react-query';
import { riskTimelineService } from './riskTimelineService';

export function useRiskTimeline(riskId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: ['risk-timeline', riskId],
    queryFn: () => riskTimelineService.getTimeline(riskId as string),
    enabled: Boolean(riskId) && enabled,
  });
}
