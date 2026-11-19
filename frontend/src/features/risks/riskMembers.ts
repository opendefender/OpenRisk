// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import type { Assignee } from '../../services/ownershipService';

/** How a member is named in the register's pickers and cells. */
export function memberName(m: Pick<Assignee, 'full_name' | 'email'>): string {
  return m.full_name?.trim() || m.email;
}
