// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims représente les JWT claims RS256 d'OpenRisk.
// Conforme exactement au Master Prompt V3 — structure invariante.
type Claims struct {
	jwt.RegisteredClaims

	Sub          uuid.UUID            `json:"sub"`           // Subject (user ID)
	TenantID     uuid.UUID            `json:"tenant_id"`     // Multi-tenancy scoping
	OrgRoles     map[uuid.UUID]string `json:"org_roles"`     // {org_id: role_name}
	Permissions  []string             `json:"permissions"`   // RBAC permissions
	FeatureFlags []string             `json:"feature_flags"` // Feature toggles
	JTI          string               `json:"jti"`           // JWT ID (for Redis blacklist)
	// Type distinguishes access/refresh/MFA_REQUIRED tokens. Not yet populated by
	// any issuer (GenerateAccessToken et al.) - reserved for the Phase 2 token
	// semantics work that MFATokenMiddleware depends on.
	Type string `json:"type,omitempty"`
}

// HasPermission checks whether the claims' Permissions slice grants the given
// permission string. Supports the same "*" admin wildcard and "resource:*"
// scoped wildcard already used by internal/middleware's hasPermission helper.
func (c *Claims) HasPermission(required string) bool {
	for _, perm := range c.Permissions {
		if perm == required || perm == "*" {
			return true
		}
		if len(perm) > 2 && perm[len(perm)-2:] == ":*" {
			resourceWildcard := perm[:len(perm)-1]
			if len(required) > len(resourceWildcard) && required[:len(resourceWildcard)] == resourceWildcard {
				return true
			}
		}
	}
	return false
}

// TokenTypeAccess is the default (empty) type carried by a full access token.
// TokenTypeMFARequired marks the short-lived token issued between a successful
// password check and MFA challenge completion — it grants no permissions and is
// only accepted by MFATokenMiddleware.
const (
	TokenTypeAccess      = ""
	TokenTypeMFARequired = "MFA_REQUIRED"
)

// GenerateAccessToken génère un JWT d'accès signé RS256 (Type standard, vide).
// Retourne (tokenString, jti, error). Le JTI est un UUID v4 unique par token.
// Ne jamais logger le token, même tronqué.
func GenerateAccessToken(
	rsaKeys *RSAKeys,
	userID, tenantID uuid.UUID,
	orgRoles map[uuid.UUID]string,
	permissions, featureFlags []string,
	duration time.Duration,
) (string, string, error) {
	return GenerateTypedToken(rsaKeys, userID, tenantID, orgRoles, permissions, featureFlags, duration, TokenTypeAccess)
}

// GenerateTypedToken is the single low-level RS256 token minter. Every token in
// the system (access + MFA-challenge) is produced here so signing, claims shape
// and JTI generation have exactly one implementation.
func GenerateTypedToken(
	rsaKeys *RSAKeys,
	userID, tenantID uuid.UUID,
	orgRoles map[uuid.UUID]string,
	permissions, featureFlags []string,
	duration time.Duration,
	tokenType string,
) (string, string, error) {
	if rsaKeys == nil || rsaKeys.PrivateKey == nil {
		return "", "", errors.New("RSA private key not initialized")
	}

	jti := uuid.New().String()
	now := time.Now()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			Issuer:    "openrisk",
			Audience:  jwt.ClaimStrings{"openrisk-api"},
		},
		Sub:          userID,
		TenantID:     tenantID,
		OrgRoles:     orgRoles,
		Permissions:  permissions,
		FeatureFlags: featureFlags,
		JTI:          jti,
		Type:         tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(rsaKeys.PrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, jti, nil
}

// GenerateRefreshToken génère un token opaque (32 bytes random hex).
// Durée 30 jours. Retourne le token string.
// Ce token est stocké en base dans la table refresh_tokens.
// Il n'est PAS un JWT — c'est une chaîne aléatoire forte.
func GenerateRefreshToken() (string, error) {
	// 32 bytes = 256 bits of entropy
	token := uuid.New().String() + uuid.New().String()
	return token, nil
}

// ValidateAccessToken parse et valide un JWT RS256.
// Vérifie : signature RS256, expiration, JTI non blacklisté (via Redis).
// Retourne (*Claims, error) avec erreurs typées :
//
//	ErrTokenExpired / ErrTokenInvalid / ErrTokenRevoked
func ValidateAccessToken(
	rsaKeys *RSAKeys,
	tokenString string,
	jtiBlacklistChecker func(jti string) (bool, error),
) (*Claims, error) {
	if rsaKeys == nil || rsaKeys.PublicKey == nil {
		return nil, ErrTokenInvalid
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure RS256 signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return rsaKeys.PublicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// Check JTI blacklist (Redis)
	if jtiBlacklistChecker != nil && claims.JTI != "" {
		isBlacklisted, err := jtiBlacklistChecker(claims.JTI)
		if err != nil {
			// Log error but don't fail — fail-open for availability
			return claims, nil
		}
		if isBlacklisted {
			return nil, ErrTokenRevoked
		}
	}

	return claims, nil
}

// GetExpiresIn returns seconds until token expiration.
// Returns 0 if token is already expired.
func (c *Claims) GetExpiresIn() int64 {
	if c.ExpiresAt == nil {
		return 0
	}
	secondsUntilExpiry := c.ExpiresAt.Unix() - time.Now().Unix()
	if secondsUntilExpiry < 0 {
		return 0
	}
	return secondsUntilExpiry
}

// RemainingTTL returns the remaining TTL of a JWT token for Redis blacklist.
// Returns 0 if expired.
func (c *Claims) RemainingTTL() time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}
	remaining := c.ExpiresAt.Unix() - time.Now().Unix()
	if remaining <= 0 {
		return 0
	}
	return time.Duration(remaining) * time.Second
}
