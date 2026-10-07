// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// Mock MFA Repository
type MockMFARepository struct {
	secrets        map[string]*domain.MFASecret
	codes          map[string][]*domain.MFABackupCode
	oauthProviders map[string]*domain.OAuthProvider
	// enrolmentErr makes StartMFAEnrolment fail; enrolments counts its calls.
	enrolmentErr error
	enrolments   int
}

func NewMockMFARepository() *MockMFARepository {
	return &MockMFARepository{
		secrets:        make(map[string]*domain.MFASecret),
		codes:          make(map[string][]*domain.MFABackupCode),
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

func (m *MockMFARepository) ConsumeTOTPStep(ctx context.Context, userID, tenantID uuid.UUID, step int64) (bool, error) {
	secret := m.secrets[userID.String()+":"+tenantID.String()]
	if secret == nil || (secret.LastTOTPStep != nil && *secret.LastTOTPStep >= step) {
		return false, nil
	}
	secret.LastTOTPStep = &step
	return true, nil
}

// StartMFAEnrolment honours what the GORM version guarantees: user_id is
// unique across tenants, a verified secret is never replaced, and the secret
// and codes are written together or not at all.
func (m *MockMFARepository) StartMFAEnrolment(ctx context.Context, userID, tenantID uuid.UUID, secretEncrypted string, codes []*domain.MFABackupCode) (bool, error) {
	m.enrolments++
	if m.enrolmentErr != nil {
		return false, m.enrolmentErr
	}
	key := userID.String() + ":" + tenantID.String()
	for k, s := range m.secrets {
		if s.UserID == userID && (k != key || s.IsVerified) {
			return false, nil
		}
	}
	if s, ok := m.secrets[key]; ok {
		s.SecretEncrypted = secretEncrypted
		s.LastTOTPStep = nil
		s.LastUsedAt = nil
	} else {
		m.secrets[key] = &domain.MFASecret{ID: uuid.New(), UserID: userID, TenantID: tenantID, SecretEncrypted: secretEncrypted}
	}
	m.codes[key] = codes
	return true, nil
}

func (m *MockMFARepository) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error {
	key := userID.String() + ":" + tenantID.String()
	delete(m.secrets, key)
	delete(m.codes, key)
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
	encKey := []byte("32-byte-key-for-aes-256-gcm_____")

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

	// The secret and the codes go to the store in one call (#714), each code
	// hashed and scoped to the caller.
	assert.Equal(t, 1, mfaRepo.enrolments)
	stored := mfaRepo.codes[userID.String()+":"+tenantID.String()]
	assert.Len(t, stored, 8)
	for i, c := range stored {
		assert.Equal(t, userID, c.UserID)
		assert.Equal(t, tenantID, c.TenantID)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(c.CodeHash), []byte(output.BackupCodes[i])))
	}
}

// No secret yet for this user and tenant: setup creates one rather than
// failing on the missing row.
func TestSetupMFA_NotFound(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	userID, tenantID := uuid.New(), uuid.New()

	_, err := NewSetupMFAUseCase(mfaRepo, make([]byte, 32)).
		Execute(ctx, SetupMFAInput{UserID: userID, TenantID: tenantID, Email: "user@example.com"})

	assert.NoError(t, err)
	secret, _ := mfaRepo.GetMFASecret(ctx, userID, tenantID)
	if assert.NotNil(t, secret) {
		assert.False(t, secret.IsVerified, "a new secret awaits verification")
	}
}

// Without a session identity nothing is generated or written.
func TestSetupMFA_Unauthorized(t *testing.T) {
	for name, in := range map[string]SetupMFAInput{
		"no user":   {TenantID: uuid.New(), Email: "user@example.com"},
		"no tenant": {UserID: uuid.New(), Email: "user@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			mfaRepo := NewMockMFARepository()
			_, err := NewSetupMFAUseCase(mfaRepo, make([]byte, 32)).Execute(context.Background(), in)

			var appErr *domain.AppError
			assert.True(t, errors.As(err, &appErr) && errors.Is(appErr.Err, domain.ErrValidation), "got %v", err)
			assert.Zero(t, mfaRepo.enrolments)
			assert.Empty(t, mfaRepo.secrets)
		})
	}
}

// A store failure is a server fault, not a conflict or a validation error:
// it stays untyped so the handler answers 500 without its text (#714).
func TestSetupMFA_StoreFailureIsNotAConflict(t *testing.T) {
	mfaRepo := NewMockMFARepository()
	mfaRepo.enrolmentErr = errors.New(`ERROR: duplicate key value violates unique constraint "idx_mfa_secrets_user_id"`)

	out, err := NewSetupMFAUseCase(mfaRepo, make([]byte, 32)).
		Execute(context.Background(), SetupMFAInput{UserID: uuid.New(), TenantID: uuid.New(), Email: "user@example.com"})

	assert.Nil(t, out)
	assert.ErrorIs(t, err, mfaRepo.enrolmentErr)
	var appErr *domain.AppError
	assert.False(t, errors.As(err, &appErr), "a store failure must not be a typed error, got %v", err)
}

func TestSetupMFA_InvalidInput(t *testing.T) {
	ctx := context.Background()
	mfaRepo := NewMockMFARepository()
	encKey := []byte("32-byte-key-for-aes-256-gcm_____")

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

// VerifyMFAUseCase and ChallengeMFAUseCase tests were removed here: both
// require NewVerifyMFAUseCase(mfaRepo, userRepo, encKey), and userRepo must be
// a concrete repository.GormUserRepository, not an interface - so it can't be
// mocked without either a real test DB or a production interface-extraction
// refactor. Deferred to a dedicated tests phase, per explicit decision not to
// touch mfa_usecase.go's production signature to accommodate these tests.
