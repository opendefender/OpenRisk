// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ListMembers returns one page of the tenant's members.
func (s *Service) ListMembers(ctx context.Context, tenantID uuid.UUID, q domain.MemberQuery) (*Page[MemberView], error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	if q.Status != "" && !domain.IsValidMembershipStatus(q.Status) {
		return nil, domain.NewValidationError("unknown status filter: " + string(q.Status))
	}
	rows, total, err := s.repo.ListMembers(ctx, tenantID, q)
	if err != nil {
		return nil, err
	}
	items := make([]MemberView, 0, len(rows))
	for i := range rows {
		items = append(items, toMemberView(&rows[i]))
	}
	return &Page[MemberView]{Items: items, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}

// GetMember returns one member of the tenant.
//
// A member id belonging to ANOTHER tenant answers exactly like an id that never
// existed. That uniformity is the point: a 403 here would confirm that the id
// is real and that it belongs to somebody, which is the first half of an
// enumeration.
func (s *Service) GetMember(ctx context.Context, tenantID, memberID uuid.UUID) (*MemberView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	m, err := s.repo.GetMemberByID(ctx, tenantID, memberID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, domain.NewNotFoundError("member", memberID)
	}
	v := toMemberView(m)
	return &v, nil
}

// ChangeRoleInput is an administrator changing a member's access.
type ChangeRoleInput struct {
	ActorID      uuid.UUID
	MemberID     uuid.UUID
	Role         domain.MemberRole
	BusinessRole domain.BusinessRoleKey
	// BusinessRoleSet distinguishes "clear the business role" from "leave it
	// alone". Without it, saving a form that has no business-role field would
	// silently strip the preset a member depends on.
	BusinessRoleSet bool
}

// ChangeRole changes a member's organization role, and optionally their
// business-role preset.
//
// The guards live in domain.CheckRoleChange so the API and the UI can ask the
// same question. What the use case adds is the fact the policy cannot know on
// its own: how many administrators the tenant still has.
func (s *Service) ChangeRole(ctx context.Context, tenantID uuid.UUID, in ChangeRoleInput) (*MemberView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	role, business, err := validateRoles(in.Role, in.BusinessRole)
	if err != nil {
		return nil, err
	}

	m, err := s.repo.GetMemberByID(ctx, tenantID, in.MemberID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, domain.NewNotFoundError("member", in.MemberID)
	}

	admins, err := s.repo.CountActiveAdmins(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckRoleChange(domain.RoleChange{
		ActorID:          in.ActorID,
		TargetUserID:     m.UserID,
		TargetRole:       m.Role,
		TargetStatus:     m.EffectiveStatus(),
		NewRole:          role,
		ActiveAdminCount: admins,
	}); err != nil {
		return nil, err
	}

	before := domain.JSONMap{"role": string(m.Role), "business_role": string(m.BusinessRole)}
	m.Role = role
	if in.BusinessRoleSet || role == domain.RoleAdmin {
		m.BusinessRole = business
	}
	m.UpdatedAt = s.clock()
	if err := s.repo.SaveMember(ctx, m); err != nil {
		return nil, err
	}

	s.record(ctx, tenantID, in.ActorID, domain.AuditActionUpdate, "organization_member", m.ID.String(),
		fmt.Sprintf("Changed %s's role from %s to %s", memberLabel(m), before["role"], m.Role),
		before, domain.JSONMap{"role": string(m.Role), "business_role": string(m.BusinessRole)})

	// A member whose role just changed is carrying an access token minted with
	// the OLD permissions. Ending their refresh lineage forces the next renewal
	// to re-derive claims, so a demotion actually takes effect rather than
	// waiting for whatever session they happen to hold to lapse on its own.
	if s.revoker != nil {
		_ = s.revoker.RevokeAllUserTokens(ctx, m.UserID)
	}

	v := toMemberView(m)
	return &v, nil
}

// SetStatusInput is an administrator withdrawing or restoring access.
type SetStatusInput struct {
	ActorID  uuid.UUID
	MemberID uuid.UUID
	Status   domain.MembershipStatus
	Reason   string
}

// SetStatus deactivates, reactivates or revokes a membership.
//
// Withdrawing access is not a UI state: the membership stops granting access,
// and the member's refresh-token lineage is destroyed so no new session can be
// minted. The access token already in their hands stays valid until it expires
// (15 minutes) — that is the session model, and the documentation says so
// rather than the UI implying an instant cut-off it does not deliver.
func (s *Service) SetStatus(ctx context.Context, tenantID uuid.UUID, in SetStatusInput) (*MemberView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}

	m, err := s.repo.GetMemberByID(ctx, tenantID, in.MemberID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, domain.NewNotFoundError("member", in.MemberID)
	}

	admins, err := s.repo.CountActiveAdmins(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	current := m.EffectiveStatus()
	if err := domain.CheckStatusChange(domain.StatusChange{
		ActorID:          in.ActorID,
		TargetUserID:     m.UserID,
		TargetRole:       m.Role,
		CurrentStatus:    current,
		NewStatus:        in.Status,
		ActiveAdminCount: admins,
	}); err != nil {
		return nil, err
	}

	if !m.SetStatus(in.Status, s.clock()) {
		// CheckStatusChange already refused every illegal move, so reaching here
		// means the two disagree — refuse rather than write a state neither
		// vouches for.
		return nil, domain.NewValidationError("cannot move a " + string(current) + " membership to " + string(in.Status))
	}
	if err := s.repo.SaveMember(ctx, m); err != nil {
		return nil, err
	}

	action := domain.AuditActionUpdate
	if in.Status == domain.MembershipRevoked {
		action = domain.AuditActionRevoke
	}
	after := domain.JSONMap{"status": string(m.Status)}
	if in.Reason != "" {
		after["reason"] = in.Reason
	}
	s.record(ctx, tenantID, in.ActorID, action, "organization_member", m.ID.String(),
		fmt.Sprintf("Membership of %s moved from %s to %s", memberLabel(m), current, m.Status),
		domain.JSONMap{"status": string(current)}, after)

	if !in.Status.GrantsAccess() && s.revoker != nil {
		// Best-effort: a revoker outage must not leave the membership half-changed
		// in the database. The membership itself already refuses new sessions.
		_ = s.revoker.RevokeAllUserTokens(ctx, m.UserID)
	}

	v := toMemberView(m)
	return &v, nil
}

// memberLabel is the human handle used in audit summaries. It is an email or a
// user id — never anything more sensitive, since summaries are read widely.
func memberLabel(m *domain.OrganizationMember) string {
	if m.User != nil && m.User.Email != "" {
		return m.User.Email
	}
	return m.UserID.String()
}
