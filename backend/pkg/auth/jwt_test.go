// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testKeys(t *testing.T) *RSAKeys {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	return &RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey}
}

// Round-trips a normal access token through the single mint+validate path and
// asserts the claims survive, the Type is empty (access), and wildcard perms work.
func TestGenerateAndValidateAccessToken(t *testing.T) {
	keys := testKeys(t)
	uid, tid := uuid.New(), uuid.New()
	orgRoles := map[uuid.UUID]string{tid: "admin"}

	tok, jti, err := GenerateAccessToken(keys, uid, tid, orgRoles, []string{"risks:*"}, nil, 15*time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if jti == "" {
		t.Fatal("expected non-empty jti")
	}

	claims, err := ValidateAccessToken(keys, tok, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Sub != uid || claims.TenantID != tid {
		t.Fatalf("subject/tenant mismatch: %v %v", claims.Sub, claims.TenantID)
	}
	if claims.Type != TokenTypeAccess {
		t.Fatalf("expected empty access type, got %q", claims.Type)
	}
	if !claims.HasPermission("risks:read") {
		t.Fatal("wildcard risks:* should grant risks:read")
	}
	if claims.HasPermission("assets:read") {
		t.Fatal("risks:* must NOT grant assets:read")
	}
}

// The MFA-challenge token must validate as a token but carry Type ==
// MFA_REQUIRED and no permissions — this is what MFATokenMiddleware keys off.
func TestGenerateTypedToken_MFARequired(t *testing.T) {
	keys := testKeys(t)
	uid, tid := uuid.New(), uuid.New()

	tok, _, err := GenerateTypedToken(keys, uid, tid, nil, nil, nil, 5*time.Minute, TokenTypeMFARequired)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := ValidateToken(keys, tok, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Type != TokenTypeMFARequired {
		t.Fatalf("expected MFA_REQUIRED, got %q", claims.Type)
	}
	if len(claims.Permissions) != 0 {
		t.Fatalf("MFA challenge token must carry no permissions, got %v", claims.Permissions)
	}
}

// A token signed by a different key must be rejected — guards the single
// validate path against accepting foreign signatures.
func TestValidateAccessToken_WrongKey(t *testing.T) {
	signer := testKeys(t)
	verifier := testKeys(t)
	tok, _, err := GenerateAccessToken(signer, uuid.New(), uuid.New(), nil, nil, nil, time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ValidateAccessToken(verifier, tok, nil); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid for foreign signature, got %v", err)
	}
}

// An expired token must map to ErrTokenExpired.
func TestValidateAccessToken_Expired(t *testing.T) {
	keys := testKeys(t)
	tok, _, err := GenerateAccessToken(keys, uuid.New(), uuid.New(), nil, nil, nil, -time.Minute)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ValidateAccessToken(keys, tok, nil); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

// #944 — ValidateAccessToken answers "is this a full access token?". A typed
// token (MFA_REQUIRED from a password-only login, MFA_ENROLLMENT for a mandated
// enrolment) is validly signed but is NOT an access token, and every caller
// that treats the result as a session must see it refused.
func TestValidateAccessToken_RejectsTypedTokens(t *testing.T) {
	keys := testKeys(t)
	for _, typ := range []string{TokenTypeMFARequired, TokenTypeMFAEnrollment, "refresh", "anything-else"} {
		t.Run(typ, func(t *testing.T) {
			tok, _, err := GenerateTypedToken(keys, uuid.New(), uuid.New(), nil, nil, nil, 5*time.Minute, typ)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if claims, err := ValidateAccessToken(keys, tok, nil); err != ErrTokenInvalid || claims != nil {
				t.Fatalf("typed token %q: got (%v, %v), want (nil, ErrTokenInvalid)", typ, claims, err)
			}
		})
	}
}

// The type check must not bypass the blacklist, and a blacklist outage (which
// fails open for availability) must not turn a typed token into an access one.
func TestValidateAccessToken_TypedTokenRefusedEvenWhenBlacklistFailsOpen(t *testing.T) {
	keys := testKeys(t)
	tok, _, err := GenerateTypedToken(keys, uuid.New(), uuid.New(), nil, nil, nil, 5*time.Minute, TokenTypeMFARequired)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	broken := func(string) (bool, error) { return false, errors.New("redis down") }
	if _, err := ValidateAccessToken(keys, tok, broken); err != ErrTokenInvalid {
		t.Fatalf("got %v, want ErrTokenInvalid", err)
	}
}

// ValidateToken is the type-agnostic primitive the MFA middlewares build on: it
// checks signature, expiry and blacklist, and leaves Type to the caller.
func TestValidateToken_ReturnsTheTypeForTheCallerToCheck(t *testing.T) {
	keys := testKeys(t)
	for _, typ := range []string{TokenTypeAccess, TokenTypeMFARequired, TokenTypeMFAEnrollment} {
		tok, jti, err := GenerateTypedToken(keys, uuid.New(), uuid.New(), nil, nil, nil, 5*time.Minute, typ)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		claims, err := ValidateToken(keys, tok, nil)
		if err != nil || claims.Type != typ {
			t.Fatalf("type %q: got (%v, %v)", typ, claims, err)
		}
		revoked := func(j string) (bool, error) { return j == jti, nil }
		if _, err := ValidateToken(keys, tok, revoked); err != ErrTokenRevoked {
			t.Fatalf("type %q revoked: got %v, want ErrTokenRevoked", typ, err)
		}
	}
}
