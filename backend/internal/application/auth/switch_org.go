// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// MembershipRepository is the narrow slice of user-repository behaviour the
// organization-switching use cases need. Kept small so it is trivial to mock and
// so the use cases cannot reach beyond membership questions.
type MembershipRepository interface {
	// ListActiveMemberships returns the user's ACTIVE memberships, Organization
	// preloaded.
	ListActiveMemberships(ctx context.Context, userID uuid.UUID) ([]*domain.OrganizationMember, error)
	// GetOrganizationMember returns the membership row for (user, org) or nil.
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
	// GetByID resolves the user (to read the default-org marker for the list).
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

// MembershipSummary is one row in a user's organization switcher.
type MembershipSummary struct {
	OrganizationID uuid.UUID              `json:"organization_id"`
	Name           string                 `json:"name"`
	Slug           string                 `json:"slug"`
	Role           domain.MemberRole      `json:"role"`
	BusinessRole   domain.BusinessRoleKey `json:"business_role,omitempty"`
	IsDefault      bool                   `json:"is_default"`
}

// ListOrganizationsUseCase lists the organizations a user may operate in.
type ListOrganizationsUseCase struct {
	members MembershipRepository
}

// NewListOrganizationsUseCase creates the use case.
func NewListOrganizationsUseCase(members MembershipRepository) *ListOrganizationsUseCase {
	return &ListOrganizationsUseCase{members: members}
}

// Execute returns the user's active memberships as switcher rows.
func (uc *ListOrganizationsUseCase) Execute(ctx context.Context, userID uuid.UUID) ([]MembershipSummary, error) {
	if userID == uuid.Nil {
		return nil, domain.NewValidationError("user is required")
	}

	memberships, err := uc.members.ListActiveMemberships(ctx, userID)
	if err != nil {
		return nil, err
	}

	var defaultOrg uuid.UUID
	if user, err := uc.members.GetByID(ctx, userID); err == nil && user != nil && user.DefaultOrgID != nil {
		defaultOrg = *user.DefaultOrgID
	}

	out := make([]MembershipSummary, 0, len(memberships))
	for _, m := range memberships {
		s := MembershipSummary{
			OrganizationID: m.OrganizationID,
			Role:           m.Role,
			BusinessRole:   m.BusinessRole,
			IsDefault:      m.OrganizationID == defaultOrg,
		}
		if m.Organization != nil {
			s.Name = m.Organization.Name
			s.Slug = m.Organization.Slug
		}
		out = append(out, s)
	}
	return out, nil
}

// SwitchOrganizationInput carries a switch request.
type SwitchOrganizationInput struct {
	UserID            uuid.UUID
	TargetOrgID       uuid.UUID
	DeviceFingerprint string
	IP                string
	UserAgent         string
}

// SwitchOrganizationOutput is the result of a successful switch.
//
// When the target organisation asks for a second factor, TokenPair is nil and
// SecondFactor carries the MFA_REQUIRED or MFA_ENROLLMENT token, scoped to the
// target organisation. The session is issued by /auth/mfa/challenge or by the
// enrolment, never by the switch.
type SwitchOrganizationOutput struct {
	TokenPair    *auth.TokenPair
	SecondFactor *SecondFactorOutcome
	Organization *domain.Organization
	Role         domain.MemberRole
	BusinessRole domain.BusinessRoleKey
}

// SecondFactorGate is the sign-in MFA gate, asked for the organisation a
// session moves into. LoginUseCase implements it.
type SecondFactorGate interface {
	SecondFactorForOrg(ctx context.Context, userID, orgID uuid.UUID, member *domain.OrganizationMember) (*SecondFactorOutcome, error)
}

// errNoSecondFactorGate refuses a switch on a deployment that wired no gate:
// without it the switch would mint a session no factor was asked for.
var errNoSecondFactorGate = errors.New("organization switch: no second-factor gate configured")

// SwitchOrganizationUseCase issues a session scoped to a DIFFERENT organization
// the user is an active member of. It is the server-authorized half of org
// switching: the client cannot change tenant by editing a token, cookie, or
// request body — only by presenting a valid session AND being an active member
// of the target org, both re-checked here on the server.
type SwitchOrganizationUseCase struct {
	members      MembershipRepository
	tokenManager *auth.TokenManager
	secondFactor SecondFactorGate
}

// NewSwitchOrganizationUseCase creates the use case. The gate is required: a
// nil one refuses every switch.
func NewSwitchOrganizationUseCase(members MembershipRepository, tokenManager *auth.TokenManager, secondFactor SecondFactorGate) *SwitchOrganizationUseCase {
	return &SwitchOrganizationUseCase{members: members, tokenManager: tokenManager, secondFactor: secondFactor}
}

// Execute validates active membership in the target org and mints a session for
// it. Non-members and deactivated members are rejected with a typed forbidden
// error — the same predicate the org resolver enforces when the token is minted,
// so there is no gap between "the switch was allowed" and "the token is valid".
func (uc *SwitchOrganizationUseCase) Execute(ctx context.Context, input SwitchOrganizationInput) (*SwitchOrganizationOutput, error) {
	if input.UserID == uuid.Nil {
		return nil, domain.NewValidationError("user is required")
	}
	if input.TargetOrgID == uuid.Nil {
		return nil, domain.NewValidationError("organization is required")
	}

	member, err := uc.members.GetOrganizationMember(ctx, input.UserID, input.TargetOrgID)
	if err != nil {
		return nil, err
	}
	// Reject non-membership and deactivated membership identically: a caller must
	// never learn whether an org they cannot enter exists.
	if member == nil || !member.IsActive {
		return nil, domain.NewForbiddenError("you are not an active member of this organization")
	}

	// The target organisation's own MFA posture, as at sign-in
	// (GHSA-45mr-j2w6-m4jv). An authenticator enrolled there is asked for, and
	// its role policy applies, whatever the session proved for the current one.
	if uc.secondFactor == nil {
		return nil, errNoSecondFactorGate
	}
	gate, err := uc.secondFactor.SecondFactorForOrg(ctx, input.UserID, input.TargetOrgID, member)
	if err != nil {
		return nil, err
	}
	if gate.Step != SecondFactorNone {
		return &SwitchOrganizationOutput{
			SecondFactor: gate,
			Organization: uc.organizationOfMember(ctx, input.UserID, input.TargetOrgID, member),
			Role:         member.Role,
			BusinessRole: member.BusinessRole,
		}, nil
	}

	pair, err := uc.tokenManager.IssueSessionForOrg(ctx, input.UserID, input.TargetOrgID, auth.DeviceContext{
		Fingerprint: input.DeviceFingerprint,
		IP:          input.IP,
		UserAgent:   input.UserAgent,
	})
	if err != nil {
		return nil, err
	}

	return &SwitchOrganizationOutput{
		TokenPair:    pair,
		Organization: uc.organizationOfMember(ctx, input.UserID, input.TargetOrgID, member),
		Role:         member.Role,
		BusinessRole: member.BusinessRole,
	}, nil
}

// organizationOfMember is the membership's preloaded organization, or the
// best-effort lookup below when it was not preloaded.
func (uc *SwitchOrganizationUseCase) organizationOfMember(ctx context.Context, userID, orgID uuid.UUID, member *domain.OrganizationMember) *domain.Organization {
	if member.Organization != nil {
		return member.Organization
	}
	return uc.organizationOf(ctx, userID, orgID)
}

// organizationOf names the organization the session was just minted for.
//
// GetOrganizationMember does not preload the organization, and it should not:
// it also resolves every session refresh, where the extra query would be paid
// on each call. The switch answered `organization: null` as a result, and the
// client had to fall back on its own list for the name. ListActiveMemberships
// preloads it, and a switch is rare enough to afford it. Best-effort: the
// session is already valid, so a failed lookup leaves the field empty rather
// than failing the switch.
func (uc *SwitchOrganizationUseCase) organizationOf(ctx context.Context, userID, orgID uuid.UUID) *domain.Organization {
	memberships, err := uc.members.ListActiveMemberships(ctx, userID)
	if err != nil {
		return nil
	}
	for _, m := range memberships {
		if m.OrganizationID == orgID && m.Organization != nil {
			return m.Organization
		}
	}
	return nil
}
