// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.

import { useQuery } from '@tanstack/react-query';
import { executiveService } from './executiveService';

/** The consolidated executive dashboard (spec §11). Refetches every 60 s so a
 *  board left open stays current without hammering the API. */
export function useExecutiveDashboard() {
  return useQuery({
    queryKey: ['analytics', 'executive'],
    queryFn: executiveService.get,
    refetchInterval: 60_000,
  });
}
