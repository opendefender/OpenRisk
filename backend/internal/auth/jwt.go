// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents the JWT claims for RS256 tokens
type Claims struct {
	Sub           uuid.UUID              `json:"sub"`
	TenantID      uuid.UUID              `json:"tenant_id"`
	OrgRoles      map[uuid.UUID]string   `json:"org_roles"`      // org_id → role
	Permissions   []string               `json:"permissions"`
	FeatureFlags  []string               `json:"feature_flags"`
	JTI           string                 `json:"jti"`            // JWT ID for blacklist
	IssuedAt      int64                  `json:"iat"`
	ExpiresAt     int64                  `json:"exp"`
}

// Valid implements jwt.Claims interface
func (c *Claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

func (c *Claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

func (c *Claims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (c *Claims) GetIssuer() (string, error) {
	return "openrisk", nil
}

func (c *Claims) GetSubject() (string, error) {
	return c.Sub.String(), nil
}

func (c *Claims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{"openrisk-api"}, nil
}

// RSAKeys holds the RSA key pair for JWT signing
type RSAKeys struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// MustLoadRSAKeys loads RSA keys from files, panics on failure
func MustLoadRSAKeys(privateKeyPath, publicKeyPath string) *RSAKeys {
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load RSA private key: %v", err))
	}

	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load RSA public key: %v", err))
	}

	return &RSAKeys{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}
}

// loadPrivateKey loads RSA private key from PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return key, nil
}

// loadPublicKey loads RSA public key from PEM file
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	keyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	key, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return key, nil
}

// GenerateAccessToken creates a new access token with RS256 signing
func GenerateAccessToken(keys *RSAKeys, claims *Claims) (string, error) {
	// Set timestamps if not set
	now := time.Now().Unix()
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now
	}
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = now + 15*60 // 15 minutes
	}

	// Generate JTI if not set
	if claims.JTI == "" {
		jti, err := generateJTI()
		if err != nil {
			return "", fmt.Errorf("failed to generate JTI: %w", err)
		}
		claims.JTI = jti
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(keys.PrivateKey)
}

// ValidateAccessToken validates and parses an access token
func ValidateAccessToken(keys *RSAKeys, tokenString string, blacklistChecker func(jti string) (bool, error)) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return keys.PublicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if token is expired
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	// Check JTI blacklist
	if blacklistChecker != nil {
		isBlacklisted, err := blacklistChecker(claims.JTI)
		if err != nil {
			return nil, fmt.Errorf("blacklist check failed: %w", err)
		}
		if isBlacklisted {
			return nil, ErrTokenRevoked
		}
	}

	return claims, nil
}

// generateJTI generates a cryptographically secure random JTI
func generateJTI() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

// Errors
var (
	ErrTokenExpired = fmt.Errorf("token expired")
	ErrTokenRevoked = fmt.Errorf("token revoked")
	ErrTokenInvalid = fmt.Errorf("token invalid")
)