// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import "errors"

// Erreurs typées (sentinel errors) — utilisées dans tout le projet
var (
	// ErrTokenExpired indique que le token a expiré
	ErrTokenExpired = errors.New("TOKEN_EXPIRED")

	// ErrTokenInvalid indique que le token est invalide (signature, format, etc.)
	ErrTokenInvalid = errors.New("TOKEN_INVALID")

	// ErrTokenRevoked indique que le token a été révoqué (blacklist Redis)
	ErrTokenRevoked = errors.New("TOKEN_REVOKED")
)

// TokenPair regroupe access + refresh token pour les réponses API
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // secondes
	TokenType    string `json:"token_type"` // "Bearer"
}

// NewTokenPair crée une nouvelle paire de tokens.
func NewTokenPair(accessToken, refreshToken string, expiresInSeconds int64) *TokenPair {
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresInSeconds,
		TokenType:    "Bearer",
	}
}
