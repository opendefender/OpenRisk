package services

import (
	"crypto/rand"
	"crypto/sha"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
)

const (
	TokenLength        =                   // Length of random token bytes
	TokenDisplayLength =                    // Length of token prefix to display
	TokenPrefix        = "orsk_"             // Prefix for OpenRisk tokens
	DefaultTokenExpiry =     time.Hour //  days default
)

// TokenService handles API token management
type TokenService struct {
	tokens map[string]domain.APIToken // tokenHash -> token
	mu     sync.RWMutex
}

// NewTokenService creates a new token service
func NewTokenService() TokenService {
	return &TokenService{
		tokens: make(map[string]domain.APIToken),
	}
}

// GenerateToken creates a new random token string
func (ts TokenService) GenerateToken() (string, error) {
	randomBytes := make([]byte, TokenLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	token := TokenPrefix + hex.EncodeToString(randomBytes)
	return token, nil
}

// HashToken creates a SHA hash of a token
func (ts TokenService) HashToken(token string) string {
	hash := sha.Sum([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetTokenPrefix extracts the display prefix from a token
func (ts TokenService) GetTokenPrefix(token string) string {
	if len(token) > TokenDisplayLength {
		return token[:TokenDisplayLength]
	}
	return token
}

// CreateToken creates a new API token for a user
func (ts TokenService) CreateToken(userID uuid.UUID, req domain.TokenCreateRequest, createdByID uuid.UUID) (domain.TokenWithValue, error) {
	// Generate the token
	tokenValue, err := ts.GenerateToken()
	if err != nil {
		return nil, err
	}

	// Hash the token for storage
	tokenHash := ts.HashToken(tokenValue)
	tokenPrefix := ts.GetTokenPrefix(tokenValue)

	// Set expiration if not provided
	expiresAt := req.ExpiresAt
	if expiresAt == nil && req.Type == domain.TokenTypeBearer {
		exp := time.Now().Add(DefaultTokenExpiry)
		expiresAt = &exp
	}

	// Create the token entity
	token := &domain.APIToken{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		Type:        req.Type,
		Status:      domain.TokenStatusActive,
		Permissions: req.Permissions,
		Scopes:      req.Scopes,
		IPWhitelist: req.IPWhitelist,
		Metadata:    req.Metadata,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		CreatedByID: createdByID,
	}

	// Store token
	ts.mu.Lock()
	ts.tokens[tokenHash] = token
	ts.mu.Unlock()

	// Return token with value (only shown once)
	return &domain.TokenWithValue{
		TokenResponse: ts.tokenToResponse(token),
		Token:         tokenValue,
	}, nil
}

// VerifyToken verifies a token string and returns the token entity if valid
func (ts TokenService) VerifyToken(tokenValue string) (domain.APIToken, error) {
	if !strings.HasPrefix(tokenValue, TokenPrefix) {
		return nil, fmt.Errorf("invalid token format")
	}

	tokenHash := ts.HashToken(tokenValue)

	ts.mu.RLock()
	token, exists := ts.tokens[tokenHash]
	ts.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("token not found")
	}

	// Check if token is valid
	if !token.IsValid() {
		status := "revoked"
		if token.IsExpired() {
			status = "expired"
		} else if token.Status == domain.TokenStatusDisabled {
			status = "disabled"
		}
		return nil, fmt.Errorf("token is %s", status)
	}

	// Update last used
	token.UpdateLastUsed()

	return token, nil
}

// GetToken retrieves a token by ID
func (ts TokenService) GetToken(tokenID uuid.UUID) (domain.APIToken, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	for _, token := range ts.tokens {
		if token.ID == tokenID {
			return token, nil
		}
	}

	return nil, fmt.Errorf("token not found")
}

// ListTokens retrieves all tokens for a user
func (ts TokenService) ListTokens(userID uuid.UUID) ([]domain.APIToken, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var userTokens []domain.APIToken
	for _, token := range ts.tokens {
		if token.UserID == userID {
			userTokens = append(userTokens, token)
		}
	}

	return userTokens, nil
}

// UpdateToken updates a token's properties
func (ts TokenService) UpdateToken(tokenID uuid.UUID, req domain.TokenUpdateRequest) (domain.APIToken, error) {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Update only provided fields
	if req.Name != "" {
		token.Name = req.Name
	}
	if req.Description != "" {
		token.Description = req.Description
	}
	if req.Permissions != nil {
		token.Permissions = req.Permissions
	}
	if req.Scopes != nil {
		token.Scopes = req.Scopes
	}
	if req.IPWhitelist != nil {
		token.IPWhitelist = req.IPWhitelist
	}
	if req.Metadata != nil {
		token.Metadata = req.Metadata
	}

	token.UpdatedAt = time.Now()

	return token, nil
}

// RevokeToken revokes a token
func (ts TokenService) RevokeToken(tokenID uuid.UUID, reason string) (domain.APIToken, error) {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	token.Revoke(reason)
	token.UpdatedAt = time.Now()

	return token, nil
}

// RotateToken revokes the old token and creates a new one
func (ts TokenService) RotateToken(tokenID uuid.UUID, userID uuid.UUID, reason string) (domain.RotateTokenResponse, error) {
	// Get old token
	oldToken, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	if oldToken.UserID != userID {
		return nil, fmt.Errorf("unauthorized: token does not belong to user")
	}

	// Revoke old token
	ts.mu.Lock()
	oldToken.Revoke(fmt.Sprintf("Rotated: %s", reason))
	oldToken.UpdatedAt = time.Now()
	ts.mu.Unlock()

	// Create new token with same properties
	createReq := &domain.TokenCreateRequest{
		Name:        oldToken.Name,
		Description: oldToken.Description,
		Type:        oldToken.Type,
		Permissions: oldToken.Permissions,
		Scopes:      oldToken.Scopes,
		IPWhitelist: oldToken.IPWhitelist,
		Metadata:    oldToken.Metadata,
	}

	// Set expiration if old token had one
	if oldToken.ExpiresAt != nil {
		// New token expires in same time period as old one
		duration := oldToken.ExpiresAt.Sub(oldToken.CreatedAt)
		newExpiry := time.Now().Add(duration)
		createReq.ExpiresAt = &newExpiry
	}

	newToken, err := ts.CreateToken(userID, createReq, oldToken.CreatedByID)
	if err != nil {
		return nil, err
	}

	return &domain.RotateTokenResponse{
		OldToken:  ts.tokenToResponse(oldToken),
		NewToken:  newToken,
		RotatedAt: time.Now(),
	}, nil
}

// DeleteToken permanently deletes a token (hard delete)
func (ts TokenService) DeleteToken(tokenID uuid.UUID) error {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	delete(ts.tokens, token.TokenHash)
	return nil
}

// DisableToken disables a token without revoking it
func (ts TokenService) DisableToken(tokenID uuid.UUID) (domain.APIToken, error) {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	token.Disable()
	token.UpdatedAt = time.Now()

	return token, nil
}

// EnableToken re-enables a disabled token
func (ts TokenService) EnableToken(tokenID uuid.UUID) (domain.APIToken, error) {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	token.Enable()
	token.UpdatedAt = time.Now()

	return token, nil
}

// CheckTokenExpiry marks expired tokens as such
func (ts TokenService) CheckTokenExpiry(tokenID uuid.UUID) (domain.APIToken, error) {
	token, err := ts.GetToken(tokenID)
	if err != nil {
		return nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	if token.IsExpired() && token.Status == domain.TokenStatusActive {
		token.Status = domain.TokenStatusExpired
	}

	return token, nil
}

// CleanupExpiredTokens removes expired tokens (maintenance task)
func (ts TokenService) CleanupExpiredTokens() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for hash, token := range ts.tokens {
		if token.IsExpired() {
			delete(ts.tokens, hash)
		}
	}

	return nil
}

// Helper function to convert token to response
func (ts TokenService) tokenToResponse(token domain.APIToken) domain.TokenResponse {
	return &domain.TokenResponse{
		ID:          token.ID,
		Name:        token.Name,
		Description: token.Description,
		TokenPrefix: token.TokenPrefix,
		Type:        token.Type,
		Status:      token.Status,
		Permissions: token.Permissions,
		LastUsed:    token.LastUsed,
		ExpiresAt:   token.ExpiresAt,
		RevokedAt:   token.RevokedAt,
		CreatedAt:   token.CreatedAt,
		UpdatedAt:   token.UpdatedAt,
		Scopes:      token.Scopes,
		Metadata:    token.Metadata,
	}
}
