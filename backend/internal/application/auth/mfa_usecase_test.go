// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
)

// Mock MFA Repository
type MockMFARepository struct {
	secrets        map[string]*domain.MFASecret
	codes          map[string][]*domain.MFABackupCode
	oauthProviders map[string]*domain.OAuthProvider
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
