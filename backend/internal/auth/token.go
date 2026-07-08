// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
)

// RefreshToken represents a refresh token stored in database
type RefreshToken struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	TenantID       uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	TokenHash      string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"token_hash"` // SHA256 hash
	DeviceFingerprint string  `gorm:"type:varchar(255)" json:"device_fingerprint"`
	ExpiresAt      time.Time  `gorm:"index;not null" json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// IsExpired checks if the refresh token has expired
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// UpdateLastUsed updates the last used timestamp
func (rt *RefreshToken) UpdateLastUsed() {
	now := time.Now()
	rt.LastUsedAt = &now
	rt.UpdatedAt = now
}

// TokenPair represents an access token and refresh token pair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // seconds until access token expires
}

// TokenManager handles token operations
type TokenManager struct {
	db      *gorm.DB
	rsaKeys *RSAKeys
}

// NewTokenManager creates a new token manager
func NewTokenManager(db *gorm.DB, rsaKeys *RSAKeys) *TokenManager {
	return &TokenManager{db: db, rsaKeys: rsaKeys}
}

// GenerateTokenPair generates a new access and refresh token pair
func (tm *TokenManager) GenerateTokenPair(ctx context.Context, userID, tenantID uuid.UUID, orgRoles map[uuid.UUID]string, permissions []string, featureFlags []string, deviceFingerprint string) (*TokenPair, error) {
	// Generate refresh token (opaque string)
	refreshTokenValue, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Hash the refresh token for storage
	hash := sha256.Sum256([]byte(refreshTokenValue))
	tokenHash := hex.EncodeToString(hash[:])

	// Store refresh token in database
	refreshToken := &RefreshToken{
		UserID:            userID,
		TenantID:          tenantID,
		TokenHash:         tokenHash,
		DeviceFingerprint: deviceFingerprint,
		ExpiresAt:         time.Now().Add(30 * 24 * time.Hour), // 30 days
	}

	if err := tm.db.WithContext(ctx).Create(refreshToken).Error; err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Generate access token
	claims := &Claims{
		Sub:          userID,
		TenantID:     tenantID,
		OrgRoles:     orgRoles,
		Permissions:  permissions,
		FeatureFlags: featureFlags,
	}

	accessToken, err := GenerateAccessToken(tm.rsaKeys, claims)
	if err != nil {
		// Clean up the refresh token if access token generation fails
		tm.db.WithContext(ctx).Delete(refreshToken)
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenValue,
		TokenType:    "Bearer",
		ExpiresIn:    15 * 60, // 15 minutes
	}, nil
}

// RefreshTokenPair refreshes an access token using a valid refresh token
func (tm *TokenManager) RefreshTokenPair(ctx context.Context, refreshTokenValue string, deviceFingerprint string) (*TokenPair, error) {
	// Hash the provided refresh token
	hash := sha256.Sum256([]byte(refreshTokenValue))
	tokenHash := hex.EncodeToString(hash[:])

	// Find the refresh token in database
	var refreshToken RefreshToken
	if err := tm.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&refreshToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid refresh token")
		}
		return nil, fmt.Errorf("failed to find refresh token: %w", err)
	}

	// Check if expired
	if refreshToken.IsExpired() {
		// Delete expired token
		tm.db.WithContext(ctx).Delete(&refreshToken)
		return nil, fmt.Errorf("refresh token expired")
	}

	// Check device fingerprint if provided
	if deviceFingerprint != "" && refreshToken.DeviceFingerprint != refreshTokenValue {
		return nil, fmt.Errorf("device fingerprint mismatch")
	}

	// Update last used and generate new pair
	refreshToken.UpdateLastUsed()
	if err := tm.db.WithContext(ctx).Save(&refreshToken).Error; err != nil {
		return nil, fmt.Errorf("failed to update refresh token: %w", err)
	}

	// Generate new token pair (this will invalidate the old refresh token)
	return tm.GenerateTokenPair(ctx, refreshToken.UserID, refreshToken.TenantID, nil, nil, nil, deviceFingerprint)
}

// RevokeRefreshToken revokes a refresh token
func (tm *TokenManager) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	hash := sha256.Sum256([]byte(refreshTokenValue))
	tokenHash := hex.EncodeToString(hash[:])

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
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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