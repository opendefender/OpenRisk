// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

func testRegistry() *scanpkg.Registry {
	r := scanpkg.NewRegistry()
	r.Register(scanpkg.NewAWSScanner(nil))
	r.Register(scanpkg.NewAzureScanner(nil))
	r.Register(scanpkg.NewGCPScanner(nil))
	r.Register(scanpkg.NewNmapScanner())
	r.Register(scanpkg.NewAgentScanner())
	return r
}

func TestCreateScanConfig_Success_Agent(t *testing.T) {
	var saved *domain.ScanConfig
	repo := &mockConfigRepo{createFunc: func(_ context.Context, c *domain.ScanConfig) error { saved = c; return nil }}
	uc := NewCreateScanConfigUseCase(repo, testRegistry(), testCipher())

	cfg, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), CreateScanConfigInput{
		Name: "LAN sweep", Provider: domain.ProviderAgent, Targets: []string{"10.0.0.0/24"},
	})
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "LAN sweep", cfg.Name)
	assert.Empty(t, cfg.EncryptedCredentials) // never leaked back
}

func TestCreateScanConfig_Success_CloudEncryptsCreds(t *testing.T) {
	// Capture the stored ciphertext at Create time — the use case strips the
	// in-memory struct's creds afterward (a real GORM repo persists a copy first).
	var savedEnc string
	repo := &mockConfigRepo{createFunc: func(_ context.Context, c *domain.ScanConfig) error {
		savedEnc = c.EncryptedCredentials
		return nil
	}}
	uc := NewCreateScanConfigUseCase(repo, testRegistry(), testCipher())

	cfg, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), CreateScanConfigInput{
		Name:     "prod aws",
		Provider: domain.ProviderAWS,
		Credentials: map[string]string{
			"access_key_id": "AKIA", "secret_access_key": "secret",
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, savedEnc)              // stored encrypted
	assert.NotContains(t, savedEnc, "secret") // not plaintext
	assert.Empty(t, cfg.EncryptedCredentials) // stripped from response
}

func TestCreateScanConfig_Unauthorized(t *testing.T) {
	uc := NewCreateScanConfigUseCase(&mockConfigRepo{}, testRegistry(), testCipher())
	_, err := uc.Execute(context.Background(), uuid.Nil, uuid.New(), CreateScanConfigInput{
		Name: "x", Provider: domain.ProviderAgent, Targets: []string{"10.0.0.0/24"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestCreateScanConfig_Validation(t *testing.T) {
	uc := NewCreateScanConfigUseCase(&mockConfigRepo{}, testRegistry(), testCipher())
	cases := []struct {
		name string
		in   CreateScanConfigInput
	}{
		{"bad provider", CreateScanConfigInput{Name: "x", Provider: "foo"}},
		{"missing name", CreateScanConfigInput{Provider: domain.ProviderAgent, Targets: []string{"10.0.0.0/24"}}},
		{"cloud missing creds", CreateScanConfigInput{Name: "x", Provider: domain.ProviderAWS}},
		{"agent target too wide", CreateScanConfigInput{Name: "x", Provider: domain.ProviderNmap, Targets: []string{"10.0.0.0/8"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), tc.in)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrValidation)
		})
	}
}
