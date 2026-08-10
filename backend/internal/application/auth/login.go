// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// LoginInput represents the input for user login
type LoginInput struct {
	Email             string
	Password          string
	DeviceFingerprint string // For security tracking
	// IP and UserAgent are recorded on the session so Settings → Sessions can
	// show something a person recognises, and so the new-device alert has
	// something to report.
	IP        string
	UserAgent string
}

// LoginOutput represents the output of successful login.
// When MFARequired is true, no full session is issued: TokenPair is nil and the
// caller must complete /auth/mfa/challenge with MFAToken to obtain the real pair.
type LoginOutput struct {
	User         *domain.User
	TokenPair    *auth.TokenPair
	Organization *domain.Organization
	MFARequired  bool
	MFAToken     string
	// MFAEnrollmentRequired is set when the account's role mandates MFA but no
	// authenticator is enrolled yet. TokenPair is nil and MFAToken carries an
	// MFA_ENROLLMENT token that only the enrolment endpoints accept.
	MFAEnrollmentRequired bool
	// BusinessRole is the member's GRC job-role preset (rssi/dsi/…), surfaced so
	// the frontend can pick a role-appropriate landing screen. Empty for
	// root/admin members.
	BusinessRole domain.BusinessRoleKey
}

// LoginUseCase handles user authentication
type LoginUseCase struct {
	userRepo       UserRepository
	tokenManager   *auth.TokenManager
	passwordHasher auth.PasswordHasher
	mfaRepo        repository.MFARepository // optional; when set, verified MFA is enforced
	// requireMFARoles are org roles that may not hold a session without MFA.
	requireMFARoles map[string]bool
}

// NewLoginUseCase creates a new login use case
func NewLoginUseCase(userRepo UserRepository, tokenManager *auth.TokenManager, passwordHasher auth.PasswordHasher) *LoginUseCase {
	return &LoginUseCase{
		userRepo:       userRepo,
		tokenManager:   tokenManager,
		passwordHasher: passwordHasher,
	}
}

// RequireMFAForRoles makes MFA mandatory for the named org roles.
//
// Wired with admin + root: those accounts can change anyone's permissions, read
// every tenant's risk register and mint API tokens, so a password alone is not a
// proportionate gate. Members keep MFA optional — mandating it for everyone in a
// product sold into organisations that may not all have authenticators is how
// you get shared accounts.
func (uc *LoginUseCase) RequireMFAForRoles(roles ...string) *LoginUseCase {
	uc.requireMFARoles = make(map[string]bool, len(roles))
	for _, r := range roles {
		uc.requireMFARoles[strings.ToLower(r)] = true
	}
	return uc
}

// WithMFA enables MFA enforcement: if the authenticating user has a verified MFA
// secret, login stops short of a full token pair and returns an MFA_REQUIRED
// challenge token instead.
func (uc *LoginUseCase) WithMFA(mfaRepo repository.MFARepository) *LoginUseCase {
	uc.mfaRepo = mfaRepo
	return uc
}

// Execute performs user login
func (uc *LoginUseCase) Execute(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	// Validate input
	if input.Email == "" {
		return nil, domain.NewValidationError("email is required")
	}
	if input.Password == "" {
		return nil, domain.NewValidationError("password is required")
	}

	// Find user by email
	user, err := uc.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}
	if user == nil {
		return nil, domain.NewValidationError("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, domain.NewValidationError("account is disabled")
	}

	// Verify password using Argon2id (OWASP recommended)
	if !uc.passwordHasher.Verify(user.Password, input.Password) {
		return nil, domain.NewValidationError("invalid credentials")
	}

	// Get user's default organization
	org, err := uc.userRepo.GetUserDefaultOrganization(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organization: %w", err)
	}
	if org == nil {
		return nil, domain.NewValidationError("user has no organization")
	}

	// Get user roles and permissions for the organization
	orgRoles := make(map[uuid.UUID]string)
	permissions := []string{}
	var businessRole domain.BusinessRoleKey

	member, err := uc.userRepo.GetOrganizationMember(ctx, user.ID, org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization membership: %w", err)
	}
	if member != nil {
		orgRoles[org.ID] = string(member.Role)
		// EffectivePermissions unifies root/admin wildcard, the business-role
		// preset, and any legacy profile rules (see domain.OrganizationMember).
		permissions = member.EffectivePermissions()
		businessRole = member.BusinessRole
	}

	// L4 — MFA enforcement. If the user has a verified MFA secret, do NOT issue a
	// full session yet: hand back a short-lived MFA_REQUIRED token that only
	// /auth/mfa/challenge accepts. The real pair is minted after code validation.
	if uc.mfaRepo != nil {
		mfaSecret, mErr := uc.mfaRepo.GetMFASecret(ctx, user.ID, org.ID)
		if mErr != nil {
			return nil, fmt.Errorf("failed to check MFA status: %w", mErr)
		}
		enrolled := mfaSecret != nil && mfaSecret.IsVerified

		if enrolled {
			mfaToken, tErr := uc.tokenManager.GenerateMFAChallengeToken(user.ID, org.ID)
			if tErr != nil {
				return nil, fmt.Errorf("failed to issue MFA challenge: %w", tErr)
			}
			return &LoginOutput{
				User:         user,
				Organization: org,
				MFARequired:  true,
				MFAToken:     mfaToken,
			}, nil
		}

		// Role mandates MFA but nothing is enrolled: stop here too. Handing out a
		// full session with a "please enrol soon" banner would make the
		// requirement advisory, and an advisory MFA requirement is not one.
		if member != nil && uc.mfaRequiredFor(string(member.Role)) {
			enrollToken, tErr := uc.tokenManager.GenerateMFAEnrollmentToken(user.ID, org.ID)
			if tErr != nil {
				return nil, fmt.Errorf("failed to issue MFA enrollment token: %w", tErr)
			}
			return &LoginOutput{
				User:                  user,
				Organization:          org,
				MFAEnrollmentRequired: true,
				MFAToken:              enrollToken,
				BusinessRole:          businessRole,
			}, nil
		}
	}

	// Generate token pair
	tokenPair, err := uc.tokenManager.GenerateTokenPair(
		ctx,
		user.ID,
		org.ID,
		orgRoles,
		permissions,
		[]string{}, // feature flags - can be extended
		auth.DeviceContext{
			Fingerprint: input.DeviceFingerprint,
			IP:          input.IP,
			UserAgent:   input.UserAgent,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update last login
	user.LastLogin = &time.Time{}
	*user.LastLogin = time.Now()
	if err := uc.userRepo.Update(ctx, user); err != nil {
		// Log error but don't fail the login
		fmt.Printf("Warning: failed to update last login: %v\n", err)
	}

	return &LoginOutput{
		User:         user,
		TokenPair:    tokenPair,
		Organization: org,
		BusinessRole: businessRole,
	}, nil
}

// mfaRequiredFor reports whether an org role may not hold a session without MFA.
func (uc *LoginUseCase) mfaRequiredFor(role string) bool {
	if len(uc.requireMFARoles) == 0 {
		return false
	}
	return uc.requireMFARoles[strings.ToLower(role)]
}

// UserRepository interface for user operations
type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	GetUserDefaultOrganization(ctx context.Context, userID uuid.UUID) (*domain.Organization, error)
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	CreateOrganizationMember(ctx context.Context, member *domain.OrganizationMember) error
}
