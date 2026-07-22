// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

// This file used to carry a SECOND, independent RS256 implementation (its own
// Claims / RSAKeys / GenerateAccessToken / ValidateAccessToken / key loaders /
// sentinel errors). Tokens were signed here but validated by pkg/auth in the
// middleware — two structurally-similar-but-separate implementations that could
// drift apart at any time.
//
// There is now exactly ONE JWT implementation, in pkg/auth. Everything below is a
// thin re-export so existing `coreauth.*` references keep compiling while the real
// generation/validation/Claims/key logic lives in a single place.

import (
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// Claims is the canonical RS256 claims type (pkg/auth.Claims). Aliased — not a copy.
type Claims = authpkg.Claims

// RSAKeys is the canonical RSA key pair type (pkg/auth.RSAKeys). Aliased — not a copy.
type RSAKeys = authpkg.RSAKeys

// TokenPair is the canonical access+refresh response DTO (pkg/auth.TokenPair).
type TokenPair = authpkg.TokenPair

// Sentinel token errors — re-exported from the single source of truth.
var (
	ErrTokenExpired = authpkg.ErrTokenExpired
	ErrTokenInvalid = authpkg.ErrTokenInvalid
	ErrTokenRevoked = authpkg.ErrTokenRevoked
)

// MustLoadRSAKeys loads both RSA keys (panics on failure). Delegates to pkg/auth.
func MustLoadRSAKeys(privateKeyPath, publicKeyPath string) *RSAKeys {
	return authpkg.MustLoadRSAKeys(privateKeyPath, publicKeyPath)
}
