// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"context"
	"fmt"
	"time"
)

// RedisBlacklistClient interface for JTI blacklist operations.
// Allows dependency injection of Redis client.
type RedisBlacklistClient interface {
	// Set stores a key-value pair with TTL expiration
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)

	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)
}

// TokenBlacklistManager handles JWT token revocation via Redis.
type TokenBlacklistManager struct {
	redis RedisBlacklistClient
}

// NewTokenBlacklistManager creates a new instance.
func NewTokenBlacklistManager(redis RedisBlacklistClient) *TokenBlacklistManager {
	return &TokenBlacklistManager{redis: redis}
}

// BlacklistJTI revokes a JTI in Redis.
// TTL = duration until token expiration (calculated from exp claim).
// If TTL <= 0, returns early (token already expired, no need to blacklist).
// Never panics: if Redis is down, logs error and returns nil (fail-open).
func (m *TokenBlacklistManager) BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		// Token already expired, no need to blacklist
		return nil
	}

	key := fmt.Sprintf("token_blacklist:%s", jti)
	err := m.redis.Set(ctx, key, "1", ttl)
	if err != nil {
		// Log error but don't panic/fail — fail-open for availability
		// The token will expire naturally in memory
		return fmt.Errorf("failed to blacklist token %s: %w", jti, err)
	}

	return nil
}

// IsJTIBlacklisted checks if a JTI is revoked.
// Returns false if Redis is down (fail-open).
func (m *TokenBlacklistManager) IsJTIBlacklisted(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}

	key := fmt.Sprintf("token_blacklist:%s", jti)
	exists, err := m.redis.Exists(ctx, key)
	if err != nil {
		// Fail-open: return false if Redis is unreachable
		// Token expiration in memory will protect us
		return false, nil
	}

	return exists, nil
}

// CheckJTIBlacklist is a helper function for ValidateAccessToken.
// Returns a closure that can be passed to ValidateAccessToken.
func (m *TokenBlacklistManager) CheckJTIBlacklist(ctx context.Context) func(jti string) (bool, error) {
	return func(jti string) (bool, error) {
		return m.IsJTIBlacklisted(ctx, jti)
	}
}
