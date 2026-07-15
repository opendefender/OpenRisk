// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"crypto/rand"
	"crypto/rsa"
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

// The MFA-challenge token must validate but carry Type == MFA_REQUIRED and no
// permissions — this is what MFATokenMiddleware keys off to reject it on normal
// protected routes.
func TestGenerateTypedToken_MFARequired(t *testing.T) {
	keys := testKeys(t)
	uid, tid := uuid.New(), uuid.New()

	tok, _, err := GenerateTypedToken(keys, uid, tid, nil, nil, nil, 5*time.Minute, TokenTypeMFARequired)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := ValidateAccessToken(keys, tok, nil)
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
