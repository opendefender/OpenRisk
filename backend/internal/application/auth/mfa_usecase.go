package auth

import (
	"context"
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
	Secret      string   `json:"secret"` // Base32-encoded TOTP secret
	QRCode      string   `json:"qr_code"` // Base64-encoded JPEG
	BackupCodes []string `json:"backup_codes"` // 8 backup codes
}

// SetupMFAUseCase handles MFA setup
type SetupMFAUseCase struct {
	mfaRepo   repository.MFARepository
	encKey    []byte // 32-byte AES-256 key
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

	// Generate backup codes
	backupCodes := otp.GenerateBackupCodes()

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
	Verified  bool   `json:"verified"`
	Message   string `json:"message"`
}

// VerifyMFAUseCase handles MFA verification (activates MFA)
type VerifyMFAUseCase struct {
	mfaRepo   repository.MFARepository
	userRepo  repository.GormUserRepository
	encKey    []byte
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

// DisableMFAInput represents MFA disable request
type DisableMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Password string // Current password (required for security)
}

// DisableMFAOutput represents MFA disable response
type DisableMFAOutput struct {
	Message string `json:"message"`
}

// DisableMFAUseCase handles MFA disable
type DisableMFAUseCase struct {
	mfaRepo        repository.MFARepository
	passwordHasher PasswordHasher
}

// NewDisableMFAUseCase creates a new disable MFA use case
func NewDisableMFAUseCase(mfaRepo repository.MFARepository, passwordHasher PasswordHasher) *DisableMFAUseCase {
	return &DisableMFAUseCase{
		mfaRepo:        mfaRepo,
		passwordHasher: passwordHasher,
	}
}

// Execute disables MFA for user
func (uc *DisableMFAUseCase) Execute(ctx context.Context, input DisableMFAInput) (*DisableMFAOutput, error) {
	if input.UserID == uuid.Nil || input.TenantID == uuid.Nil {
		return nil, domain.NewValidationError("user_id and tenant_id required")
	}

	// TODO: Verify password before disabling (requires user repo + password verification)

	// Delete MFA secret and backup codes
	if err := uc.mfaRepo.DisableMFA(ctx, input.UserID, input.TenantID); err != nil {
		return nil, fmt.Errorf("failed to disable MFA: %w", err)
	}

	if err := uc.mfaRepo.DeleteBackupCodes(ctx, input.UserID, input.TenantID); err != nil {
		return nil, fmt.Errorf("failed to delete backup codes: %w", err)
	}

	return &DisableMFAOutput{
		Message: "MFA disabled successfully",
	}, nil
}

// ChallengeMFAInput represents MFA challenge request (after login)
type ChallengeMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Code     string // TOTP code OR backup code
}

// ChallengeMFAOutput represents MFA challenge response
type ChallengeMFAOutput struct {
	Verified  bool   `json:"verified"`
	Message   string `json:"message"`
}

// ChallengeMFAUseCase handles MFA challenge during login
type ChallengeMFAUseCase struct {
	mfaRepo repository.MFARepository
	encKey  []byte
}

// NewChallengeMFAUseCase creates a new challenge MFA use case
func NewChallengeMFAUseCase(mfaRepo repository.MFARepository, encKey []byte) *ChallengeMFAUseCase {
	return &ChallengeMFAUseCase{
		mfaRepo: mfaRepo,
		encKey:  encKey,
	}
}

// Execute verifies TOTP or backup code during login
func (uc *ChallengeMFAUseCase) Execute(ctx context.Context, input ChallengeMFAInput) (*ChallengeMFAOutput, error) {
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

	return nil, domain.NewValidationError("invalid MFA code")
}
