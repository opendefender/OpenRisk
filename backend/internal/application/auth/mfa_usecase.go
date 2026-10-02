// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
	"golang.org/x/crypto/bcrypt"
)

// SetupMFAInput represents MFA setup request
type SetupMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Email    string
}

// SetupMFAOutput represents MFA setup response
type SetupMFAOutput struct {
	Secret      string   `json:"secret"`       // Base32-encoded TOTP secret
	QRCode      string   `json:"qr_code"`      // Base64-encoded JPEG
	BackupCodes []string `json:"backup_codes"` // 8 backup codes
}

// SetupMFAUseCase handles MFA setup
type SetupMFAUseCase struct {
	mfaRepo repository.MFARepository
	encKey  []byte // 32-byte AES-256 key
}

// NewSetupMFAUseCase creates a new setup MFA use case
func NewSetupMFAUseCase(mfaRepo repository.MFARepository, encKey []byte) *SetupMFAUseCase {
	return &SetupMFAUseCase{
		mfaRepo: mfaRepo,
		encKey:  encKey,
	}
}

// Execute generates TOTP secret, QR code, and backup codes
func (uc *SetupMFAUseCase) Execute(ctx context.Context, input SetupMFAInput) (*SetupMFAOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewValidationError("user_id and tenant_id required")
	}
	if input.Email == "" {
		return nil, domain.NewValidationError("email required")
	}

	// Check if MFA already exists
	existingSecret, err := uc.mfaRepo.GetMFASecret(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing MFA: %w", err)
	}
	if existingSecret != nil && existingSecret.IsVerified {
		return nil, domain.NewConflictError("MFA", "already_enabled")
	}

	// Generate TOTP secret
	secret, err := otp.GenerateTOTPSecret2(input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Generate QR code
	qrCode, err := otp.GetTOTPQRCode(secret, input.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Encrypt secret before storage
	encryptedSecret, err := crypto.EncryptAES256GCM(secret, uc.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %w", err)
	}

	// Store encrypted secret (not yet verified)
	mfaSecret := &domain.MFASecret{
		UserID:          input.UserID,
		TenantID:        input.TenantID,
		SecretEncrypted: encryptedSecret,
		IsVerified:      false,
	}

	if err := uc.mfaRepo.CreateMFASecret(ctx, mfaSecret); err != nil {
		return nil, fmt.Errorf("failed to store MFA secret: %w", err)
	}

	// Generate backup codes (CSPRNG, unique per user — see otp.GenerateBackupCodes).
	backupCodes, err := otp.GenerateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Replace any previously-issued codes for this user so re-enrolment
	// invalidates the old set (defence in depth against a stale/compromised set).
	if err := uc.mfaRepo.DeleteBackupCodes(ctx, input.UserID, input.TenantID); err != nil {
		return nil, fmt.Errorf("failed to clear previous backup codes: %w", err)
	}

	// Hash and store backup codes
	var hashedCodes []*domain.MFABackupCode
	for _, code := range backupCodes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash backup code: %w", err)
		}

		hashedCodes = append(hashedCodes, &domain.MFABackupCode{
			UserID:   input.UserID,
			TenantID: input.TenantID,
			CodeHash: string(hash),
		})
	}

	if err := uc.mfaRepo.SaveBackupCodes(ctx, hashedCodes); err != nil {
		return nil, fmt.Errorf("failed to store backup codes: %w", err)
	}

	return &SetupMFAOutput{
		Secret:      secret,
		QRCode:      qrCode,
		BackupCodes: backupCodes,
	}, nil
}

// VerifyMFAInput represents MFA verification request
type VerifyMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Code     string // TOTP code (6 digits)
}

// VerifyMFAOutput represents MFA verification response
type VerifyMFAOutput struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// VerifyMFAUseCase handles MFA verification (activates MFA)
type VerifyMFAUseCase struct {
	mfaRepo  repository.MFARepository
	userRepo repository.GormUserRepository
	encKey   []byte
}

// NewVerifyMFAUseCase creates a new verify MFA use case
func NewVerifyMFAUseCase(mfaRepo repository.MFARepository, userRepo repository.GormUserRepository, encKey []byte) *VerifyMFAUseCase {
	return &VerifyMFAUseCase{
		mfaRepo:  mfaRepo,
		userRepo: userRepo,
		encKey:   encKey,
	}
}

// Execute verifies TOTP code and activates MFA
func (uc *VerifyMFAUseCase) Execute(ctx context.Context, input VerifyMFAInput) (*VerifyMFAOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewValidationError("user_id and tenant_id required")
	}
	if input.Code == "" {
		return nil, domain.NewValidationError("code required")
	}

	// Get MFA secret
	mfaSecret, err := uc.mfaRepo.GetMFASecret(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA secret: %w", err)
	}
	if mfaSecret == nil {
		return nil, domain.NewNotFoundError("MFA secret", input.UserID)
	}

	// Decrypt secret
	decryptedSecret, err := crypto.DecryptAES256GCM(mfaSecret.SecretEncrypted, uc.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret: %w", err)
	}

	// Verify TOTP code (±1 window)
	if !otp.VerifyTOTP(decryptedSecret, input.Code) {
		return nil, domain.NewValidationError("invalid TOTP code")
	}

	// Mark MFA as verified
	mfaSecret.IsVerified = true
	now := time.Now()
	mfaSecret.VerifiedAt = &now

	if err := uc.mfaRepo.UpdateMFASecret(ctx, mfaSecret); err != nil {
		return nil, fmt.Errorf("failed to update MFA secret: %w", err)
	}

	return &VerifyMFAOutput{
		Verified: true,
		Message:  "MFA activated successfully",
	}, nil
}

// ---------------------------------------------------------------------------
// Disabling MFA (#754).
//
// Removing a factor is the one MFA operation an attacker at an unlocked
// workstation wants most, so the session alone is not enough: the caller must
// prove the password again. An account that signs in through an identity
// provider has no password here, so it proves possession of the factor it is
// removing instead: a current code from the authenticator app. Backup codes do
// not count — they are the factor most often written down next to the desk. Privileged roles cannot remove it at all — the
// login policy would demand it back on the next sign-in, and in between the
// account would sit on a password alone.
// ---------------------------------------------------------------------------

var (
	// ErrMFADisablePasswordIncorrect covers a wrong and a missing password alike.
	// Deliberately unspecific.
	ErrMFADisablePasswordIncorrect = errors.New("password is incorrect")
	// ErrMFARequiredByRole refuses the removal for a role the deployment requires
	// MFA for.
	ErrMFARequiredByRole = errors.New("two-factor authentication is required for this role")
	// ErrMFADisableCodeIncorrect covers a wrong and a missing authenticator code
	// alike, for an account without a local password.
	ErrMFADisableCodeIncorrect = errors.New("authentication code is incorrect")
)

// DisableMFAUserLookup reads the account and its membership. Satisfied by
// *repository.GormUserRepository.
type DisableMFAUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
}

// MFADisabledMailer sends the deactivation notice.
type MFADisabledMailer interface {
	SendMFADisabled(ctx context.Context, to, fullName, locale string) error
}

// MFADisabledInAppNotifier records the in-app deactivation notice.
type MFADisabledInAppNotifier func(ctx context.Context, tenantID, userID uuid.UUID, subject, message string)

// DisableMFAInput represents MFA disable request. UserID, TenantID and
// OrgRoleHint come from the session, never from the body.
type DisableMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Password string // Current password (required for security)
	// Code is a current TOTP code. Required instead of Password when the account
	// has no local password (identity-provider sign-in); ignored otherwise.
	Code string
	// OrgRoleHint is the org role in the caller's signed token. It can only
	// widen the privileged check, never narrow it (same rule as MFAStatusResolver).
	OrgRoleHint string
	Locale      string
}

// DisableMFAOutput represents MFA disable response
type DisableMFAOutput struct {
	Message string `json:"message"`
}

// DisableMFAUseCase handles MFA disable
type DisableMFAUseCase struct {
	mfaRepo        repository.MFARepository
	users          DisableMFAUserLookup
	passwordHasher PasswordHasher
	privileged     domain.MFAPrivilegeSet
	mailer         MFADisabledMailer
	inApp          MFADisabledInAppNotifier
	// totpKey decrypts the stored secret to check Code. Without it an account
	// with no local password cannot prove anything and is refused.
	totpKey []byte
}

// NewDisableMFAUseCase creates a new disable MFA use case
func NewDisableMFAUseCase(mfaRepo repository.MFARepository, users DisableMFAUserLookup, passwordHasher PasswordHasher) *DisableMFAUseCase {
	return &DisableMFAUseCase{
		mfaRepo:        mfaRepo,
		users:          users,
		passwordHasher: passwordHasher,
	}
}

// RequireMFAForRoles names the roles that may not remove their factor. Pass the
// same lists login enforces, so the two can never disagree.
func (uc *DisableMFAUseCase) RequireMFAForRoles(orgRoles, businessRoles []string) *DisableMFAUseCase {
	uc.privileged = domain.NewMFAPrivilegeSet(orgRoles, businessRoles)
	return uc
}

// WithTOTPKey lets an account without a local password confirm with a current
// authenticator code. Pass the same key the secrets were encrypted with.
func (uc *DisableMFAUseCase) WithTOTPKey(key []byte) *DisableMFAUseCase {
	uc.totpKey = key
	return uc
}

// WithMailer wires the deactivation notice. Optional.
func (uc *DisableMFAUseCase) WithMailer(m MFADisabledMailer) *DisableMFAUseCase {
	uc.mailer = m
	return uc
}

// WithInAppNotifier wires the in-app deactivation notice. Optional.
func (uc *DisableMFAUseCase) WithInAppNotifier(n MFADisabledInAppNotifier) *DisableMFAUseCase {
	uc.inApp = n
	return uc
}

// Execute verifies the password, refuses privileged roles, then deletes the
// secret and the backup codes in one transaction and notifies the owner.
func (uc *DisableMFAUseCase) Execute(ctx context.Context, input DisableMFAInput) (*DisableMFAOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("authentication required")
	}

	user, err := uc.users.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth.DisableMFA: load user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, domain.NewNotFoundError("user", input.UserID)
	}
	hasPassword := user.Password != ""
	if !hasPassword && uc.totpKey == nil {
		// Nothing this account could prove. Refuse rather than let the session
		// alone remove the factor.
		return nil, ErrNoLocalPassword
	}
	if hasPassword && (input.Password == "" || !uc.passwordHasher.Verify(user.Password, input.Password)) {
		return nil, ErrMFADisablePasswordIncorrect
	}

	secret, err := uc.mfaRepo.GetMFASecret(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("auth.DisableMFA: load secret: %w", err)
	}
	if secret == nil {
		return nil, domain.NewNotFoundError("MFA secret", input.UserID)
	}
	if !hasPassword {
		plain, err := crypto.DecryptAES256GCM(secret.SecretEncrypted, uc.totpKey)
		if err != nil {
			return nil, fmt.Errorf("auth.DisableMFA: decrypt secret: %w", err)
		}
		if input.Code == "" || !otp.VerifyTOTP(plain, input.Code) {
			return nil, ErrMFADisableCodeIncorrect
		}
	}

	if !uc.privileged.Empty() {
		member, err := uc.users.GetOrganizationMember(ctx, input.UserID, input.TenantID)
		if err != nil {
			// Cannot tell whether the role requires MFA: refuse rather than guess.
			return nil, fmt.Errorf("auth.DisableMFA: load membership: %w", err)
		}
		privileged := uc.privileged.Includes(domain.MemberRole(input.OrgRoleHint), "")
		if member != nil && uc.privileged.Includes(member.Role, member.BusinessRole) {
			privileged = true
		}
		if privileged {
			return nil, ErrMFARequiredByRole
		}
	}

	if err := uc.mfaRepo.DisableMFA(ctx, input.UserID, input.TenantID); err != nil {
		return nil, fmt.Errorf("auth.DisableMFA: %w", err)
	}

	if uc.mailer != nil {
		_ = uc.mailer.SendMFADisabled(ctx, user.Email, user.FullName, normaliseLocale(input.Locale))
	}
	if uc.inApp != nil {
		subject, message := mfaDisabledInAppCopy(normaliseLocale(input.Locale))
		uc.inApp(ctx, input.TenantID, input.UserID, subject, message)
	}

	return &DisableMFAOutput{
		Message: "MFA disabled successfully",
	}, nil
}

func mfaDisabledInAppCopy(locale string) (string, string) {
	if locale == "en" {
		return "Two-factor authentication turned off",
			"Two-factor authentication was turned off on your account after your identity was confirmed. If this wasn't you, secure your account and turn it back on from Settings → Security."
	}
	return "Double authentification désactivée",
		"La double authentification a été désactivée sur votre compte après confirmation de votre identité. Si ce n'est pas vous, sécurisez votre compte et réactivez-la depuis Paramètres → Sécurité."
}

// ChallengeMFAInput represents MFA challenge request (after login)
type ChallengeMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Code     string // TOTP code OR backup code
	// ChallengeJTI and ChallengeExpiresAt identify the MFA_REQUIRED token the
	// request came with. Attempts are counted against it (#689).
	ChallengeJTI       string
	ChallengeExpiresAt time.Time
}

// ChallengeMFAOutput represents MFA challenge response
type ChallengeMFAOutput struct {
	Verified bool   `json:"verified"`
	Message  string `json:"message"`
}

// ChallengeMFAUseCase handles MFA challenge during login
type ChallengeMFAUseCase struct {
	mfaRepo repository.MFARepository
	encKey  []byte
	limits  *challengeLimits
}

// NewChallengeMFAUseCase creates a new challenge MFA use case
func NewChallengeMFAUseCase(mfaRepo repository.MFARepository, encKey []byte) *ChallengeMFAUseCase {
	return &ChallengeMFAUseCase{
		mfaRepo: mfaRepo,
		encKey:  encKey,
	}
}

// Execute verifies TOTP or backup code during login.
//
// With limits wired, every attempt is reserved against the challenge token and
// the account before the code is checked, so parallel requests cannot all slip
// under the limit together. A TOTP code and a backup code are the same attempt.
func (uc *ChallengeMFAUseCase) Execute(ctx context.Context, input ChallengeMFAInput) (*ChallengeMFAOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewValidationError("user_id and tenant_id required")
	}
	if input.Code == "" {
		return nil, domain.NewValidationError("code required")
	}

	if uc.limits != nil {
		if err := uc.limits.reserve(ctx, input); err != nil {
			return nil, err
		}
	}

	out, err := uc.verify(ctx, input)
	if errors.Is(err, errMFACodeMismatch) {
		if uc.limits != nil {
			if limitErr := uc.limits.recordFailure(ctx, input); limitErr != nil {
				return nil, limitErr
			}
		}
		return nil, domain.NewValidationError("invalid MFA code")
	}
	if err != nil {
		return nil, err
	}

	if uc.limits != nil {
		uc.limits.recordSuccess(ctx, input)
	}
	return out, nil
}

// errMFACodeMismatch is the one outcome that counts as a failed guess.
var errMFACodeMismatch = errors.New("mfa code mismatch")

func (uc *ChallengeMFAUseCase) verify(ctx context.Context, input ChallengeMFAInput) (*ChallengeMFAOutput, error) {
	// Get MFA secret
	mfaSecret, err := uc.mfaRepo.GetMFASecret(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA secret: %w", err)
	}
	if mfaSecret == nil {
		return nil, domain.NewNotFoundError("MFA secret", input.UserID)
	}

	// Decrypt secret
	decryptedSecret, err := crypto.DecryptAES256GCM(mfaSecret.SecretEncrypted, uc.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret: %w", err)
	}

	// Try TOTP first
	if otp.VerifyTOTP(decryptedSecret, input.Code) {
		// Mark as last used
		now := time.Now()
		mfaSecret.LastUsedAt = &now
		_ = uc.mfaRepo.UpdateMFASecret(ctx, mfaSecret)

		return &ChallengeMFAOutput{
			Verified: true,
			Message:  "MFA verified successfully",
		}, nil
	}

	// Try backup code
	codes, err := uc.mfaRepo.GetUnusedBackupCodes(ctx, input.UserID, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup codes: %w", err)
	}

	for _, code := range codes {
		if err := bcrypt.CompareHashAndPassword([]byte(code.CodeHash), []byte(input.Code)); err == nil {
			// Backup code matched, mark as used
			if err := uc.mfaRepo.MarkBackupCodeAsUsed(ctx, code.ID); err != nil {
				return nil, fmt.Errorf("failed to mark backup code as used: %w", err)
			}

			return &ChallengeMFAOutput{
				Verified: true,
				Message:  "Backup code verified successfully",
			}, nil
		}
	}

	return nil, errMFACodeMismatch
}
