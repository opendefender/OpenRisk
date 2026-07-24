// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package rbac

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// UserProvisioner is the narrow port needed to create a member: look up existing
// users (email/username uniqueness) and persist the user + its org membership.
// Satisfied by the shared GormUserRepository.
type UserProvisioner interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	CreateOrganizationMember(ctx context.Context, member *domain.OrganizationMember) error
}

// PasswordHasher hashes a plaintext password (Argon2id in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// InviteMemberInput is the admin-supplied payload to add a member to the tenant.
type InviteMemberInput struct {
	Email        string
	FullName     string
	MemberRole   domain.MemberRole      // "" defaults to "user"
	BusinessRole domain.BusinessRoleKey // "" = no preset
}

// InviteMemberResult carries the created member plus the one-time temporary
// password the admin shares with the new member (never stored in clear).
type InviteMemberResult struct {
	Member       MemberView `json:"member"`
	TempPassword string     `json:"temp_password"`
}

// InviteMemberUseCase adds a NEW user to the CURRENT tenant with an org role and
// (optionally) a business-role preset — the missing "invite a member" path
// (OR-BUG-003). It reuses the same create sequence as registration
// (user + organization_member) so the invited member can actually sign in and
// its role resolves at login.
type InviteMemberUseCase struct {
	users  UserProvisioner
	hasher PasswordHasher
}

// NewInviteMemberUseCase builds the use case.
func NewInviteMemberUseCase(users UserProvisioner, hasher PasswordHasher) *InviteMemberUseCase {
	return &InviteMemberUseCase{users: users, hasher: hasher}
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Execute validates, provisions the user + membership in tenantID, and returns
// the member view + a one-time temporary password.
func (uc *InviteMemberUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in InviteMemberInput) (*InviteMemberResult, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if email == "" || !emailRe.MatchString(email) {
		return nil, domain.NewValidationError("a valid email is required")
	}
	if fullName == "" {
		return nil, domain.NewValidationError("full name is required")
	}
	role := in.MemberRole
	if role == "" {
		role = domain.RoleUser
	}
	if role != domain.RoleAdmin && role != domain.RoleUser {
		return nil, domain.NewValidationError("member_role must be 'admin' or 'user'")
	}
	if in.BusinessRole != "" && !domain.IsBusinessRole(in.BusinessRole) {
		return nil, domain.NewValidationError("unknown business role: " + string(in.BusinessRole))
	}

	// Email must be free.
	existing, err := uc.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, domain.NewConflictError("user", "email")
	}

	// Derive a unique username from the email local-part.
	username, err := uc.uniqueUsername(ctx, email)
	if err != nil {
		return nil, err
	}

	// Temporary password (shared once with the member).
	temp, err := randomPassword()
	if err != nil {
		return nil, err
	}
	hashed, err := uc.hasher.Hash(temp)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tid := tenantID
	user := &domain.User{
		Email:        email,
		Username:     username,
		Password:     hashed,
		FullName:     fullName,
		DefaultOrgID: &tid, // sign in lands them in the inviting tenant
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := uc.users.Create(ctx, user); err != nil {
		return nil, err
	}

	member := &domain.OrganizationMember{
		OrganizationID: tenantID,
		UserID:         user.ID,
		Role:           role,
		BusinessRole:   in.BusinessRole,
		IsActive:       true,
		JoinedAt:       now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := uc.users.CreateOrganizationMember(ctx, member); err != nil {
		return nil, err
	}
	member.User = user

	return &InviteMemberResult{
		Member: MemberView{
			UserID:       user.ID,
			Email:        user.Email,
			FullName:     user.FullName,
			OrgRole:      member.Role,
			BusinessRole: member.BusinessRole,
			IsActive:     true,
			Permissions:  member.EffectivePermissions(),
		},
		TempPassword: temp,
	}, nil
}

var usernameSanitize = regexp.MustCompile(`[^a-z0-9_.-]`)

func (uc *InviteMemberUseCase) uniqueUsername(ctx context.Context, email string) (string, error) {
	base := usernameSanitize.ReplaceAllString(strings.Split(email, "@")[0], "")
	if len(base) < 3 {
		base = base + "usr"
	}
	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := uc.users.GetByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
		suffix, err := randomHex(2)
		if err != nil {
			return "", err
		}
		candidate = base + suffix
	}
	return "", domain.NewConflictError("user", "username")
}

func randomPassword() (string, error) {
	// 9 random bytes → 18 hex chars, comfortably above any min-length policy.
	return randomHex(9)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
