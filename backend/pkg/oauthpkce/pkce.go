// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package oauthpkce implements Proof Key for Code Exchange (RFC 7636).
//
// PKCE closes the authorization-code interception window. Without it, the code
// returned to our redirect URI is a bearer credential: anything that observes it
// — a malicious browser extension, a logged Referer, a mis-registered custom
// scheme, a shared-host proxy — can redeem it for tokens, because the token
// endpoint only asks for the code and the client secret.
//
// With PKCE we send a one-way hash of a fresh random secret up front
// (code_challenge), and the actual secret (code_verifier) only on the back
// channel at exchange time. A stolen code is then useless on its own: the thief
// would also need the verifier, which never left this server.
//
// state and PKCE solve different problems and both are required. state binds the
// callback to the browser session that started it (CSRF); PKCE binds the code to
// the client that requested it (interception). Neither substitutes for the other.
package oauthpkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// ChallengeMethodS256 is the only method this package emits.
//
// RFC 7636 also permits "plain", where the challenge IS the verifier. That
// offers no protection against an attacker who can see the authorization
// request, which is precisely the attacker PKCE exists for.
const ChallengeMethodS256 = "S256"

// verifierBytes is the entropy behind a code verifier.
//
// 32 bytes → 43 base64url characters, the minimum length RFC 7636 §4.1 allows
// and comfortably beyond brute force.
const verifierBytes = 32

// Pair is a generated verifier and its derived challenge.
type Pair struct {
	// Verifier is the secret. It stays server-side, sent only on the back
	// channel at token exchange.
	Verifier string
	// Challenge is the public S256 hash, safe to put in a redirect URL.
	Challenge string
	// Method is always ChallengeMethodS256.
	Method string
}

// New generates a fresh verifier/challenge pair.
func New() (Pair, error) {
	buf := make([]byte, verifierBytes)
	if _, err := rand.Read(buf); err != nil {
		return Pair{}, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	return Pair{
		Verifier:  verifier,
		Challenge: Challenge(verifier),
		Method:    ChallengeMethodS256,
	}, nil
}

// Challenge derives the S256 challenge for a verifier.
//
// Per RFC 7636 §4.2 the hash is taken over the ASCII characters of the verifier
// — the encoded string itself, not the bytes it decodes to. Hashing the decoded
// bytes instead is a classic PKCE bug: it is self-consistent, so it works
// against your own code and fails against every real provider.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Verify reports whether a verifier matches a challenge.
//
// The authorization server does this check; we keep it so the flow can be
// exercised end to end in tests without a live provider.
func Verify(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	return Challenge(verifier) == challenge
}
