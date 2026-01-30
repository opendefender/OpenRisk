package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPITokenIsExpired(t testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "no expiration",
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "expired in the past",
			expiresAt: ptrTime(time.Now().Add(-  time.Hour)),
			expected:  true,
		},
		{
			name:      "expires in the future",
			expiresAt: ptrTime(time.Now().Add(  time.Hour)),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			token := &APIToken{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.expected, token.IsExpired())
		})
	}
}

func TestAPITokenIsRevoked(t testing.T) {
	tests := []struct {
		name      string
		status    TokenStatus
		revokedAt time.Time
		expected  bool
	}{
		{
			name:      "active token",
			status:    TokenStatusActive,
			revokedAt: nil,
			expected:  false,
		},
		{
			name:      "revoked status",
			status:    TokenStatusRevoked,
			revokedAt: ptrTime(time.Now()),
			expected:  true,
		},
		{
			name:      "revoked with nil revokedAt",
			status:    TokenStatusRevoked,
			revokedAt: nil,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			token := &APIToken{Status: tt.status, RevokedAt: tt.revokedAt}
			assert.Equal(t, tt.expected, token.IsRevoked())
		})
	}
}

func TestAPITokenIsValid(t testing.T) {
	tests := []struct {
		name     string
		token    APIToken
		expected bool
	}{
		{
			name: "active and not expired",
			token: &APIToken{
				Status:    TokenStatusActive,
				ExpiresAt: ptrTime(time.Now().Add(  time.Hour)),
			},
			expected: true,
		},
		{
			name: "expired",
			token: &APIToken{
				Status:    TokenStatusActive,
				ExpiresAt: ptrTime(time.Now().Add(-  time.Hour)),
			},
			expected: false,
		},
		{
			name: "revoked",
			token: &APIToken{
				Status:    TokenStatusRevoked,
				ExpiresAt: ptrTime(time.Now().Add(  time.Hour)),
				RevokedAt: ptrTime(time.Now()),
			},
			expected: false,
		},
		{
			name: "disabled",
			token: &APIToken{
				Status:    TokenStatusDisabled,
				ExpiresAt: ptrTime(time.Now().Add(  time.Hour)),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.expected, tt.token.IsValid())
		})
	}
}

func TestAPITokenUpdateLastUsed(t testing.T) {
	token := &APIToken{
		LastUsed: nil,
	}

	token.UpdateLastUsed()

	assert.NotNil(t, token.LastUsed)
	assert.WithinDuration(t, time.Now(), token.LastUsed, time.Millisecond)
}

func TestAPITokenRevoke(t testing.T) {
	token := &APIToken{
		Status:    TokenStatusActive,
		RevokedAt: nil,
	}

	token.Revoke("Test revocation")

	assert.Equal(t, TokenStatusRevoked, token.Status)
	assert.NotNil(t, token.RevokedAt)
	assert.Equal(t, "Test revocation", token.RevokeReason)
}

func TestAPITokenDisable(t testing.T) {
	token := &APIToken{
		Status: TokenStatusActive,
	}

	token.Disable()

	assert.Equal(t, TokenStatusDisabled, token.Status)
}

func TestAPITokenEnable(t testing.T) {
	token := &APIToken{
		Status: TokenStatusDisabled,
	}

	token.Enable()

	assert.Equal(t, TokenStatusActive, token.Status)
}

func TestAPITokenHasPermission(t testing.T) {
	tests := []struct {
		name       string
		token      APIToken
		permission string
		expected   bool
	}{
		{
			name: "no specific permissions - inherits from role",
			token: &APIToken{
				Permissions: []string{},
			},
			permission: "read",
			expected:   true,
		},
		{
			name: "has exact permission",
			token: &APIToken{
				Permissions: []string{"read", "write", "delete"},
			},
			permission: "write",
			expected:   true,
		},
		{
			name: "missing permission",
			token: &APIToken{
				Permissions: []string{"read"},
			},
			permission: "delete",
			expected:   false,
		},
		{
			name: "wildcard permission",
			token: &APIToken{
				Permissions: []string{""},
			},
			permission: "anything",
			expected:   true,
		},
		{
			name: "nil permissions",
			token: &APIToken{
				Permissions: nil,
			},
			permission: "read",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.expected, tt.token.HasPermission(tt.permission))
		})
	}
}

func TestAPITokenHasScope(t testing.T) {
	tests := []struct {
		name     string
		token    APIToken
		scope    string
		expected bool
	}{
		{
			name: "no scope restrictions",
			token: &APIToken{
				Scopes: []string{},
			},
			scope:    "risk",
			expected: true,
		},
		{
			name: "has exact scope",
			token: &APIToken{
				Scopes: []string{"risk", "mitigation", "asset"},
			},
			scope:    "mitigation",
			expected: true,
		},
		{
			name: "missing scope",
			token: &APIToken{
				Scopes: []string{"risk"},
			},
			scope:    "user",
			expected: false,
		},
		{
			name: "wildcard scope",
			token: &APIToken{
				Scopes: []string{""},
			},
			scope:    "anything",
			expected: true,
		},
		{
			name: "nil scopes",
			token: &APIToken{
				Scopes: nil,
			},
			scope:    "any",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.expected, tt.token.HasScope(tt.scope))
		})
	}
}

func TestAPITokenIsIPAllowed(t testing.T) {
	tests := []struct {
		name     string
		token    APIToken
		clientIP string
		expected bool
	}{
		{
			name: "no IP restrictions",
			token: &APIToken{
				IPWhitelist: []string{},
			},
			clientIP: "...",
			expected: true,
		},
		{
			name: "IP in whitelist",
			token: &APIToken{
				IPWhitelist: []string{"...", "...", "..."},
			},
			clientIP: "...",
			expected: true,
		},
		{
			name: "IP not in whitelist",
			token: &APIToken{
				IPWhitelist: []string{"...", "..."},
			},
			clientIP: "...",
			expected: false,
		},
		{
			name: "wildcard IP",
			token: &APIToken{
				IPWhitelist: []string{""},
			},
			clientIP: "any.ip.address",
			expected: true,
		},
		{
			name: "nil IP whitelist",
			token: &APIToken{
				IPWhitelist: nil,
			},
			clientIP: "any.ip",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t testing.T) {
			assert.Equal(t, tt.expected, tt.token.IsIPAllowed(tt.clientIP))
		})
	}
}

func TestTokenCreateRequest_Valid(t testing.T) {
	req := &TokenCreateRequest{
		Name: "Test Token",
		Type: TokenTypeBearer,
	}

	assert.NotNil(t, req)
	assert.Equal(t, "Test Token", req.Name)
	assert.Equal(t, TokenTypeBearer, req.Type)
}

func TestTokenUpdateRequest_PartialUpdate(t testing.T) {
	req := &TokenUpdateRequest{
		Name: "Updated Name",
	}

	assert.Equal(t, "Updated Name", req.Name)
	assert.Empty(t, req.Description)
	assert.Nil(t, req.Permissions)
}

func TestTokenRevokeRequest(t testing.T) {
	req := &TokenRevokeRequest{
		Reason: "Security issue",
	}

	assert.Equal(t, "Security issue", req.Reason)
}

func TestTokenResponse_Conversion(t testing.T) {
	now := time.Now()
	token := &APIToken{
		ID:          uuid.New(),
		Name:        "Test Token",
		Description: "Test Description",
		TokenPrefix: "orsk_abc",
		Type:        TokenTypeBearer,
		Status:      TokenStatusActive,
		ExpiresAt:   &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	response := &TokenResponse{
		ID:          token.ID,
		Name:        token.Name,
		Description: token.Description,
		TokenPrefix: token.TokenPrefix,
		Type:        token.Type,
		Status:      token.Status,
		ExpiresAt:   token.ExpiresAt,
		CreatedAt:   token.CreatedAt,
		UpdatedAt:   token.UpdatedAt,
	}

	assert.Equal(t, token.ID, response.ID)
	assert.Equal(t, token.Name, response.Name)
	assert.Equal(t, token.Status, response.Status)
}

func TestTokenWithValue_SecureTokenDisplay(t testing.T) {
	tokenValue := "orsk_verysecrettokenvalue"
	response := &TokenWithValue{
		TokenResponse: &TokenResponse{
			TokenPrefix: "orsk_ve",
		},
		Token: tokenValue,
	}

	// Token should only be visible in the Token field
	assert.Equal(t, tokenValue, response.Token)
	assert.NotEqual(t, tokenValue, response.TokenPrefix)
}

func TestRotateTokenResponse(t testing.T) {
	oldResponse := &TokenResponse{
		ID:     uuid.New(),
		Name:   "Old Token",
		Status: TokenStatusRevoked,
	}

	newResponse := &TokenWithValue{
		TokenResponse: &TokenResponse{
			ID:     uuid.New(),
			Name:   "Old Token",
			Status: TokenStatusActive,
		},
		Token: "orsk_newtoken",
	}

	rotateResp := &RotateTokenResponse{
		OldToken:  oldResponse,
		NewToken:  newResponse,
		RotatedAt: time.Now(),
	}

	assert.NotNil(t, rotateResp.OldToken)
	assert.NotNil(t, rotateResp.NewToken)
	assert.Equal(t, TokenStatusRevoked, rotateResp.OldToken.Status)
	assert.Equal(t, TokenStatusActive, rotateResp.NewToken.Status)
}

// Helper function
func ptrTime(t time.Time) time.Time {
	return &t
}
