// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #850 — does this account have a password in OpenRisk at all?

import { useQuery } from '@tanstack/react-query';
import { fetchHasLocalPassword } from './authService';

/**
 * Read fresh when a password surface mounts. `data` is null when the server did
 * not say; callers treat that like "has a password" and let the server's
 * no_local_password answer correct them.
 */
export function useHasLocalPassword() {
  return useQuery({
    queryKey: ['auth', 'has-local-password'],
    queryFn: fetchHasLocalPassword,
    staleTime: 0,
    retry: 1,
  });
}
