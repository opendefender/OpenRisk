// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJWT_GenerateAccessToken tests JWT access token generation
func TestJWT_GenerateAccessToken(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userID := uuid.New()
	organizationID := uuid.New()

	tokenManager := &JWTManager{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  1 * time.Hour,
	}

	token, err := tokenManager.GenerateAccessToken(userID, organizationID, "admin")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token structure
	parts := len(token) // Should have 3 parts separated by dots
	assert.Equal(t, 3, parts)
}

// TestJWT_ValidateAccessToken tests JWT access token validation
func TestJWT_ValidateAccessToken(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userID := uuid.New()
	organizationID := uuid.New()

	tokenManager := &JWTManager{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  1 * time.Hour,
	}

	// Generate token
	token, err := tokenManager.GenerateAccessToken(userID, organizationID, "admin")
	require.NoError(t, err)

	// Validate token
	claims, err := tokenManager.ValidateAccessToken(token)

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, organizationID, claims.OrganizationID)
	assert.Equal(t, "admin", claims.Role)
}

// TestJWT_ValidateAccessToken_Expired tests expired token validation
func TestJWT_ValidateAccessToken_Expired(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userID := uuid.New()
	organizationID := uuid.New()

	tokenManager := &JWTManager{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  -1 * time.Hour, // Negative = already expired
	}

	// Generate token
	token, err := tokenManager.GenerateAccessToken(userID, organizationID, "admin")
	require.NoError(t, err)

	// Reset to normal expiration for validation
	tokenManager.ExpiresIn = 1 * time.Hour

	// Attempt to validate expired token
	_, err = tokenManager.ValidateAccessToken(token)

	assert.Error(t, err)
}

// TestJWT_ValidateAccessToken_InvalidSignature tests invalid signature handling
func TestJWT_ValidateAccessToken_InvalidSignature(t *testing.T) {
	// Generate two different RSA key pairs
	privateKey1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKey2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userID := uuid.New()
	organizationID := uuid.New()

	// Generate token with first key
	tokenManager1 := &JWTManager{
		PrivateKey: privateKey1,
		PublicKey:  &privateKey1.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  1 * time.Hour,
	}

	token, err := tokenManager1.GenerateAccessToken(userID, organizationID, "admin")
	require.NoError(t, err)

	// Try to validate with second key
	tokenManager2 := &JWTManager{
		PrivateKey: privateKey2,
		PublicKey:  &privateKey2.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  1 * time.Hour,
	}

	_, err = tokenManager2.ValidateAccessToken(token)

	assert.Error(t, err)
}

// TestClaims_IsExpired tests expiration check
func TestClaims_IsExpired(t *testing.T) {
	claims := &Claims{
		UserID:         uuid.New(),
		OrganizationID: uuid.New(),
		Role:           "admin",
		ExpiresAt:      time.Now().Add(-1 * time.Hour), // Expired
	}

	assert.True(t, claims.IsExpired())

	claims.ExpiresAt = time.Now().Add(1 * time.Hour) // Not expired
	assert.False(t, claims.IsExpired())
}

// TestClaims_MarshalJSON tests claims JSON marshalling
func TestClaims_MarshalJSON(t *testing.T) {
	userID := uuid.New()
	organizationID := uuid.New()

	claims := &Claims{
		UserID:         userID,
		OrganizationID: organizationID,
		Role:           "admin",
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	data, err := json.Marshal(claims)

	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal and verify
	var unmarshalled Claims
	err = json.Unmarshal(data, &unmarshalled)

	assert.NoError(t, err)
	assert.Equal(t, userID, unmarshalled.UserID)
	assert.Equal(t, organizationID, unmarshalled.OrganizationID)
	assert.Equal(t, "admin", unmarshalled.Role)
}

// TestTokenPair tests token pair generation
func TestTokenPair_Generation(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userID := uuid.New()
	organizationID := uuid.New()

	tokenManager := &JWTManager{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		Issuer:     "openrisk",
		ExpiresIn:  1 * time.Hour,
	}

	accessToken, err := tokenManager.GenerateAccessToken(userID, organizationID, "admin")
	require.NoError(t, err)

	refreshToken := "refresh_token_123"

	tokenPair := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}

	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.Equal(t, refreshToken, tokenPair.RefreshToken)
	assert.Equal(t, 3600, tokenPair.ExpiresIn)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
}

// TestArgon2idPasswordHasher_Hash tests Argon2id password hashing
func TestArgon2idPasswordHasher_Hash(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()

	password := "mysecurepassword123"
	hash, err := hasher.Hash(password)

	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
	// Verify it's in Argon2id format
	assert.Contains(t, hash, "$argon2id$")
}

// TestArgon2idPasswordHasher_Verify tests Argon2id password verification
func TestArgon2idPasswordHasher_Verify(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()

	password := "mysecurepassword123"
	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	// Verify correct password
	assert.True(t, hasher.Verify(hash, password))

	// Verify incorrect password
	assert.False(t, hasher.Verify(hash, "wrongpassword"))
}

// TestArgon2idPasswordHasher_VerifyWithDifferentHash tests verification with different hash
func TestArgon2idPasswordHasher_VerifyWithDifferentHash(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()

	password := "mysecurepassword123"
	_, err := hasher.Hash(password)
	require.NoError(t, err)

	// Verify with invalid Argon2id hash should fail
	assert.False(t, hasher.Verify("$argon2id$invalid$hash", password))
}

// TestArgon2idPasswordHasher_MultipleHashes tests that different hashes are generated for same password
func TestArgon2idPasswordHasher_MultipleHashes(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()

	password := "mysecurepassword123"
	hash1, err1 := hasher.Hash(password)
	hash2, err2 := hasher.Hash(password)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Hashes should be different (due to random salt)
	assert.NotEqual(t, hash1, hash2)

	// But both should verify the same password
	assert.True(t, hasher.Verify(hash1, password))
	assert.True(t, hasher.Verify(hash2, password))
}
