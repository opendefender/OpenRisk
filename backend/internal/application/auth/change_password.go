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

// OtherSessionsRevoker ends every session of a user but the one whose refresh
// token hashes to keepHash. Satisfied by the session repository.
type OtherSessionsRevoker interface {
	RevokeAllExcept(ctx context.Context, userID uuid.UUID, keepHash string) (int64, error)
}

// PasswordChangedMailer sends the in-session change notice.
type PasswordChangedMailer interface {
	SendPasswordChanged(ctx context.Context, to, fullName, locale string) error
}

// ChangePasswordInput is the request. UserID and CurrentSessionHash come from
// the session, never from the body.
type ChangePasswordInput struct {
	UserID             uuid.UUID
	CurrentPassword    string
	NewPassword        string
	CurrentSessionHash string
	Locale             string
}

// ChangePasswordOutput reports the result; Assessment is set on a policy refusal.
type ChangePasswordOutput struct {
	OtherSessionsRevoked int64
	Assessment           *pwpolicy.Assessment
}

// ChangePasswordUseCase changes the signed-in user's own password.
type ChangePasswordUseCase struct {
	users    ResetUserRepository
	hasher   PasswordHasher
	policy   PasswordAssessor
	sessions OtherSessionsRevoker
	mailer   PasswordChangedMailer
}

// NewChangePasswordUseCase builds the use case. sessions and mailer may be nil.
func NewChangePasswordUseCase(users ResetUserRepository, hasher PasswordHasher, policy PasswordAssessor, sessions OtherSessionsRevoker, mailer PasswordChangedMailer) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{users: users, hasher: hasher, policy: policy, sessions: sessions, mailer: mailer}
}

// Execute verifies the current password, applies the policy to the new one,
// stores it, and signs out every other device.
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
	if uc.sessions != nil {
		n, err := uc.sessions.RevokeAllExcept(ctx, user.ID, in.CurrentSessionHash)
		if err != nil {
			return out, fmt.Errorf("password changed but other sessions could not be revoked: %w", err)
		}
		out.OtherSessionsRevoked = n
	}
	if uc.mailer != nil {
		_ = uc.mailer.SendPasswordChanged(ctx, user.Email, user.FullName, normaliseLocale(in.Locale))
	}
	return out, nil
}
