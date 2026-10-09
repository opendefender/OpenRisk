// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Incident } from '../../incidents/incidentService';

const RANK: Record<string, number> = { critical: 0, high: 1 };

/** The incident the banner is about, or undefined. */
export function bannerIncident(list: Incident[]): Incident | undefined {
  return list
    .filter((i) => (i.status === 'open' || i.status === 'in_progress') && i.severity in RANK)
    .sort(
      (a, b) =>
        RANK[a.severity] - RANK[b.severity] ||
        new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
    )[0];
}
