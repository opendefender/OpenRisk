// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package rbac holds the application use cases that let a tenant admin manage
// business roles: read the permission catalog + presets, list the tenant's
// members with their effective access, and assign a business role to a member.
// Everything is tenant-scoped (RULE #2) and expressed in the canonical
// permission vocabulary from internal/domain/business_roles.go.
package rbac

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MemberRepository is the narrow, tenant-scoped port the RBAC use cases need.
type MemberRepository interface {
	// GetMember returns the membership of userID in tenantID, or nil if none.
	GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error)
	// ListMembers returns every membership of the tenant, User preloaded.
	ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error)
	// UpdateMember persists role / business-role changes on a membership.
	UpdateMember(ctx context.Context, member *domain.OrganizationMember) error
}

// MemberView is a flattened member row for the admin RBAC screen.
type MemberView struct {
	UserID       uuid.UUID              `json:"user_id"`
	Email        string                 `json:"email"`
	FullName     string                 `json:"full_name"`
	OrgRole      domain.MemberRole      `json:"org_role"`
	BusinessRole domain.BusinessRoleKey `json:"business_role,omitempty"`
	IsActive     bool                   `json:"is_active"`
	// Permissions is the member's resolved effective permission set (["*"] for
	// admin/root), so the UI can show exactly what a member can do.
	Permissions []string `json:"permissions"`
}

// ListMembersUseCase lists a tenant's members with their resolved access.
type ListMembersUseCase struct {
	members MemberRepository
}

// NewListMembersUseCase builds the use case.
func NewListMembersUseCase(members MemberRepository) *ListMembersUseCase {
	return &ListMembersUseCase{members: members}
}

// Execute returns the tenant's member views.
func (uc *ListMembersUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]MemberView, error) {
	rows, err := uc.members.ListMembers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberView, 0, len(rows))
	for i := range rows {
		m := rows[i]
		v := MemberView{
			UserID:       m.UserID,
			OrgRole:      m.Role,
			BusinessRole: m.BusinessRole,
			IsActive:     m.IsActive,
			Permissions:  m.EffectivePermissions(),
		}
		if m.User != nil {
			v.Email = m.User.Email
			v.FullName = m.User.FullName
		}
		out = append(out, v)
	}
	return out, nil
}

// AssignBusinessRoleInput is the payload for assigning a business role.
type AssignBusinessRoleInput struct {
	TargetUserID uuid.UUID
	BusinessRole domain.BusinessRoleKey // "" clears the business role
	// MemberRole optionally changes the target's org role at the same time
	// ("admin" | "user"). Empty leaves it unchanged. This lets an admin, in one
	// call, both downgrade a full admin to a scoped "user" and give them a
	// business role so the preset actually governs their access.
	MemberRole domain.MemberRole
}

// AssignBusinessRoleUseCase assigns (or clears) a member's business role.
type AssignBusinessRoleUseCase struct {
	members MemberRepository
}

// NewAssignBusinessRoleUseCase builds the use case.
func NewAssignBusinessRoleUseCase(members MemberRepository) *AssignBusinessRoleUseCase {
	return &AssignBusinessRoleUseCase{members: members}
}

// Execute validates and applies the change, returning the updated member view.
func (uc *AssignBusinessRoleUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in AssignBusinessRoleInput) (*MemberView, error) {
	// Validate the business role (empty is allowed = clear).
	if in.BusinessRole != "" && !domain.IsBusinessRole(in.BusinessRole) {
		return nil, domain.NewValidationError("unknown business role: " + string(in.BusinessRole))
	}
	// Validate optional org-role change.
	if in.MemberRole != "" && in.MemberRole != domain.RoleAdmin && in.MemberRole != domain.RoleUser {
		return nil, domain.NewValidationError("member_role must be 'admin' or 'user'")
	}

	member, err := uc.members.GetMember(ctx, tenantID, in.TargetUserID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, domain.NewNotFoundError("member", in.TargetUserID)
	}
	// Never touch the organization owner (root) through this endpoint.
	if member.IsRoot() {
		return nil, domain.NewValidationError("cannot change the organization owner's role")
	}

	if in.MemberRole != "" {
		member.Role = in.MemberRole
	}
	member.BusinessRole = in.BusinessRole

	if err := uc.members.UpdateMember(ctx, member); err != nil {
		return nil, err
	}

	view := &MemberView{
		UserID:       member.UserID,
		OrgRole:      member.Role,
		BusinessRole: member.BusinessRole,
		IsActive:     member.IsActive,
		Permissions:  member.EffectivePermissions(),
	}
	if member.User != nil {
		view.Email = member.User.Email
		view.FullName = member.User.FullName
	}
	return view, nil
}
