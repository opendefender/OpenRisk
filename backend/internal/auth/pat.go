// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// PersonalAccessTokenService handles PAT operations
type PersonalAccessTokenService struct {
	repo repository.PersonalAccessTokenRepository
}

// NewPersonalAccessTokenService creates a new PAT service
func NewPersonalAccessTokenService(repo repository.PersonalAccessTokenRepository) *PersonalAccessTokenService {
	return &PersonalAccessTokenService{repo: repo}
}

// generateSecureToken generates a cryptographically secure random token
func (s *PersonalAccessTokenService) generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateTokenHash creates a SHA256 hash of the token
func (s *PersonalAccessTokenService) generateTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateTokenPrefix creates a short prefix for token identification
func (s *PersonalAccessTokenService) generateTokenPrefix() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:8]
}

// CreateToken creates a new personal access token
func (s *PersonalAccessTokenService) CreateToken(ctx context.Context, userID uuid.UUID, name, description string, scopes []string, expiresAt *time.Time) (*domain.PersonalAccessToken, string, error) {
	// Generate secure token
	token, err := s.generateSecureToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create token hash
	tokenHash := s.generateTokenHash(token)
	tokenPrefix := s.generateTokenPrefix()

	// Create PAT entity
	encodedScopes, err := json.Marshal(scopes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode scopes: %w", err)
	}

	pat := &domain.PersonalAccessToken{
		UserID:      userID,
		Name:        name,
		Description: description,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		Scopes:      encodedScopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to repository
	if err := s.repo.Create(ctx, pat); err != nil {
		return nil, "", fmt.Errorf("failed to save token: %w", err)
	}

	return pat, token, nil
}

// ValidateToken validates a token and returns the PAT if valid
func (s *PersonalAccessTokenService) ValidateToken(ctx context.Context, token string) (*domain.PersonalAccessToken, error) {
	// Extract prefix and validate format
	parts := strings.Split(token, "_")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	prefix := parts[0]
	tokenValue := parts[1]

	if len(prefix) != 8 {
		return nil, fmt.Errorf("invalid token prefix")
	}

	// Hash the token value
	tokenHash := s.generateTokenHash(tokenValue)

	// Find token by hash
	pat, err := s.repo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("token not found")
	}

	// Check if token belongs to the correct prefix
	if pat.TokenPrefix != prefix {
		return nil, fmt.Errorf("invalid token")
	}

	// Check if token is expired
	if pat.IsExpired() {
		return nil, fmt.Errorf("token expired")
	}

	// Update last used timestamp
	if err := s.repo.UpdateLastUsed(ctx, pat.ID); err != nil {
		// Log error but don't fail validation
		fmt.Printf("Failed to update last used timestamp: %v\n", err)
	}

	return pat, nil
}

// HasScope checks if the token has a specific scope (supports wildcards)
func (s *PersonalAccessTokenService) HasScope(pat *domain.PersonalAccessToken, requiredScope string) bool {
	var scopes []string
	if err := json.Unmarshal(pat.Scopes, &scopes); err != nil {
		return false
	}

	for _, scope := range scopes {
		if scope == requiredScope || scope == "*" {
			return true
		}
		// Support wildcard matching (e.g., "read:*" matches "read:users")
		if strings.HasSuffix(scope, "*") {
			prefix := strings.TrimSuffix(scope, "*")
			if strings.HasPrefix(requiredScope, prefix) {
				return true
			}
		}
	}
	return false
}

// ListUserTokens gets all tokens for a user
func (s *PersonalAccessTokenService) ListUserTokens(ctx context.Context, userID uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// RevokeToken revokes a token
func (s *PersonalAccessTokenService) RevokeToken(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID) error {
	// First verify the token belongs to the user
	pat, err := s.repo.GetByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("token not found")
	}

	if pat.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	return s.repo.Delete(ctx, tokenID)
}
