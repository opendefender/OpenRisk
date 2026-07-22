// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// Agent token scopes. These are placed in the JWT Permissions claim so the same
// RS256 signer/validator used for user tokens works unchanged, while the token
// grants ONLY scanner operations — never risk/compliance/etc.
const (
	ScopeRegister = "scanner:register" // 24h token embedded in the Agent download
	ScopeStream   = "scanner:stream"   // long-lived, lets the Agent hold the SSE job stream
	ScopePush     = "scanner:push"     // long-lived, lets the Agent push results
)

const (
	// RegistrationTokenTTL — the download's embedded token is valid 24h.
	RegistrationTokenTTL = 24 * time.Hour
	// AgentTokenTTL — the scoped agent token rotates every 7 days.
	AgentTokenTTL = 7 * 24 * time.Hour
)

// ErrScannerScope is returned when a token is valid but lacks the required
// scanner scope.
var ErrScannerScope = errors.New("token missing required scanner scope")

// MintRegistrationToken issues the 24h token embedded in an Agent download. The
// config ID is carried in the JWT subject so registration knows which ScanConfig
// the Agent is enrolling into; the tenant is the standard tenant_id claim.
func MintRegistrationToken(keys *authpkg.RSAKeys, tenantID, configID uuid.UUID) (string, error) {
	token, _, err := authpkg.GenerateAccessToken(
		keys, configID, tenantID, nil, []string{ScopeRegister}, nil, RegistrationTokenTTL,
	)
	return token, err
}

// MintAgentToken issues the long-lived scoped token an Agent uses for the SSE
// stream and pushes. The subject is the Agent ID.
func MintAgentToken(keys *authpkg.RSAKeys, tenantID, agentID uuid.UUID) (string, error) {
	token, _, err := authpkg.GenerateAccessToken(
		keys, agentID, tenantID, nil, []string{ScopeStream, ScopePush}, nil, AgentTokenTTL,
	)
	return token, err
}

// HashToken returns the SHA-256 hex of a token. The SaaS stores this (never the
// token) so it can authenticate an Agent's requests and revoke instantly by
// clearing/replacing the hash.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ValidateScannerToken validates an RS256 token and asserts it carries the given
// scanner scope. Reused by both the registration endpoint (ScopeRegister) and
// the agent-auth middleware (ScopePush/ScopeStream).
func ValidateScannerToken(keys *authpkg.RSAKeys, tokenString, requiredScope string, blacklist func(jti string) (bool, error)) (*authpkg.Claims, error) {
	claims, err := authpkg.ValidateAccessToken(keys, tokenString, blacklist)
	if err != nil {
		return nil, err
	}
	if !claims.HasPermission(requiredScope) {
		return nil, ErrScannerScope
	}
	return claims, nil
}

// GenerateHMACSecret returns 32 random bytes hex-encoded, used as the per-agent
// push-signing secret. Handed to the Agent once at registration; stored encrypted
// server-side.
func GenerateHMACSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SignPush computes the HMAC-SHA256 (hex) of a request body with the agent's push
// secret. The Agent sends this in the X-OpenRisk-Signature header.
func SignPush(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyPushSignature constant-time-compares a provided signature against the
// expected HMAC of the body.
func VerifyPushSignature(secret string, body []byte, providedHex string) bool {
	expected := SignPush(secret, body)
	return hmac.Equal([]byte(expected), []byte(providedHex))
}
