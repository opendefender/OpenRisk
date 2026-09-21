// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The organizations an account may operate in, and the switch between them
// (#296). Shapes mirror internal/application/auth/switch_org.go.
//
// An account belongs to several organizations as soon as an invitation adds a
// membership to it. The server lists the ACTIVE ones and, on a switch, re-checks
// that membership before minting a session for the target organization — the
// client never chooses its tenant by editing anything it holds.

import { api } from '../../lib/api';
import type { MemberRole } from './organizationService';

/** One row of the switcher: an organization the user is an active member of. */
export interface MembershipSummary {
  organization_id: string;
  name: string;
  slug: string;
  role: MemberRole;
  /** The GRC job-role preset in that organization; absent for root/admin. */
  business_role?: string;
  /** The organization the user signs into by default. */
  is_default: boolean;
}

/** What `POST /auth/switch-org` answers. The session cookies are re-issued on
 *  the same response; the body repeats the access token for the client claims. */
export interface SwitchResult {
  token_pair: { access_token: string; refresh_token: string; expires_in: number };
  /** Can be null: the server does not always load the organization row. */
  organization: { id: string; name: string } | null;
  role: MemberRole;
  business_role?: string;
}

export const orgSwitchService = {
  async listMine(): Promise<MembershipSummary[]> {
    const { data } = await api.get<{ organizations: MembershipSummary[] | null }>(
      '/auth/organizations',
    );
    return data.organizations ?? [];
  },

  async switchTo(organizationId: string): Promise<SwitchResult> {
    const { data } = await api.post<SwitchResult>('/auth/switch-org', {
      organization_id: organizationId,
    });
    return data;
  },
};
