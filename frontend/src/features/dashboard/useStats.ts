// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Dashboard headline stats from the real /stats endpoint (tenant-scoped).

import { useEffect, useState } from 'react';
import { api } from '../../lib/api';

/** One cell of the 5x5 heatmap. Bands are computed server-side (see
 *  dashboard_handler.go) so every consumer bands identically. */
export interface MatrixCell {
  probability: number; // 1-5 band
  impact: number;      // 1-5 band
  count: number;
}

/**
 * Every field here is a tenant-wide SQL aggregate. None of it may be recomputed
 * on the client from a page of results: the register is paginated, so a
 * .filter().length reports the current page while reading like a total.
 */
export interface DashboardStats {
  total_risks: number;
  global_risk_score: number;
  high_risks: number;
  mitigated_risks: number;
  in_progress_risks: number;
  quantified_risks: number;
  risks_by_severity?: Record<string, number>;
  risk_matrix?: MatrixCell[];
}

export function useDashboardStats() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  // Surfaced so widgets can render the `error` empty state. Without it a failed
  // fetch is indistinguishable from an empty tenant, and the dashboard reports
  // zeros it never actually read.
  const [error, setError] = useState(false);
  useEffect(() => {
    let alive = true;
    api.get<DashboardStats>('/stats')
      .then((r) => { if (alive) { setStats(r.data); setError(false); } })
      .catch(() => { if (alive) { setStats(null); setError(true); } })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, []);
  return { stats, loading, error };
}
