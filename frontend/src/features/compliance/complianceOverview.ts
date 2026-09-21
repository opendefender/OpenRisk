// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Combined compliance overview (frameworks + per-framework progress in one query)
// shaped for the dc.html Compliance screen. Complements the per-framework hooks in
// useCompliance.ts, which can't be composed in a loop.

import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import { frameworkColor, SERIES } from '../../shared/riskColors';
import { OVERVIEW_QUERY_KEY } from './useCompliance';

interface RawFramework {
  id: string;
  name: string;
  version: string;
  description?: string;
}
interface RawProgress {
  Total: number;
  Applicable: number;
  PercentComplete: number;
  ByStatus?: Record<string, number>;
}

export interface FrameworkWithProgress extends RawFramework {
  total: number;
  passed: number;
  pct: number;
}

// Framework names arrive free-form ("ISO/IEC 27001:2022", "SOC 2 Type II"), so
// match on a stem and resolve through the one shared map.
const STEMS: [stem: string, key: string][] = [
  ['ISO', 'ISO27001'],
  ['SOC', 'SOC2'],
  ['NIST', 'NIST'],
  ['DORA', 'DORA'],
  ['BCEAO', 'BCEAO'],
  ['ANSSI', 'ANSSI'],
  ['COBAC', 'COBAC'],
  ['ANTIC', 'ANTIC'],
];

export function frameworkColorFor(name: string, index: number): string {
  const hit = STEMS.find(([stem]) => name.toUpperCase().includes(stem));
  return hit ? frameworkColor[hit[1]] : SERIES[index % SERIES.length];
}

export function useComplianceOverview() {
  return useQuery({
    queryKey: OVERVIEW_QUERY_KEY,
    queryFn: async (): Promise<FrameworkWithProgress[]> => {
      const { data: fws } = await api.get<RawFramework[]>('/compliance/frameworks');
      return Promise.all(
        (fws ?? []).map(async (f) => {
          try {
            const { data: p } = await api.get<RawProgress>(
              `/compliance/frameworks/${f.id}/progress`,
            );
            const passed =
              (p.ByStatus?.implemented ?? 0) + (p.ByStatus?.partially_implemented ?? 0);
            return { ...f, total: p.Total ?? 0, passed, pct: Math.round(p.PercentComplete ?? 0) };
          } catch {
            return { ...f, total: 0, passed: 0, pct: 0 };
          }
        }),
      );
    },
  });
}
