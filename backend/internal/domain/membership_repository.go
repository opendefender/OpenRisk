// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MemberQuery narrows a member listing. Zero-value fields are ignored.
type MemberQuery struct {
	// Search matches email or full name, case-insensitively.
	Search string
	// Status filters on the lifecycle state; empty means every state.
	Status MembershipStatus
	// Role filters on the org role; empty means every role.
	Role MemberRole
	// Limit / Offset page the result. The repository clamps Limit, so an
	// unbounded listing is not reachable by omitting it.
	Limit  int
	Offset int
	// SortBy is one of "joined_at" | "email" | "role" | "status"; SortDesc
	// reverses it. Anything else falls back to joined_at.
	SortBy   string
	SortDesc bool
}

// InvitationQuery narrows an invitation listing.
type InvitationQuery struct {
	// Status filters on the STORED status. Expiry is projected on read, so a
	// pending-but-expired invitation is returned by Status=pending and
	// presented as expired.
	Status InvitationStatus
	Email  string
	Limit  int
	Offset int
}

// OrganizationCounts is the tenant's membership headline, computed in one pass
// so the sidebar does not need a query per number.
type OrganizationCounts struct {
	TotalMembers       int64 `json:"total_members"`
	ActiveMembers      int64 `json:"active_members"`
	DeactivatedMembers int64 `json:"deactivated_members"`
	RevokedMembers     int64 `json:"revoked_members"`
	Admins             int64 `json:"admins"`
	PendingInvitations int64 `json:"pending_invitations"`
}

// MembershipRepository is the tenant-scoped store behind organization member
// management. Every method except FindInvitationByToken takes a tenantID and
// filters on it (RULE #2).
//
// FindInvitationByToken is the deliberate exception: acceptance happens before
// the accepting user has any relationship with the organization, so there is no
// tenant to scope by. The token IS the credential, it is looked up by its hash,
// and the organization it names comes from the stored row — never from the
// request.
type MembershipRepository interface {
	// Members
	ListMembers(ctx context.Context, tenantID uuid.UUID, q MemberQuery) ([]OrganizationMember, int64, error)
	GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*OrganizationMember, error)
	GetMemberByID(ctx context.Context, tenantID, memberID uuid.UUID) (*OrganizationMember, error)
	SaveMember(ctx context.Context, m *OrganizationMember) error
	// CountActiveAdmins counts memberships that both grant access and hold an
	// administrative role — the tenant's remaining administrative capacity.
	CountActiveAdmins(ctx context.Context, tenantID uuid.UUID) (int, error)
	// Counts takes the instant to judge invitation expiry against, rather than
	// reading the wall clock itself. The membership service owns a clock seam and
	// the whole invitation lifecycle is written against it; a repository that
	// calls time.Now() silently opts out of that seam and answers a different
	// question from the service that called it.
	Counts(ctx context.Context, tenantID uuid.UUID, asOf time.Time) (OrganizationCounts, error)

	// Invitations
	CreateInvitation(ctx context.Context, inv *Invitation) error
	SaveInvitation(ctx context.Context, inv *Invitation) error
	ListInvitations(ctx context.Context, tenantID uuid.UUID, q InvitationQuery) ([]Invitation, int64, error)
	GetInvitation(ctx context.Context, tenantID, id uuid.UUID) (*Invitation, error)
	// FindPendingInvitation returns the tenant's outstanding invitation for an
	// email, or nil. Used to refuse a duplicate before minting a second token.
	FindPendingInvitation(ctx context.Context, tenantID uuid.UUID, email string) (*Invitation, error)
	// FindInvitationByToken resolves a bearer token to its invitation. Not
	// tenant-scoped by design — see the interface comment.
	FindInvitationByToken(ctx context.Context, token string) (*Invitation, error)
	// AcceptInvitation consumes the invitation and creates the membership in one
	// transaction, so a crash between the two cannot leave a member with no
	// invitation record or an invitation that grants access twice.
	AcceptInvitation(ctx context.Context, inv *Invitation, member *OrganizationMember) error
}
