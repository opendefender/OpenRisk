// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

const (
	// AccessTokenTTL — L3: 15-minute access tokens.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL — L3: 30-day refresh tokens.
	RefreshTokenTTL = 30 * 24 * time.Hour
	// MFAChallengeTTL — window to complete an MFA challenge after password check.
	MFAChallengeTTL = 5 * time.Minute
)

// RefreshToken represents a refresh token stored in database
type RefreshToken struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	TenantID          uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	TokenHash         string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"token_hash"` // SHA256 hash
	DeviceFingerprint string     `gorm:"type:varchar(255)" json:"device_fingerprint"`
	ExpiresAt         time.Time  `gorm:"index;not null" json:"expires_at"`
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// IsExpired checks if the refresh token has expired
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// SessionClaims is the resolved set of claims a user carries in a fresh token.
type SessionClaims struct {
	TenantID     uuid.UUID
	OrgRoles     map[uuid.UUID]string
	Permissions  []string
	FeatureFlags []string
}

// SessionResolver re-derives a user's tenant, org roles and permissions at token
// mint time. It is the single source used by login, OAuth/SAML, and refresh so
// every path produces identical claims — and refresh always reflects the user's
// CURRENT permissions (revocations take effect on the next refresh, and no
// permissions are ever "lost" across a refresh).
type SessionResolver func(ctx context.Context, userID uuid.UUID) (*SessionClaims, error)

// TokenManager handles token operations
type TokenManager struct {
	db       *gorm.DB
	rsaKeys  *authpkg.RSAKeys
	resolver SessionResolver
}

// NewTokenManager creates a new token manager
func NewTokenManager(db *gorm.DB, rsaKeys *authpkg.RSAKeys) *TokenManager {
	return &TokenManager{db: db, rsaKeys: rsaKeys}
}

// SetSessionResolver wires the resolver used by refresh and IssueSession.
func (tm *TokenManager) SetSessionResolver(r SessionResolver) {
	tm.resolver = r
}

// GenerateTokenPair generates a new access (RS256, 15 min) + refresh (30 day) pair.
func (tm *TokenManager) GenerateTokenPair(ctx context.Context, userID, tenantID uuid.UUID, orgRoles map[uuid.UUID]string, permissions []string, featureFlags []string, deviceFingerprint string) (*TokenPair, error) {
	// Generate refresh token (opaque string) and hash it for storage.
	refreshTokenValue, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	tokenHash := hashToken(refreshTokenValue)

	refreshToken := &RefreshToken{
		UserID:            userID,
		TenantID:          tenantID,
		TokenHash:         tokenHash,
		DeviceFingerprint: deviceFingerprint,
		ExpiresAt:         time.Now().Add(RefreshTokenTTL),
	}
	if err := tm.db.WithContext(ctx).Create(refreshToken).Error; err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Generate access token via the SINGLE pkg/auth minter.
	accessToken, _, err := authpkg.GenerateAccessToken(tm.rsaKeys, userID, tenantID, orgRoles, permissions, featureFlags, AccessTokenTTL)
	if err != nil {
		// Clean up the refresh token if access token generation fails.
		tm.db.WithContext(ctx).Delete(refreshToken)
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return authpkg.NewTokenPair(accessToken, refreshTokenValue, int64(AccessTokenTTL.Seconds())), nil
}

// IssueSession resolves the user's current claims and issues a full token pair.
// Used by OAuth2/SAML (and available to any post-authentication flow) so their
// result is byte-for-byte identical to password login.
func (tm *TokenManager) IssueSession(ctx context.Context, userID uuid.UUID, deviceFingerprint string) (*TokenPair, error) {
	if tm.resolver == nil {
		return nil, fmt.Errorf("session resolver not configured")
	}
	sc, err := tm.resolver(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tm.GenerateTokenPair(ctx, userID, sc.TenantID, sc.OrgRoles, sc.Permissions, sc.FeatureFlags, deviceFingerprint)
}

// GenerateMFAChallengeToken issues a short-lived, permission-less RS256 token of
// type MFA_REQUIRED. It is the only credential accepted by /auth/mfa/challenge and
// carries NO refresh token — the full pair is only minted after the code is valid.
func (tm *TokenManager) GenerateMFAChallengeToken(userID, tenantID uuid.UUID) (string, error) {
	token, _, err := authpkg.GenerateTypedToken(tm.rsaKeys, userID, tenantID, nil, nil, nil, MFAChallengeTTL, authpkg.TokenTypeMFARequired)
	if err != nil {
		return "", fmt.Errorf("failed to generate MFA challenge token: %w", err)
	}
	return token, nil
}

// RefreshTokenPair rotates a refresh token: it validates and then DELETES the
// presented token (single-use / reuse prevention), re-resolves the user's current
// claims, and issues a brand-new pair. Presenting a rotated token again fails
// (record not found), which is what makes rotation meaningful.
func (tm *TokenManager) RefreshTokenPair(ctx context.Context, refreshTokenValue string, deviceFingerprint string) (*TokenPair, error) {
	tokenHash := hashToken(refreshTokenValue)

	var refreshToken RefreshToken
	if err := tm.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&refreshToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid refresh token")
		}
		return nil, fmt.Errorf("failed to find refresh token: %w", err)
	}

	// Bind the token to its originating device: if both sides present a
	// fingerprint they must match (correct comparison — previously the stored
	// fingerprint was compared against the raw token value and always failed).
	if deviceFingerprint != "" && refreshToken.DeviceFingerprint != "" && refreshToken.DeviceFingerprint != deviceFingerprint {
		return nil, fmt.Errorf("device fingerprint mismatch")
	}

	// ROTATION: delete the presented token before issuing a new one. Any replay of
	// this exact token from here on hits ErrRecordNotFound above.
	if err := tm.db.WithContext(ctx).Delete(&refreshToken).Error; err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	if refreshToken.IsExpired() {
		return nil, fmt.Errorf("refresh token expired")
	}

	// Preserve (and freshen) permissions + org roles + claims via the resolver.
	// Fall back to the token's own tenant with empty perms only if unconfigured.
	tenantID := refreshToken.TenantID
	var orgRoles map[uuid.UUID]string
	var permissions, featureFlags []string
	if tm.resolver != nil {
		sc, err := tm.resolver(ctx, refreshToken.UserID)
		if err != nil {
			return nil, err
		}
		if sc.TenantID != uuid.Nil {
			tenantID = sc.TenantID
		}
		orgRoles, permissions, featureFlags = sc.OrgRoles, sc.Permissions, sc.FeatureFlags
	}

	fp := deviceFingerprint
	if fp == "" {
		fp = refreshToken.DeviceFingerprint
	}
	return tm.GenerateTokenPair(ctx, refreshToken.UserID, tenantID, orgRoles, permissions, featureFlags, fp)
}

// RevokeRefreshToken revokes a refresh token
func (tm *TokenManager) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	tokenHash := hashToken(refreshTokenValue)

	result := tm.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&RefreshToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("refresh token not found")
	}
	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (tm *TokenManager) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	result := tm.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&RefreshToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", result.Error)
	}
	return nil
}

// generateRefreshToken generates a cryptographically secure random refresh token
func generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashToken returns the SHA-256 hex digest used to store/look up refresh tokens.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Value implements driver.Valuer for JSON marshaling
func (rt RefreshToken) Value() (driver.Value, error) {
	return json.Marshal(rt)
}

// Scan implements sql.Scanner for JSON unmarshaling
func (rt *RefreshToken) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), rt)
}
