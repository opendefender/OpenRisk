// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ScoreBand, ScoreModel } from '../../../services/scoreService';

/**
 * The band a 0–100 reading falls in, read off the model the server serves
 * (GET /score/model). The cut-offs live on the server; this only looks them up.
 */
export function bandOfReading(model: ScoreModel | undefined, value: number): ScoreBand | undefined {
  if (!model) return undefined;
  return model.bands.find(
    (b) => value >= b.min && (b.max_inclusive ? value <= b.max : value < b.max),
  )?.band;
}
