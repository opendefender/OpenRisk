// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Permanent demonstration-data banner.
//
// When the backend is started with DEMO_MODE=true it seeds a tenant from
// dev/fixtures/demo.json. Everything on screen then describes a fictional
// organisation. This banner states that, at the top of every page, and there is
// deliberately no dismiss control: the moment it can be closed, a screenshot of
// demo data becomes indistinguishable from a screenshot of a real posture, which
// is the exact confusion the whole zero-state effort exists to prevent.
//
// The flag is read from GET /health, so it reflects what the server actually
// seeded rather than a frontend build variable that could drift out of sync.

import { useQuery } from '@tanstack/react-query';
import { FlaskConical } from 'lucide-react';
import { api } from '../lib/api';
import { useUIStore } from '../store/uiStore';

interface HealthResponse {
  status: string;
  demo_mode?: boolean;
}

/** True when the server reports it is running on demonstration fixtures. */
export function useDemoMode(): boolean {
  const { data } = useQuery({
    queryKey: ['health', 'demo-mode'],
    queryFn: async () => (await api.get<HealthResponse>('/health')).data,
    staleTime: Infinity,
    retry: false,
  });
  return data?.demo_mode === true;
}

export function DemoBanner() {
  const isDemo = useDemoMode();
  const lang = useUIStore((s) => s.lang);
  if (!isDemo) return null;

  const message = lang === 'fr'
    ? 'Données de démonstration — ce tenant ne contient pas vos données réelles'
    : 'Demonstration data — this tenant does not contain your real data';

  return (
    <div
      data-testid="demo-banner"
      role="status"
      aria-live="polite"
      className="shrink-0 w-full flex items-center justify-center gap-2 px-4 py-2 text-[12.5px] font-semibold"
      style={{
        background: 'color-mix(in srgb, var(--high) 16%, transparent)',
        borderBottom: '1px solid color-mix(in srgb, var(--high) 34%, transparent)',
        color: 'var(--high)',
      }}
    >
      <FlaskConical size={15} strokeWidth={2} />
      <span className="text-center">{message}</span>
    </div>
  );
}
