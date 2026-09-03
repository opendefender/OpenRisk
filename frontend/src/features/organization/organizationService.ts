// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for organization member management (W0-04).
//
// Shapes mirror internal/application/membership exactly — the backend's views
// are allowlists, and so are these. Nothing here has an `any`, and nothing
// carries an invitation token: the API never returns one except once, at
// creation or resend, and only when the email could not be delivered.

import { api } from '../../lib/api';

/* ---------------------------------------------------------------- vocabulary */

export type MemberRole = 'root' | 'admin' | 'user';
export type MembershipStatus = 'invited' | 'active' | 'deactivated' | 'revoked';
export type InvitationStatus = 'pending' | 'accepted' | 'expired' | 'revoked';
/** How the invitation email actually went — `sent` is the only one that means
 *  a message left the building. */
export type DeliveryStatus = 'sent' | 'unavailable' | 'failed';

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface MemberView {
  member_id: string;
  user_id: string;
  email: string;
  full_name: string;
  org_role: MemberRole;
  business_role: string;
  status: MembershipStatus;
  is_active: boolean;
  joined_at: string;
  deactivated_at?: string;
  revoked_at?: string;
  invited_by_id?: string;
  last_login?: string;
  permissions: string[];
  is_owner: boolean;
}

export interface InvitationView {
  id: string;
  email: string;
  role: MemberRole;
  business_role?: string;
  status: InvitationStatus;
  expires_at: string;
  invited_by_id: string;
  invited_by_email?: string;
  last_sent_at: string;
  send_count: number;
  accepted_at?: string;
  revoked_at?: string;
  created_at: string;
  /** The server's own answer to "may this be re-sent right now". Mirrored so
   *  the button is disabled for the same reason the API would refuse it. */
  can_resend: boolean;
}

export interface InviteResult {
  invitation: InvitationView;
  delivery: DeliveryStatus;
  delivery_detail?: string;
  /** Present only when the email did NOT go out, so an administrator can relay
   *  the link by hand. Never returned by any listing. */
  accept_url?: string;
}

export interface OrganizationCounts {
  total_members: number;
  active_members: number;
  deactivated_members: number;
  revoked_members: number;
  admins: number;
  pending_invitations: number;
}

export interface OrganizationView {
  id: string;
  name: string;
  slug: string;
  logo_url?: string;
  industry?: string;
  size?: string;
  plan: string;
  is_active: boolean;
  owner_id: string;
  owner_name?: string;
  timezone?: string;
  created_at: string;
  updated_at: string;
  counts: OrganizationCounts;
  /** The server's answer to "may this caller edit the profile", so the UI
   *  renders read-only for the same reason the API would refuse a write. */
  can_edit: boolean;
}

export interface AuditEntryView {
  id: string;
  at: string;
  actor_id?: string;
  actor_email?: string;
  action: string;
  entity_type: string;
  entity_id: string;
  summary: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  ip_address?: string;
}

export interface InvitationPreview {
  organization_name: string;
  email: string;
  role: MemberRole;
  expires_at: string;
  requires_signup: boolean;
}

export interface AcceptResult {
  organization_id: string;
  organization_name?: string;
  user_id: string;
  email: string;
  role: MemberRole;
  created_account: boolean;
}

export interface MemberQuery {
  q?: string;
  status?: MembershipStatus | '';
  role?: MemberRole | '';
  limit?: number;
  offset?: number;
  sort_by?: 'joined_at' | 'email' | 'role' | 'status';
  sort_dir?: 'asc' | 'desc';
}

/* -------------------------------------------------------------------- client */

function params(q: Record<string, string | number | undefined | null>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) {
    if (v !== undefined && v !== null && v !== '') sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

export const organizationService = {
  /** The tenant's own profile, with live membership counts. */
  async getOrganization(): Promise<OrganizationView> {
    const { data } = await api.get<OrganizationView>('/organization');
    return data;
  },

  /** The membership headline — what the sidebar badge reads. */
  async getCounts(): Promise<OrganizationCounts> {
    const { data } = await api.get<OrganizationCounts>('/organization/counts');
    return data;
  },

  async listMembers(q: MemberQuery = {}): Promise<Page<MemberView>> {
    const { data } = await api.get<Page<MemberView>>(`/organization/members${params({ ...q })}`);
    return data;
  },

  async getMember(memberId: string): Promise<MemberView> {
    const { data } = await api.get<MemberView>(`/organization/members/${memberId}`);
    return data;
  },

  /** Change a member's org role, and optionally their business-role preset.
   *  `businessRole` is omitted from the body when undefined, so a form that
   *  does not carry the field cannot silently strip the preset. */
  async setRole(memberId: string, role: MemberRole, businessRole?: string): Promise<MemberView> {
    const body: { role: MemberRole; business_role?: string } = { role };
    if (businessRole !== undefined) body.business_role = businessRole;
    const { data } = await api.put<MemberView>(`/organization/members/${memberId}/role`, body);
    return data;
  },

  async setStatus(
    memberId: string,
    status: MembershipStatus,
    reason?: string,
  ): Promise<MemberView> {
    const { data } = await api.put<MemberView>(`/organization/members/${memberId}/status`, {
      status,
      reason,
    });
    return data;
  },

  async listInvitations(status?: InvitationStatus): Promise<Page<InvitationView>> {
    const { data } = await api.get<Page<InvitationView>>(
      `/organization/invitations${params({ status })}`,
    );
    return data;
  },

  async invite(input: {
    email: string;
    role: MemberRole;
    business_role?: string;
    locale?: string;
  }): Promise<InviteResult> {
    const { data } = await api.post<InviteResult>('/organization/invitations', input);
    return data;
  },

  async resendInvitation(id: string, locale?: string): Promise<InviteResult> {
    const { data } = await api.post<InviteResult>(`/organization/invitations/${id}/resend`, {
      locale,
    });
    return data;
  },

  async revokeInvitation(id: string): Promise<InvitationView> {
    const { data } = await api.delete<InvitationView>(`/organization/invitations/${id}`);
    return data;
  },

  async membershipAudit(
    q: { entity_type?: string; limit?: number; offset?: number } = {},
  ): Promise<Page<AuditEntryView>> {
    const { data } = await api.get<Page<AuditEntryView>>(
      `/organization/members/audit${params({ ...q })}`,
    );
    return data;
  },

  /* --- acceptance: unauthenticated, and deliberately not tenant-scoped --- */

  async previewInvitation(token: string): Promise<InvitationPreview> {
    const { data } = await api.get<InvitationPreview>(`/invitations/preview${params({ token })}`);
    return data;
  },

  async acceptInvitation(input: {
    token: string;
    full_name?: string;
    password?: string;
  }): Promise<AcceptResult> {
    const { data } = await api.post<AcceptResult>('/invitations/accept', input);
    return data;
  },
};
