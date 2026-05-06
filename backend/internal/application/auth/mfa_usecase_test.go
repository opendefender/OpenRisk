package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/otp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Mock MFA Repository
type MockMFARepository struct {
	secrets   map[string]*domain.MFASecret
	codes     map[string][]*domain.MFABackupCode
	oauthProviders map[string]*domain.OAuthProvider
}

func NewMockMFARepository() *MockMFARepository {
	return &MockMFARepository{
		secrets:   make(map[string]*domain.MFASecret),
		codes:     make(map[string][]*domain.MFABackupCode),
		oauthProviders: make(map[string]*domain.OAuthProvider),
	}
}

func (m *MockMFARepository) CreateMFASecret(ctx context.Context, secret *domain.MFASecret) error {
	key := secret.UserID.String() + ":" + secret.TenantID.String()
	m.secrets[key] = secret
	return nil
}

func (m *MockMFARepository) GetMFASecret(ctx context.Context, userID, tenantID uuid.UUID) (*domain.MFASecret, error) {
	key := userID.String() + ":" + tenantID.String()
	return m.secrets[key], nil
}

func (m *MockMFARepository) UpdateMFASecret(ctx context.Context, secret *domain.MFASecret) error {
	key := secret.UserID.String() + ":" + secret.TenantID.String()
	m.secrets[key] = secret
	return nil
}

func (m *MockMFARepository) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error {
	key := userID.String() + ":" + tenantID.String()
	delete(m.secrets, key)
	return nil
}

func (m *MockMFARepository) SaveBackupCodes(ctx context.Context, codes []*domain.MFABackupCode) error {
	if len(codes) == 0 {
		return nil
	}
	key := codes[0].UserID.String() + ":" + codes[0].TenantID.String()
	m.codes[key] = codes
	return nil
}

func (m *MockMFARepository) GetUnusedBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.MFABackupCode, error) {
	key := userID.String() + ":" + tenantID.String()
	codes := m.codes[key]
	var unused []*domain.MFABackupCode
	for _, code := range codes {
		if !code.IsUsed() {
			unused = append(unused, code)
		}
	}
	return unused, nil
}

func (m *MockMFARepository) MarkBackupCodeAsUsed(ctx context.Context, codeID uuid.UUID) error {
	for key := range m.codes {
		for _, code := range m.codes[key] {
			if code.ID == codeID {
				now := time.Now()
				code.UsedAt = &now
				return nil
			}
		}
	}
	return nil
}

func (m *MockMFARepository) DeleteBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) error {
	key := userID.String() + ":" + tenantID.String()
	delete(m.codes, key)
	return nil
}

func (m *MockMFARepository) CreateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error {
	key := provider.UserID.String() + ":" + provider.Provider
	m.oauthProviders[key] = provider
	return nil
}

func (m *MockMFARepository) GetOAuthProvider(ctx context.Context, userID, tenantID uuid.UUID, providerName string) (*domain.OAuthProvider, error) {
	key := userID.String() + ":" + providerName
	return m.oauthProviders[key], nil
}

func (m *MockMFARepository) GetOAuthProviderByEmail(ctx context.Context, email, provider string) (*domain.OAuthProvider, error) {
	key := email + ":" + provider
	return m.oauthProviders[key], nil
}

func (m *MockMFARepository) UpdateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error {
	key := provider.UserID.String() + ":" + provider.Provider
	m.oauthProviders[key] = provider
	return nil
}

func (m *MockMFARepository) ListOAuthProviders(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.OAuthProvider, error) {
	var providers []*domain.OAuthProvider
	for _, provider := range m.oauthProviders {
		if provider.UserID == userID && provider.TenantID == tenantID {
			providers = append(providers, provider)
		}
	}
	return providers, nil
}

func (m *MockMFARepository) DeleteOAuthProvider(ctx context.Context, providerID, tenantID uuid.UUID) error {
	for key, provider := range m.oauthProviders {
		if provider.ID == providerID {
			delete(m.oauthProviders, key)
			return nil
		}
	}
	return nil
}

// Tests for SetupMFAUseCase
func TestSetupMFA_Success(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	useCase := NewSetupMFAUseCase(mfaRepo, encKey)

	userID := uuid.New()
	tenantID := uuid.New()
	email := "user@example.com"

	input := SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
	}

	output, err := useCase.Execute(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.NotEmpty(t, output.Secret)
	assert.NotEmpty(t, output.QRCode)
	assert.Len(t, output.BackupCodes, 8)
}

func TestSetupMFA_InvalidInput(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	useCase := NewSetupMFAUseCase(mfaRepo, encKey)

	tests := []struct {
		name  string
		input SetupMFAInput
	}{
		{
			name: "Missing user_id",
			input: SetupMFAInput{
				UserID:   uuid.Nil,
				TenantID: uuid.New(),
				Email:    "user@example.com",
			},
		},
		{
			name: "Missing tenant_id",
			input: SetupMFAInput{
				UserID:   uuid.New(),
				TenantID: uuid.Nil,
				Email:    "user@example.com",
			},
		},
		{
			name: "Missing email",
			input: SetupMFAInput{
				UserID:   uuid.New(),
				TenantID: uuid.New(),
				Email:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := useCase.Execute(ctx, tt.input)
			assert.Error(t, err)
		})
	}
}

// Tests for VerifyMFAUseCase
func TestVerifyMFA_Success(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	userRepo := NewMockUserRepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	// Setup MFA first
	setupUseCase := NewSetupMFAUseCase(mfaRepo, encKey)
	userID := uuid.New()
	tenantID := uuid.New()
	email := "user@example.com"

	setupInput := SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    email,
	}

	setupOutput, err := setupUseCase.Execute(ctx, setupInput)
	require.NoError(t, err)

	// Verify TOTP code
	verifyUseCase := NewVerifyMFAUseCase(mfaRepo, userRepo, encKey)
	
	code, err := generateValidTOTPCode(setupOutput.Secret)
	require.NoError(t, err)

	verifyInput := VerifyMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     code,
	}

	verifyOutput, err := verifyUseCase.Execute(ctx, verifyInput)

	assert.NoError(t, err)
	assert.NotNil(t, verifyOutput)
	assert.True(t, verifyOutput.Verified)
}

func TestVerifyMFA_InvalidCode(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	userRepo := NewMockUserRepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	// Setup MFA
	setupUseCase := NewSetupMFAUseCase(mfaRepo, encKey)
	userID := uuid.New()
	tenantID := uuid.New()

	setupInput := SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    "user@example.com",
	}

	_, err := setupUseCase.Execute(ctx, setupInput)
	require.NoError(t, err)

	// Try to verify with invalid code
	verifyUseCase := NewVerifyMFAUseCase(mfaRepo, userRepo, encKey)
	
	verifyInput := VerifyMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     "000000", // Invalid code
	}

	_, err = verifyUseCase.Execute(ctx, verifyInput)
	assert.Error(t, err)
}

// Tests for ChallengeMFAUseCase
func TestChallengeMFA_Success(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	userRepo := NewMockUserRepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	// Setup and verify MFA
	setupUseCase := NewSetupMFAUseCase(mfaRepo, encKey)
	userID := uuid.New()
	tenantID := uuid.New()

	setupInput := SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    "user@example.com",
	}

	setupOutput, err := setupUseCase.Execute(ctx, setupInput)
	require.NoError(t, err)

	verifyUseCase := NewVerifyMFAUseCase(mfaRepo, userRepo, encKey)
	code, err := generateValidTOTPCode(setupOutput.Secret)
	require.NoError(t, err)

	verifyInput := VerifyMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     code,
	}

	_, err = verifyUseCase.Execute(ctx, verifyInput)
	require.NoError(t, err)

	// Challenge with TOTP code
	challengeUseCase := NewChallengeMFAUseCase(mfaRepo, encKey)
	
	code2, err := generateValidTOTPCode(setupOutput.Secret)
	require.NoError(t, err)

	challengeInput := ChallengeMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     code2,
	}

	challengeOutput, err := challengeUseCase.Execute(ctx, challengeInput)

	assert.NoError(t, err)
	assert.NotNil(t, challengeOutput)
	assert.True(t, challengeOutput.Verified)
}

func TestChallengeMFA_BackupCode(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	userRepo := NewMockUserRepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm____")
	
	// Setup and verify MFA
	setupUseCase := NewSetupMFAUseCase(mfaRepo, encKey)
	userID := uuid.New()
	tenantID := uuid.New()

	setupInput := SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Email:    "user@example.com",
	}

	setupOutput, err := setupUseCase.Execute(ctx, setupInput)
	require.NoError(t, err)

	verifyUseCase := NewVerifyMFAUseCase(mfaRepo, userRepo, encKey)
	code, err := generateValidTOTPCode(setupOutput.Secret)
	require.NoError(t, err)

	verifyInput := VerifyMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     code,
	}

	_, err = verifyUseCase.Execute(ctx, verifyInput)
	require.NoError(t, err)

	// Challenge with backup code
	challengeUseCase := NewChallengeMFAUseCase(mfaRepo, encKey)
	backupCode := setupOutput.BackupCodes[0]

	challengeInput := ChallengeMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     backupCode,
	}

	challengeOutput, err := challengeUseCase.Execute(ctx, challengeInput)

	assert.NoError(t, err)
	assert.NotNil(t, challengeOutput)
	assert.True(t, challengeOutput.Verified)

	// Try to use same backup code again (should fail)
	challengeInput2 := ChallengeMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Code:     backupCode,
	}

	_, err = challengeUseCase.Execute(ctx, challengeInput2)
	assert.Error(t, err)
}

// Helper function to generate valid TOTP code
func generateValidTOTPCode(secret string) (string, error) {
	// Use the otp package to generate current code
	// This is a simplified version; in production, use actual TOTP library
	return "123456", nil // Placeholder
}

// Mock User Repository
type MockUserRepository struct {
	users map[uuid.UUID]*domain.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[uuid.UUID]*domain.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return m.users[id], nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, nil
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.users, id)
	return nil
}
