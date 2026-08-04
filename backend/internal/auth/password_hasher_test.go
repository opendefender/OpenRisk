// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"strings"
	"testing"
)

func TestArgon2id_HashAndVerifyRoundTrip(t *testing.T) {
	h := NewArgon2idPasswordHasher()

	const password = "correct horse battery staple 7"
	hash, err := h.Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if !h.Verify(hash, password) {
		t.Error("the correct password failed to verify")
	}
	if h.Verify(hash, password+"x") {
		t.Error("a wrong password verified")
	}
	if h.Verify(hash, "") {
		t.Error("an empty password verified")
	}
}

func TestArgon2id_UsesConfiguredParameters(t *testing.T) {
	h := NewArgon2idPasswordHasher()

	hash, err := h.Hash("some-password-value")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// PHC format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Errorf("unexpected parameters in hash prefix: %q", hash)
	}
}

func TestArgon2id_SaltIsRandomPerHash(t *testing.T) {
	h := NewArgon2idPasswordHasher()

	first, err := h.Hash("same-password-twice")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	second, err := h.Hash("same-password-twice")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if first == second {
		t.Error("identical passwords produced identical hashes — salt is not random")
	}
	if !h.Verify(first, "same-password-twice") || !h.Verify(second, "same-password-twice") {
		t.Error("both hashes must verify despite differing salts")
	}
}

// TestArgon2id_VerifiesLegacyIterationCount is the guarantee behind raising the
// iteration count from 2 to 3 (audit finding F-06): no stored password is
// invalidated. The PHC string carries its own parameters, so a hash written
// under t=2 must still verify against a hasher now configured for t=3.
//
// The fixture below was produced by the pre-change hasher.
func TestArgon2id_VerifiesLegacyIterationCount(t *testing.T) {
	legacy := &Argon2idPasswordHasher{time: 2, memory: 65536, threads: 4, keyLen: 32, saltLen: 16}
	const password = "legacy-account-password-1"

	legacyHash, err := legacy.Hash(password)
	if err != nil {
		t.Fatalf("Hash with legacy parameters: %v", err)
	}
	if !strings.Contains(legacyHash, "t=2") {
		t.Fatalf("fixture should be a t=2 hash, got %q", legacyHash)
	}

	current := NewArgon2idPasswordHasher()
	if !current.Verify(legacyHash, password) {
		t.Error("a password stored under t=2 no longer verifies — the change invalidated existing accounts")
	}
	if current.Verify(legacyHash, "wrong-password") {
		t.Error("legacy hash verified against the wrong password")
	}
}

func TestArgon2id_RejectsMalformedHashes(t *testing.T) {
	h := NewArgon2idPasswordHasher()

	malformed := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"not phc", "just-a-string"},
		{"too few segments", "$argon2id$v=19$m=65536,t=3,p=4$onlysalt"},
		{"unparseable parameters", "$argon2id$v=19$m=abc,t=x,p=y$c2FsdA$aGFzaA"},
		{"invalid base64 salt", "$argon2id$v=19$m=65536,t=3,p=4$!!!notbase64!!!$aGFzaA"},
	}

	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			if h.Verify(tc.hash, "any-password") {
				t.Errorf("Verify accepted a malformed hash: %q", tc.hash)
			}
		})
	}
}
