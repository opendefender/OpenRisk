// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"
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
	// MFAEnrollmentRequired is set when the account's role mandates MFA, no
	// authenticator is enrolled, AND the grace window has run out. TokenPair is
	// nil and MFAToken carries an MFA_ENROLLMENT token that only the enrolment
	// endpoints accept.
	//
	// OR26-03: this used to fire the moment a privileged account had no
	// authenticator. It now fires only once deferral is genuinely over — see
	// domain.DecideMFA.
	MFAEnrollmentRequired bool
	// MFAStatus is the resolved enrolment state for this member, shipped with
	// every successful login so the SPA can render the banner without a second
	// round trip. It is the same value /auth/me returns and the same value the
	// request-time guard enforces — one decision, three readers.
	MFAStatus domain.MFADecision
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
	// privileged names the roles for which MFA is not negotiable. Empty = the
	// deployment has switched mandatory enrolment off entirely.
	privileged domain.MFAPrivilegeSet
	// mfaPolicies reads the tenant's grace window. Optional and nil-safe: a
	// deployment without it behaves as though every tenant saved the defaults.
	mfaPolicies MFAPolicyReader
	// now is injectable so the grace arithmetic is testable without sleeping.
	now func() time.Time
}

// MFAPolicyReader resolves one tenant's MFA grace policy.
//
// A narrow port rather than the repository interface: the login use case has no
// business writing policies, and a reader is what a mock has to satisfy.
type MFAPolicyReader interface {
	GetMFAPolicy(ctx context.Context, tenantID uuid.UUID) (*domain.MFAPolicy, error)
}

// MFAGraceAnchorWriter stamps the instant a member became subject to the MFA
// requirement, the first time anything notices they are.
//
// Optional. Without it the anchor still resolves from the membership start
// (OrganizationMember.MFAGraceAnchor), so the countdown never becomes unknown;
// what the writer adds is a stable, explicit anchor for accounts created before
// migration 0060 ran.
type MFAGraceAnchorWriter interface {
	SetMFAGraceStartedAt(ctx context.Context, memberID uuid.UUID, at time.Time) error
}

// NewLoginUseCase creates a new login use case
func NewLoginUseCase(userRepo UserRepository, tokenManager *auth.TokenManager, passwordHasher auth.PasswordHasher) *LoginUseCase {
	return &LoginUseCase{
		userRepo:       userRepo,
		tokenManager:   tokenManager,
		passwordHasher: passwordHasher,
	}
}

// RequireMFAForRoles makes MFA mandatory for the named org roles and business
// roles.
//
// Wired with admin + root (org) and rssi (business): those accounts can change
// anyone's permissions, read every tenant's risk register and mint API tokens,
// so a password alone is not a proportionate gate. Ordinary members keep MFA
// optional — mandating it for everyone in a product sold into organisations
// that may not all have authenticators is how you get shared accounts.
//
// "Mandatory" no longer means "immediately" (OR26-03). It means: within the
// tenant's grace window, after which the server refuses access. See DecideMFA.
func (uc *LoginUseCase) RequireMFAForRoles(orgRoles []string, businessRoles []string) *LoginUseCase {
	uc.privileged = domain.NewMFAPrivilegeSet(orgRoles, businessRoles)
	return uc
}

// WithMFAPolicies wires the tenant grace-window reader.
func (uc *LoginUseCase) WithMFAPolicies(r MFAPolicyReader) *LoginUseCase {
	uc.mfaPolicies = r
	return uc
}

// WithClock overrides the clock the grace arithmetic reads.
func (uc *LoginUseCase) WithClock(f func() time.Time) *LoginUseCase {
	uc.now = f
	return uc
}

func (uc *LoginUseCase) clock() time.Time {
	if uc.now != nil {
		return uc.now()
	}
	return time.Now()
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
		// A revoked (deactivated) membership must not yield a session, even though
		// the user account itself is active: being removed from an organization is
		// exactly the case where a password must stop granting access to it. The
		// token resolver enforces the same predicate on refresh, so a session
		// cannot outlive the membership that authorized it.
		if !member.IsActive {
			return nil, domain.NewValidationError("your access to this organization has been revoked")
		}
		orgRoles[org.ID] = string(member.Role)
		// EffectivePermissions unifies root/admin wildcard, the business-role
		// preset, and any legacy profile rules (see domain.OrganizationMember).
		permissions = member.EffectivePermissions()
		businessRole = member.BusinessRole
	}

	// L4 — MFA enforcement. If the user has a verified MFA secret, do NOT issue a
	// full session yet: hand back a short-lived MFA_REQUIRED token that only
	// /auth/mfa/challenge accepts. The real pair is minted after code validation.
	//
	// A deployment that wired no MFA repository at all cannot enrol or enforce
	// anything, so the honest resolved state is "recommended" — the banner is all
	// there is to say. Seeding it here means the field is never the empty string,
	// which the SPA would have to special-case.
	mfaStatus := domain.DecideMFA(domain.MFADecisionInput{GraceDays: domain.MFAGraceDaysDefault, Now: uc.clock()})
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
				// Reported for completeness; no session is issued on this leg, so
				// nothing reads it until the challenge succeeds.
				MFAStatus: uc.decideMFA(ctx, member, org.ID, true),
			}, nil
		}

		// OR26-03 — nothing enrolled. Whether that stops the login is now a
		// POLICY question, not a role check: an ordinary member is let in with a
		// banner, and a privileged one is let in until their grace window runs
		// out. Only past the deadline does login stop short, and then it stops
		// exactly as before — no session, an enrolment token, nothing else.
		mfaStatus = uc.decideMFA(ctx, member, org.ID, false)
		if mfaStatus.Required {
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
				MFAStatus:             mfaStatus,
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
		MFAStatus:    mfaStatus,
	}, nil
}

// decideMFA resolves the requirement for one member against the tenant policy.
//
// Nil-safe throughout: no member, no policy reader, or a policy read that fails
// all fall back to the shipped default window rather than to "no requirement".
// A policy lookup that errors must never be the thing that lets a privileged
// account in — the default is 7 days, not forever.
func (uc *LoginUseCase) decideMFA(ctx context.Context, member *domain.OrganizationMember, tenantID uuid.UUID, enrolled bool) domain.MFADecision {
	policy := domain.DefaultMFAPolicy(tenantID)
	if uc.mfaPolicies != nil {
		if p, err := uc.mfaPolicies.GetMFAPolicy(ctx, tenantID); err == nil && p != nil {
			policy = *p
		}
	}

	in := domain.MFADecisionInput{
		Enrolled:  enrolled,
		GraceDays: policy.EffectiveGraceDays(),
		Now:       uc.clock(),
	}
	if member != nil && !uc.privileged.Empty() {
		in.Privileged = uc.privileged.Includes(member.Role, member.BusinessRole)
		in.GraceStartedAt = member.MFAGraceAnchor()
	}
	return domain.DecideMFA(in)
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
