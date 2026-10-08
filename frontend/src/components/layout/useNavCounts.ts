// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Live counters behind the sidebar badges (#900). Each one reads a real,
// tenant-scoped endpoint, and only when the member may open the screen the
// badge sits on: a badge on an entry the member cannot see would be a request
// the API refuses, retried every minute for nothing.
//
// A count that is loading, failed or refused stays at zero, which renders no
// badge. Zero is the honest answer when we do not know the number.

import { useQuery } from '@tanstack/react-query';
import type { NavCount } from '../../shared/navModel';
import { useOrganizationCounts } from '../../features/organization/useOrganization';
import { actionCenterService } from '../../features/action-center/actionCenterService';
import { vulnerabilityService } from '../../features/vulnerabilities/vulnerabilityService';
import { incidentService } from '../../features/incidents/incidentService';
import { useAuthStore } from '../../hooks/useAuthStore';

const REFRESH_MS = 60_000;

export function useNavCounts(can: (perm: string) => boolean): Record<NavCount, number> {
  const tenant = useAuthStore((s) => s.user?.tenant_id) ?? 'anonymous';
  const { data: orgCounts } = useOrganizationCounts();

  // limit=1: only the total is wanted, the rows are not.
  const actions = useQuery({
    queryKey: ['nav-counts', tenant, 'action-items'],
    queryFn: () => actionCenterService.list({ limit: 1, offset: 0 }),
    refetchInterval: REFRESH_MS,
    retry: 1,
  });
  const vulns = useQuery({
    queryKey: ['vulnerabilities', 'stats'],
    queryFn: vulnerabilityService.stats,
    enabled: can('vulnerabilities:read'),
    refetchInterval: REFRESH_MS,
    retry: 1,
  });
  const incidents = useQuery({
    queryKey: ['incidents', 'stats'],
    queryFn: () => incidentService.stats(),
    enabled: can('incidents:read'),
    refetchInterval: REFRESH_MS,
    retry: 1,
  });

  return {
    pending_invitations: orgCounts?.pending_invitations ?? 0,
    action_items: actions.data?.total ?? 0,
    kev_open: vulns.data?.kev_open ?? 0,
    open_incidents: incidents.data?.open_incidents ?? 0,
  };
}
