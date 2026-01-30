package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t testing.T) {
	ts := NewTokenService()

	token, err := ts.GenerateToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, len(token) > )
	assert.Equal(t, TokenPrefix, token[:len(TokenPrefix)])
}

func TestHashToken(t testing.T) {
	ts := NewTokenService()

	token := "test_token_value"
	hash := ts.HashToken(token)
	hash := ts.HashToken(token)

	assert.Equal(t, hash, hash, "same token should produce same hash")
	assert.NotEmpty(t, hash)
}

func TestGetTokenPrefix(t testing.T) {
	ts := NewTokenService()

	token := TokenPrefix + "abcdefghijklmnop"
	prefix := ts.GetTokenPrefix(token)

	assert.Equal(t, TokenPrefix+"ab", prefix[:len(TokenPrefix)+])
}

func TestCreateToken_Success(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name:        "Test Token",
		Description: "For testing",
		Type:        domain.TokenTypeBearer,
		Permissions: []string{"read", "write"},
		Scopes:      []string{"risk"},
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)
	assert.NotNil(t, tokenWithValue)
	assert.Equal(t, "Test Token", tokenWithValue.Name)
	assert.Equal(t, domain.TokenStatusActive, tokenWithValue.Status)
	assert.NotEmpty(t, tokenWithValue.Token)
	assert.Equal(t, TokenPrefix, tokenWithValue.TokenPrefix[:len(TokenPrefix)])
}

func TestCreateToken_WithExpiry(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	expiryTime := time.Now().Add(  time.Hour)
	req := &domain.TokenCreateRequest{
		Name:      "Test Token",
		Type:      domain.TokenTypeBearer,
		ExpiresAt: &expiryTime,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)
	assert.NotNil(t, tokenWithValue.ExpiresAt)
}

func TestVerifyToken_Valid(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Verify the token
	token, err := ts.VerifyToken(tokenWithValue.Token)
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, userID, token.UserID)
	assert.Equal(t, domain.TokenStatusActive, token.Status)
}

func TestVerifyToken_InvalidFormat(t testing.T) {
	ts := NewTokenService()

	_, err := ts.VerifyToken("invalid_token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token format")
}

func TestVerifyToken_NotFound(t testing.T) {
	ts := NewTokenService()

	_, err := ts.VerifyToken(TokenPrefix + "nonexistenttoken")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token not found")
}

func TestVerifyToken_UpdatesLastUsed(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	time.Sleep(  time.Millisecond)

	token, err := ts.VerifyToken(tokenWithValue.Token)
	require.NoError(t, err)
	assert.NotNil(t, token.LastUsed)
}

func TestGetToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Retrieve token by ID
	token, err := ts.GetToken(tokenWithValue.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Token", token.Name)
}

func TestListTokens(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	otherUserID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "User Token",
		Type: domain.TokenTypeBearer,
	}

	// Create tokens for different users
	_, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	_, err = ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	_, err = ts.CreateToken(otherUserID, req, createdByID)
	require.NoError(t, err)

	// List tokens for first user
	tokens, err := ts.ListTokens(userID)
	require.NoError(t, err)
	assert.Equal(t, , len(tokens))

	// List tokens for second user
	tokens, err = ts.ListTokens(otherUserID)
	require.NoError(t, err)
	assert.Equal(t, , len(tokens))
}

func TestUpdateToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Original Name",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Update token
	updateReq := &domain.TokenUpdateRequest{
		Name:        "Updated Name",
		Description: "New description",
	}

	updated, err := ts.UpdateToken(tokenWithValue.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "New description", updated.Description)
}

func TestRevokeToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Revoke token
	revoked, err := ts.RevokeToken(tokenWithValue.ID, "Security concerns")
	require.NoError(t, err)
	assert.Equal(t, domain.TokenStatusRevoked, revoked.Status)
	assert.NotNil(t, revoked.RevokedAt)
	assert.Equal(t, "Security concerns", revoked.RevokeReason)

	// Verification should fail
	_, err = ts.VerifyToken(tokenWithValue.Token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRotateToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Original Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Rotate token
	rotateResp, err := ts.RotateToken(tokenWithValue.ID, userID, "Regular rotation")
	require.NoError(t, err)
	assert.NotNil(t, rotateResp.OldToken)
	assert.NotNil(t, rotateResp.NewToken)
	assert.Equal(t, domain.TokenStatusRevoked, rotateResp.OldToken.Status)
	assert.Equal(t, domain.TokenStatusActive, rotateResp.NewToken.Status)
}

func TestDeleteToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Delete token
	err = ts.DeleteToken(tokenWithValue.ID)
	require.NoError(t, err)

	// Token should not exist
	_, err = ts.GetToken(tokenWithValue.ID)
	assert.Error(t, err)
}

func TestDisableToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Disable token
	disabled, err := ts.DisableToken(tokenWithValue.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TokenStatusDisabled, disabled.Status)

	// Verification should fail
	_, err = ts.VerifyToken(tokenWithValue.Token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestEnableToken(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	req := &domain.TokenCreateRequest{
		Name: "Test Token",
		Type: domain.TokenTypeBearer,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Disable then enable
	ts.DisableToken(tokenWithValue.ID)
	enabled, err := ts.EnableToken(tokenWithValue.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TokenStatusActive, enabled.Status)

	// Verification should succeed
	_, err = ts.VerifyToken(tokenWithValue.Token)
	require.NoError(t, err)
}

func TestCheckTokenExpiry(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	// Create token that expires in the past
	pastTime := time.Now().Add(-  time.Hour)
	req := &domain.TokenCreateRequest{
		Name:      "Expired Token",
		Type:      domain.TokenTypeBearer,
		ExpiresAt: &pastTime,
	}

	tokenWithValue, err := ts.CreateToken(userID, req, createdByID)
	require.NoError(t, err)

	// Check expiry
	checked, err := ts.CheckTokenExpiry(tokenWithValue.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TokenStatusExpired, checked.Status)
}

func TestCleanupExpiredTokens(t testing.T) {
	ts := NewTokenService()
	userID := uuid.New()
	createdByID := uuid.New()

	// Create an expired token
	pastTime := time.Now().Add(-  time.Hour)
	expiredReq := &domain.TokenCreateRequest{
		Name:      "Expired Token",
		Type:      domain.TokenTypeBearer,
		ExpiresAt: &pastTime,
	}

	_, err := ts.CreateToken(userID, expiredReq, createdByID)
	require.NoError(t, err)

	// Create a valid token
	validReq := &domain.TokenCreateRequest{
		Name: "Valid Token",
		Type: domain.TokenTypeBearer,
	}

	validToken, err := ts.CreateToken(userID, validReq, createdByID)
	require.NoError(t, err)

	// Cleanup
	err = ts.CleanupExpiredTokens()
	require.NoError(t, err)

	// Expired token should be deleted
	_, err = ts.GetToken(validToken.ID)
	require.NoError(t, err) // Valid token should still exist
}

func TestAPITokenIsExpired(t testing.T) {
	pastTime := time.Now().Add(-  time.Hour)
	futureTime := time.Now().Add(  time.Hour)

	expiredToken := &domain.APIToken{
		ExpiresAt: &pastTime,
	}

	validToken := &domain.APIToken{
		ExpiresAt: &futureTime,
	}

	noExpiryToken := &domain.APIToken{
		ExpiresAt: nil,
	}

	assert.True(t, expiredToken.IsExpired())
	assert.False(t, validToken.IsExpired())
	assert.False(t, noExpiryToken.IsExpired())
}

func TestAPITokenIsValid(t testing.T) {
	futureTime := time.Now().Add(  time.Hour)

	tests := []struct {
		name  string
		token domain.APIToken
		valid bool
	}{
		{
			name: "active and not expired",
			token: &domain.APIToken{
				Status:    domain.TokenStatusActive,
				ExpiresAt: &futureTime,
			},
			valid: true,
		},
		{
			name: "revoked token",
			token: &domain.APIToken{
				Status:    domain.TokenStatusRevoked,
				ExpiresAt: &futureTime,
			},
			valid: false,
		},
		{
			name: "disabled token",
			token: &domain.APIToken{
				Status:    domain.TokenStatusDisabled,
				ExpiresAt: &futureTime,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.valid, tt.token.IsValid())
		})
	}
}

func TestAPITokenHasPermission(t testing.T) {
	tests := []struct {
		name       string
		token      domain.APIToken
		permission string
		has        bool
	}{
		{
			name:       "no specific permissions - inherits role",
			token:      &domain.APIToken{Permissions: []string{}},
			permission: "read",
			has:        true,
		},
		{
			name:       "has specific permission",
			token:      &domain.APIToken{Permissions: []string{"read", "write"}},
			permission: "read",
			has:        true,
		},
		{
			name:       "missing permission",
			token:      &domain.APIToken{Permissions: []string{"read"}},
			permission: "delete",
			has:        false,
		},
		{
			name:       "wildcard permission",
			token:      &domain.APIToken{Permissions: []string{""}},
			permission: "anything",
			has:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.has, tt.token.HasPermission(tt.permission))
		})
	}
}

func TestAPITokenHasScope(t testing.T) {
	tests := []struct {
		name  string
		token domain.APIToken
		scope string
		has   bool
	}{
		{
			name:  "no scope restrictions",
			token: &domain.APIToken{Scopes: []string{}},
			scope: "any",
			has:   true,
		},
		{
			name:  "has specific scope",
			token: &domain.APIToken{Scopes: []string{"risk", "mitigation"}},
			scope: "risk",
			has:   true,
		},
		{
			name:  "missing scope",
			token: &domain.APIToken{Scopes: []string{"risk"}},
			scope: "user",
			has:   false,
		},
		{
			name:  "wildcard scope",
			token: &domain.APIToken{Scopes: []string{""}},
			scope: "anything",
			has:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.has, tt.token.HasScope(tt.scope))
		})
	}
}

func TestAPITokenIsIPAllowed(t testing.T) {
	tests := []struct {
		name     string
		token    domain.APIToken
		clientIP string
		allowed  bool
	}{
		{
			name:     "no IP restrictions",
			token:    &domain.APIToken{IPWhitelist: []string{}},
			clientIP: "...",
			allowed:  true,
		},
		{
			name:     "IP in whitelist",
			token:    &domain.APIToken{IPWhitelist: []string{"...", "..."}},
			clientIP: "...",
			allowed:  true,
		},
		{
			name:     "IP not in whitelist",
			token:    &domain.APIToken{IPWhitelist: []string{"..."}},
			clientIP: "...",
			allowed:  false,
		},
		{
			name:     "wildcard IP",
			token:    &domain.APIToken{IPWhitelist: []string{""}},
			clientIP: "any.ip",
			allowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.allowed, tt.token.IsIPAllowed(tt.clientIP))
		})
	}
}
