// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// In-session password change (#720, D-047). Built from the parts the product
// already trusts: the argon2id hasher, the shared password policy, and the
// "sign out other devices" revocation.

var (
	// ErrCurrentPasswordIncorrect is deliberately unspecific about why.
	ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
	// ErrNoLocalPassword marks an account whose password belongs to an identity provider.
	ErrNoLocalPassword = errors.New("this account has no local password")
	// ErrSamePassword refuses a "change" that changes nothing.
	ErrSamePassword = errors.New("the new password must differ from the current one")
)

// SessionReissuer ends every session of a user and mints a fresh one for the
// device that made the change. Satisfied by *auth.TokenManager.
//
// Why not "revoke all but the current session": the refresh cookie is scoped to
// /api/v1/auth/refresh, so no other route can tell which refresh row belongs to
// the caller, and an empty keep-hash would silently sign the caller out too.
// Re-issuing is also the stronger outcome: the device that proved the new
// password starts a new session lineage, and every old one is gone.
type SessionReissuer interface {
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
	IssueSessionForOrg(ctx context.Context, userID, orgID uuid.UUID, device auth.DeviceContext) (*auth.TokenPair, error)
}

// PasswordChangedMailer sends the in-session change notice.
type PasswordChangedMailer interface {
	SendPasswordChanged(ctx context.Context, to, fullName, locale string) error
}

// ChangePasswordInput is the request. UserID and CurrentSessionHash come from
// the session, never from the body.
type ChangePasswordInput struct {
	UserID          uuid.UUID
	TenantID        uuid.UUID
	CurrentPassword string
	NewPassword     string
	Device          auth.DeviceContext
	Locale          string
}

// ChangePasswordOutput reports the result. Assessment is set on a policy
// refusal. TokenPair is the caller's new session; when it is nil after a
// successful change, the caller must sign in again.
type ChangePasswordOutput struct {
	TokenPair  *auth.TokenPair
	Assessment *pwpolicy.Assessment
}

// ChangePasswordUseCase changes the signed-in user's own password.
type ChangePasswordUseCase struct {
	users    ResetUserRepository
	hasher   PasswordHasher
	policy   PasswordAssessor
	sessions SessionReissuer
	mailer   PasswordChangedMailer
}

// NewChangePasswordUseCase builds the use case. sessions and mailer may be nil.
func NewChangePasswordUseCase(users ResetUserRepository, hasher PasswordHasher, policy PasswordAssessor, sessions SessionReissuer, mailer PasswordChangedMailer) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{users: users, hasher: hasher, policy: policy, sessions: sessions, mailer: mailer}
}

// Execute verifies the current password, applies the policy to the new one,
// stores it, ends every session and re-issues one for the calling device.
func (uc *ChangePasswordUseCase) Execute(ctx context.Context, in ChangePasswordInput) (*ChangePasswordOutput, error) {
	if in.UserID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("authentication required")
	}
	user, err := uc.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth.ChangePassword: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, domain.NewNotFoundError("user", in.UserID)
	}
	if user.Password == "" {
		return nil, ErrNoLocalPassword
	}
	if in.CurrentPassword == "" || !uc.hasher.Verify(user.Password, in.CurrentPassword) {
		return nil, ErrCurrentPasswordIncorrect
	}
	if in.NewPassword == in.CurrentPassword {
		return nil, ErrSamePassword
	}

	assessment := uc.policy.Assess(ctx, in.NewPassword, identityInputs(user))
	if !assessment.OK {
		return &ChangePasswordOutput{Assessment: &assessment}, domain.NewValidationError(assessment.Blocking[0].EN)
	}

	hashed, err := uc.hasher.Hash(in.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("auth.ChangePassword: hash: %w", err)
	}
	user.Password = hashed
	if err := uc.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("auth.ChangePassword: update: %w", err)
	}

	out := &ChangePasswordOutput{}
	if uc.mailer != nil {
		_ = uc.mailer.SendPasswordChanged(ctx, user.Email, user.FullName, normaliseLocale(in.Locale))
	}
	if uc.sessions != nil {
		if err := uc.sessions.RevokeAllUserTokens(ctx, user.ID); err != nil {
			return out, fmt.Errorf("password changed but sessions could not be revoked: %w", err)
		}
		// A failure to mint the new session is not a failed change: the password
		// is stored and every old session is gone. The caller signs in again.
		if in.TenantID != uuid.Nil {
			if pair, err := uc.sessions.IssueSessionForOrg(ctx, user.ID, in.TenantID, in.Device); err == nil {
				out.TokenPair = pair
			}
		}
	}
	return out, nil
}
