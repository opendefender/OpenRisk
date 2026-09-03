// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React Query hooks for organization member management (W0-04).
//
// Every mutation invalidates the counts alongside its own list, because the
// sidebar badge and the members screen are two views of the same number and a
// user who sees them disagree stops trusting both.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  organizationService,
  type MemberQuery,
  type MemberRole,
  type MembershipStatus,
  type InvitationStatus,
} from './organizationService';

export const ORG_KEY = ['organization'] as const;
export const ORG_COUNTS_KEY = ['organization', 'counts'] as const;
export const MEMBERS_KEY = ['organization', 'members'] as const;
export const INVITATIONS_KEY = ['organization', 'invitations'] as const;
export const MEMBER_AUDIT_KEY = ['organization', 'member-audit'] as const;

/** Everything a membership change can move. Invalidated together so no view
 *  can lag behind another. */
function useInvalidateMembership() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: MEMBERS_KEY });
    void qc.invalidateQueries({ queryKey: INVITATIONS_KEY });
    void qc.invalidateQueries({ queryKey: MEMBER_AUDIT_KEY });
    void qc.invalidateQueries({ queryKey: ORG_KEY });
  };
}

export function useOrganization() {
  return useQuery({
    queryKey: ORG_KEY,
    queryFn: () => organizationService.getOrganization(),
  });
}

/** The sidebar badge. Open to any member, so it is safe to mount app-wide;
 *  a 30s stale window keeps it current without polling every render. */
export function useOrganizationCounts() {
  return useQuery({
    queryKey: ORG_COUNTS_KEY,
    queryFn: () => organizationService.getCounts(),
    staleTime: 30_000,
    // A member without the permission still gets counts, but a signed-out or
    // broken state must not retry forever behind the scenes.
    retry: 1,
  });
}

export function useMembers(query: MemberQuery) {
  return useQuery({
    queryKey: [...MEMBERS_KEY, query],
    queryFn: () => organizationService.listMembers(query),
    placeholderData: (prev) => prev, // keep the table on screen while re-filtering
  });
}

export function useInvitations(status?: InvitationStatus) {
  return useQuery({
    queryKey: [...INVITATIONS_KEY, status ?? 'all'],
    queryFn: () => organizationService.listInvitations(status),
  });
}

export function useMembershipAudit(limit = 50) {
  return useQuery({
    queryKey: [...MEMBER_AUDIT_KEY, limit],
    queryFn: () => organizationService.membershipAudit({ limit }),
  });
}

export function useInviteMember() {
  const invalidate = useInvalidateMembership();
  return useMutation({
    mutationFn: (input: {
      email: string;
      role: MemberRole;
      business_role?: string;
      locale?: string;
    }) => organizationService.invite(input),
    onSuccess: invalidate,
  });
}

export function useResendInvitation() {
  const invalidate = useInvalidateMembership();
  return useMutation({
    mutationFn: ({ id, locale }: { id: string; locale?: string }) =>
      organizationService.resendInvitation(id, locale),
    onSuccess: invalidate,
  });
}

export function useRevokeInvitation() {
  const invalidate = useInvalidateMembership();
  return useMutation({
    mutationFn: (id: string) => organizationService.revokeInvitation(id),
    onSuccess: invalidate,
  });
}

export function useSetMemberRole() {
  const invalidate = useInvalidateMembership();
  return useMutation({
    mutationFn: ({
      memberId,
      role,
      businessRole,
    }: {
      memberId: string;
      role: MemberRole;
      businessRole?: string;
    }) => organizationService.setRole(memberId, role, businessRole),
    onSuccess: invalidate,
  });
}

export function useSetMemberStatus() {
  const invalidate = useInvalidateMembership();
  return useMutation({
    mutationFn: ({
      memberId,
      status,
      reason,
    }: {
      memberId: string;
      status: MembershipStatus;
      reason?: string;
    }) => organizationService.setStatus(memberId, status, reason),
    onSuccess: invalidate,
  });
}
