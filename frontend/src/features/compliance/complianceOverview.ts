// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Combined compliance overview (frameworks + per-framework progress in one query)
// shaped for the dc.html Compliance screen. Complements the per-framework hooks in
// useCompliance.ts, which can't be composed in a loop.

import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import { OVERVIEW_QUERY_KEY } from './useCompliance';

interface RawFramework { id: string; name: string; version: string; description?: string }
interface RawProgress { Total: number; Applicable: number; PercentComplete: number; ByStatus?: Record<string, number> }

export interface FrameworkWithProgress extends RawFramework {
  total: number;
  passed: number;
  pct: number;
}

const PALETTE = ['#7c6cff', '#64d2ff', '#0a84ff', '#ff2d92', '#30d158', '#ff9f0a', '#ff453a'];
const NAMED: Record<string, string> = { ISO: '#7c6cff', SOC: '#64d2ff', NIST: '#0a84ff', DORA: '#ff2d92', BCEAO: '#30d158', ANSSI: '#ff9f0a', COBAC: '#30d158', ANTIC: '#ff9f0a' };

export function frameworkColorFor(name: string, index: number): string {
  const key = Object.keys(NAMED).find((k) => name.toUpperCase().includes(k));
  return key ? NAMED[key] : PALETTE[index % PALETTE.length];
}

export function useComplianceOverview() {
  return useQuery({
    queryKey: OVERVIEW_QUERY_KEY,
    queryFn: async (): Promise<FrameworkWithProgress[]> => {
      const { data: fws } = await api.get<RawFramework[]>('/compliance/frameworks');
      return Promise.all(
        (fws ?? []).map(async (f) => {
          try {
            const { data: p } = await api.get<RawProgress>(`/compliance/frameworks/${f.id}/progress`);
            const passed = (p.ByStatus?.implemented ?? 0) + (p.ByStatus?.partially_implemented ?? 0);
            return { ...f, total: p.Total ?? 0, passed, pct: Math.round(p.PercentComplete ?? 0) };
          } catch {
            return { ...f, total: 0, passed: 0, pct: 0 };
          }
        })
      );
    },
  });
}
