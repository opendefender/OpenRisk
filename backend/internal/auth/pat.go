// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

// PATValuePrefix opens every token value this service mints, so a leaked token
// is recognisable on sight and by secret scanners: "orsk_<8-hex>_<64-hex>".
// Tokens minted before #782 have no such prefix and still validate.
const PATValuePrefix = "orsk_"

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
func (s *PersonalAccessTokenService) generateTokenPrefix() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:8], nil
}

// CreateToken creates a personal access token for userID in tenantID. The token
// can only ever act in that tenant. The raw value is returned once and is never
// stored; only its SHA-256 hash is.
func (s *PersonalAccessTokenService) CreateToken(ctx context.Context, userID, tenantID uuid.UUID, name, description string, scopes []string, expiresAt *time.Time) (*domain.PersonalAccessToken, string, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, "", fmt.Errorf("%w: token owner and tenant are required", domain.ErrValidation)
	}
	// Generate secure token
	token, err := s.generateSecureToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create token hash
	tokenHash := s.generateTokenHash(token)
	tokenPrefix, err := s.generateTokenPrefix()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token prefix: %w", err)
	}

	// Create PAT entity
	encodedScopes, err := json.Marshal(scopes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode scopes: %w", err)
	}

	pat := &domain.PersonalAccessToken{
		UserID:      userID,
		TenantID:    tenantID,
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

	return pat, PATValuePrefix + tokenPrefix + "_" + token, nil
}

// SplitPATValue parses "orsk_<8-hex>_<secret>" or the pre-#782 form
// "<8-hex>_<secret>" into its lookup prefix and secret. ok is false for
// anything else, including a JWT.
func SplitPATValue(raw string) (prefix, secret string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(raw, PATValuePrefix), "_")
	if len(parts) != 2 || len(parts[0]) != 8 || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ValidateToken validates a token and returns the PAT if valid
func (s *PersonalAccessTokenService) ValidateToken(ctx context.Context, token string) (*domain.PersonalAccessToken, error) {
	prefix, tokenValue, ok := SplitPATValue(token)
	if !ok {
		return nil, fmt.Errorf("%w: invalid token format", domain.ErrUnauthorized)
	}

	// Hash the token value
	tokenHash := s.generateTokenHash(tokenValue)

	// Find token by hash
	pat, err := s.repo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown token", domain.ErrUnauthorized)
	}

	// Check if token belongs to the correct prefix
	if pat.TokenPrefix != prefix {
		return nil, fmt.Errorf("%w: invalid token", domain.ErrUnauthorized)
	}

	// Check if token is expired
	if pat.IsExpired() {
		return nil, fmt.Errorf("%w: token expired", domain.ErrUnauthorized)
	}

	// A row from before #782 that the backfill could not attribute is unusable.
	if pat.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: token has no tenant", domain.ErrUnauthorized)
	}

	// Update last used timestamp
	if err := s.repo.UpdateLastUsed(ctx, pat.TenantID, pat.ID); err != nil {
		// Log error but don't fail validation
		fmt.Printf("Failed to update last used timestamp: %v\n", err)
	}

	return pat, nil
}

// GetScopes returns the token's granted scopes (empty slice if none/undecodable).
func (s *PersonalAccessTokenService) GetScopes(pat *domain.PersonalAccessToken) []string {
	var scopes []string
	if len(pat.Scopes) == 0 {
		return scopes
	}
	if err := json.Unmarshal(pat.Scopes, &scopes); err != nil {
		return nil
	}
	return scopes
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

// ListUserTokens lists the tokens userID minted in tenantID. Tokens the same
// person minted in another organization are not shown.
func (s *PersonalAccessTokenService) ListUserTokens(ctx context.Context, tenantID, userID uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	return s.repo.ListByOwner(ctx, tenantID, userID)
}

// RevokeToken deletes a token userID owns in tenantID. Someone else's token, or
// the caller's own token from another organization, is ErrNotFound: whether it
// exists is not the caller's business.
func (s *PersonalAccessTokenService) RevokeToken(ctx context.Context, tenantID, tokenID, userID uuid.UUID) error {
	removed, err := s.repo.DeleteByOwner(ctx, tenantID, userID, tokenID)
	if err != nil {
		return err
	}
	if !removed {
		return domain.ErrNotFound
	}
	return nil
}
