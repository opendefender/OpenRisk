// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the risk timeline (GET /risks/:id/timeline). Mirrors
// domain.RiskHistory: each entry is a snapshot at the moment of a change
// (change_type CREATE | UPDATE | MITIGATE, who, when, and the score/impact/
// probability/status at that time). The endpoint wraps the list in { timeline }.

import { api } from '../../lib/api';

export interface RiskHistoryEntry {
  id: string;
  risk_id: string;
  score: number;
  impact: number;
  probability: number;
  status: string;
  changed_by: string; // user id, or "System" for engine-driven changes
  change_type: string; // CREATE | UPDATE | MITIGATE
  created_at: string;
}

interface TimelineResponse {
  timeline: RiskHistoryEntry[];
  total: number;
  count: number;
}

export const riskTimelineService = {
  async getTimeline(riskId: string): Promise<RiskHistoryEntry[]> {
    const { data } = await api.get<TimelineResponse>(`/risks/${riskId}/timeline`);
    return data.timeline ?? [];
  },
};
