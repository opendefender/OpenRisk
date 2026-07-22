// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under

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
