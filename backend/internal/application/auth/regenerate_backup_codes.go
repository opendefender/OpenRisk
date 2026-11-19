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
	"golang.org/x/crypto/bcrypt"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// ---------------------------------------------------------------------------
// Regenerating MFA backup codes (#1032).
//
// New codes are a substitute for the second factor, so issuing them requires
// that factor: a current code from the authenticator app, always, plus the
// password for an account that has one. An existing backup code is not
// accepted: whoever holds only a backup code has a way in, not the factor, and
// re-enrols instead. This is the path for a role that may not turn MFA off
// (ErrMFARequiredByRole) and whose codes were spent, lost or purged (#1001).
// ---------------------------------------------------------------------------

var (
	// ErrBackupCodesPasswordIncorrect covers a wrong and a missing password
	// alike. Deliberately unspecific.
	ErrBackupCodesPasswordIncorrect = errors.New("password is incorrect")
	// ErrBackupCodesCodeIncorrect covers a wrong, missing or replayed
	// authenticator code alike (#849).
	ErrBackupCodesCodeIncorrect = errors.New("authentication code is incorrect")
)

// RegenerateBackupCodesUserLookup reads the account. Satisfied by
// *repository.GormUserRepository.
type RegenerateBackupCodesUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// MFABackupCodesRegeneratedMailer sends the regeneration notice.
type MFABackupCodesRegeneratedMailer interface {
	SendMFABackupCodesRegenerated(ctx context.Context, to, fullName, locale string) error
}

// RegenerateBackupCodesInput carries the proofs. UserID and TenantID come from
// the session, never from the body.
type RegenerateBackupCodesInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	// Password is required when the account has a local password; ignored
	// otherwise.
	Password string
	// Code is a current TOTP code from the authenticator app. Always required.
	Code   string
	Locale string
}

// RegenerateBackupCodesOutput holds the new codes in clear. They are shown
// once and never stored in clear.
type RegenerateBackupCodesOutput struct {
	BackupCodes []string `json:"backup_codes"`
}

// RegenerateBackupCodesUseCase replaces the caller's backup codes.
type RegenerateBackupCodesUseCase struct {
	mfaRepo        repository.MFARepository
	users          RegenerateBackupCodesUserLookup
	passwordHasher PasswordHasher
	// totpKey decrypts the stored secret to check Code.
	totpKey []byte
	mailer  MFABackupCodesRegeneratedMailer
	// limits is the in-session lock shared with disabling MFA (#1392).
	// Optional; main.go always wires it.
	limits *MFAInSessionLimits
}

// NewRegenerateBackupCodesUseCase wires the use case. totpKey is the key the
// secrets were encrypted with.
func NewRegenerateBackupCodesUseCase(mfaRepo repository.MFARepository, users RegenerateBackupCodesUserLookup, passwordHasher PasswordHasher, totpKey []byte) *RegenerateBackupCodesUseCase {
	return &RegenerateBackupCodesUseCase{
		mfaRepo:        mfaRepo,
		users:          users,
		passwordHasher: passwordHasher,
		totpKey:        totpKey,
	}
}

// WithMailer wires the regeneration notice. Optional.
func (uc *RegenerateBackupCodesUseCase) WithMailer(m MFABackupCodesRegeneratedMailer) *RegenerateBackupCodesUseCase {
	uc.mailer = m
	return uc
}

// WithInSessionLimits counts wrong proofs toward the lock shared with disabling
// MFA (#1392). Pass the same instance to both use cases.
func (uc *RegenerateBackupCodesUseCase) WithInSessionLimits(l *MFAInSessionLimits) *RegenerateBackupCodesUseCase {
	uc.limits = l
	return uc
}

// wrongProof counts a wrong password or code toward the in-session lock. The
// failure that sets the lock is answered with the lock.
func (uc *RegenerateBackupCodesUseCase) wrongProof(ctx context.Context, userID uuid.UUID, proofErr error) error {
	if err := uc.limits.RecordFailure(ctx, userID); err != nil {
		return err
	}
	return proofErr
}

// Execute checks the password (when the account has one), then the
// authenticator code, then swaps the codes in one transaction and mails the
// owner. On any failure the old codes stay.
func (uc *RegenerateBackupCodesUseCase) Execute(ctx context.Context, input RegenerateBackupCodesInput) (*RegenerateBackupCodesOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("authentication required")
	}
	if len(uc.totpKey) == 0 {
		// Without the key no code can be checked: refuse rather than let the
		// session alone mint a factor.
		return nil, errors.New("auth.RegenerateBackupCodes: no TOTP key configured")
	}
	// Before any proof is looked at: under the lock even the right one is
	// refused (#1392).
	if err := uc.limits.Check(ctx, input.UserID); err != nil {
		return nil, err
	}

	user, err := uc.users.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth.RegenerateBackupCodes: load user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, domain.NewNotFoundError("user", input.UserID)
	}
	if user.Password != "" && (input.Password == "" || !uc.passwordHasher.Verify(user.Password, input.Password)) {
		return nil, uc.wrongProof(ctx, input.UserID, ErrBackupCodesPasswordIncorrect)
	}

	secret, err := uc.mfaRepo.GetMFASecret(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("auth.RegenerateBackupCodes: load secret: %w", err)
	}
	if secret == nil || !secret.IsVerified {
		// An unfinished enrolment is not MFA: it gets its codes from setup.
		return nil, domain.NewNotFoundError("MFA secret", input.UserID)
	}
	if input.Code == "" {
		return nil, uc.wrongProof(ctx, input.UserID, ErrBackupCodesCodeIncorrect)
	}
	plain, err := crypto.DecryptAES256GCM(secret.SecretEncrypted, uc.totpKey)
	if err != nil {
		return nil, fmt.Errorf("auth.RegenerateBackupCodes: decrypt secret: %w", err)
	}
	accepted, err := acceptTOTP(ctx, uc.mfaRepo, input.UserID, input.TenantID, plain, input.Code)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, uc.wrongProof(ctx, input.UserID, ErrBackupCodesCodeIncorrect)
	}

	codes, err := otp.GenerateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("auth.RegenerateBackupCodes: generate: %w", err)
	}
	// Hashed before the write: bcrypt is slow, and the write holds a row lock.
	hashed := make([]*domain.MFABackupCode, 0, len(codes))
	for _, code := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("auth.RegenerateBackupCodes: hash: %w", err)
		}
		hashed = append(hashed, &domain.MFABackupCode{UserID: input.UserID, TenantID: input.TenantID, CodeHash: string(hash)})
	}

	replaced, err := uc.mfaRepo.ReplaceBackupCodes(ctx, input.UserID, input.TenantID, hashed)
	if err != nil {
		return nil, fmt.Errorf("auth.RegenerateBackupCodes: replace: %w", err)
	}
	if !replaced {
		// MFA was turned off between the read and the write.
		return nil, domain.NewNotFoundError("MFA secret", input.UserID)
	}
	uc.limits.RecordSuccess(ctx, input.UserID)

	if uc.mailer != nil {
		_ = uc.mailer.SendMFABackupCodesRegenerated(ctx, user.Email, user.FullName, normaliseLocale(input.Locale))
	}
	return &RegenerateBackupCodesOutput{BackupCodes: codes}, nil
}
