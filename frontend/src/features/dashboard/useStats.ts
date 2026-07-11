// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Dashboard headline stats from the real /stats endpoint (tenant-scoped).

import { useEffect, useState } from 'react';
import { api } from '../../lib/api';

export interface DashboardStats {
  total_risks: number;
  global_risk_score: number;
  high_risks: number;
  mitigated_risks: number;
  risks_by_severity?: Record<string, number>;
}

export function useDashboardStats() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let alive = true;
    api.get<DashboardStats>('/stats')
      .then((r) => { if (alive) setStats(r.data); })
      .catch(() => { if (alive) setStats(null); })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, []);
  return { stats, loading };
}
