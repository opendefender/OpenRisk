// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// A 402 is the plan saying no (#783): asking again cannot change the answer.
// Queries and mutations share this so neither replays a plan refusal — a
// tenant at a cap would otherwise wait out the backoff before the cap notice.
export function isPlanRefusal(err: unknown): boolean {
  return (err as { response?: { status?: number } } | null)?.response?.status === 402;
}

/** Retry up to `max` times, never on a plan refusal. */
export function retryUnlessPlanRefusal(max: number) {
  return (failures: number, err: unknown): boolean => !isPlanRefusal(err) && failures < max;
}
