// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for GET /risks/:id/score-working (#486). Mirrors
// domain.ScoreWorking: the frozen formula's terms, what the Score Engine makes
// of them now, the stored score, and — for a caller allowed to read the audit
// trail — the chained audit entry behind each term.

import { api } from '../../lib/api';

export type ScoreTermKey = 'probability' | 'impact' | 'asset_criticality';

/** One audit entry cited as the origin of a value. */
export interface ScoreWorkingSource {
  event_id: string;
  sequence: number;
  hash: string;
  prev_hash: string;
  /** Whether the entry still hashes to what was sealed. */
  hash_valid: boolean;
  action: string;
  source?: string;
  actor_id?: string;
  actor_email?: string;
  /** user | service_token | job */
  actor_type?: string;
  /** token id or job name */
  actor_label?: string;
  at: string;
  summary?: string;
  /** What the entry set the field to. */
  value: string | number | boolean | null;
}

export interface ScoreWorkingTerm {
  key: ScoreTermKey;
  value: number;
  min: number;
  max: number;
  source: ScoreWorkingSource | null;
}

export interface ScoreWorkingAsset {
  id: string;
  name: string;
  criticality: string;
  factor: number;
  source: ScoreWorkingSource | null;
}

export interface ScoreWorking {
  risk_id: string;
  formula: string;
  terms: ScoreWorkingTerm[];
  assets: ScoreWorkingAsset[];
  asset_criticality_defaulted: boolean;
  computed: number;
  criticality: string;
  explanation: string;
  stored: number;
  stored_criticality: string;
  consistent: boolean;
  score_source: ScoreWorkingSource | null;
  sources_visible: boolean;
}

export const scoreWorkingService = {
  async get(riskId: string): Promise<ScoreWorking> {
    const { data } = await api.get<ScoreWorking>(`/risks/${riskId}/score-working`);
    return data;
  },
};
