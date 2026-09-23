// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"
)

func legacyDigest(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// fastHasher is the real hasher at its floor cost; the tests below are about
// formats and decisions, not about how slow a hash is.
func fastHasher() *Argon2idPasswordHasher {
	return NewArgon2idPasswordHasherWithParams(Argon2idParams{Time: 2, Memory: 19456, Threads: 1})
}

func TestVerify_LegacySHA256_Success(t *testing.T) {
	h := fastHasher()
	stored := legacyDigest("legacy-password-1")

	if !h.Verify(stored, "legacy-password-1") {
		t.Error("a legacy digest must verify its own password so the account can migrate")
	}
	if !h.Verify(strings.ToUpper(stored), "legacy-password-1") {
		t.Error("hex case must not matter")
	}
}

func TestVerify_LegacySHA256_WrongPassword(t *testing.T) {
	h := fastHasher()
	if h.Verify(legacyDigest("legacy-password-1"), "legacy-password-2") {
		t.Error("a wrong password verified against a legacy digest")
	}
}

func TestVerify_UnknownFormatVerifiesNothing(t *testing.T) {
	h := fastHasher()
	for _, stored := range []string{
		"",
		"plaintext-password",
		"$2a$10$abcdefghijklmnopqrstuv", // bcrypt: never written by this product
		legacyDigest("x")[:63],          // truncated digest
		"$argon2id$v=19$m=0,t=0,p=0$$",
	} {
		if h.Verify(stored, "plaintext-password") {
			t.Errorf("stored %q verified; an unrecognised format must verify nothing", stored)
		}
	}
}

func TestHashAlgorithm_Classifies(t *testing.T) {
	current, err := fastHasher().Hash("p")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		current:                 AlgorithmArgon2id,
		legacyDigest("p"):       AlgorithmLegacySHA256,
		"":                      AlgorithmUnknown,
		"not-a-hash":            AlgorithmUnknown,
		strings.Repeat("g", 64): AlgorithmUnknown,
	}
	for stored, want := range cases {
		if got := HashAlgorithm(stored); got != want {
			t.Errorf("HashAlgorithm(%q) = %q, want %q", stored, got, want)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	current := NewArgon2idPasswordHasherWithParams(Argon2idParams{Time: 3, Memory: 19456, Threads: 1})
	weaker := NewArgon2idPasswordHasherWithParams(Argon2idParams{Time: 2, Memory: 19456, Threads: 1})

	upToDate, err := current.Hash("p")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := weaker.Hash("p")
	if err != nil {
		t.Fatal(err)
	}

	if !current.NeedsRehash(legacyDigest("p")) {
		t.Error("a legacy digest must be scheduled for rehash")
	}
	if !current.NeedsRehash(stale) {
		t.Error("an Argon2id hash below today's cost must be scheduled for rehash")
	}
	if current.NeedsRehash(upToDate) {
		t.Error("a current hash must not be rehashed on every sign-in")
	}
	if current.NeedsRehash("") {
		t.Error("an empty column (SSO-only account) has nothing to rehash")
	}
}

// TestHash_NeverWritesALegacyHash is the non-regression guard of #484: whatever
// the input, the only thing the hasher can produce is an Argon2id PHC string.
func TestHash_NeverWritesALegacyHash(t *testing.T) {
	h := fastHasher()
	for _, password := range []string{"", "a", "correct horse battery staple", strings.Repeat("x", 512), "ünïcødé-🔐"} {
		stored, err := h.Hash(password)
		if err != nil {
			t.Fatalf("Hash(%q): %v", password, err)
		}
		if HashAlgorithm(stored) != AlgorithmArgon2id {
			t.Errorf("Hash(%q) produced %q (%s); only Argon2id may be written", password, stored, HashAlgorithm(stored))
		}
		if IsLegacyHash(stored) {
			t.Errorf("Hash(%q) produced a legacy digest", password)
		}
	}
}

// TestNoSHA256WritePathInThePasswordHasher pins the second half of the guard:
// SHA-256 appears in the password hasher only inside verifyLegacySHA256, which
// reads and never writes. Reintroducing a SHA-256 hasher, or computing a digest
// anywhere else in this file, fails here before it can reach a database.
func TestNoSHA256WritePathInThePasswordHasher(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "password_hasher.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "sha256" && fn.Name.Name != "verifyLegacySHA256" {
				t.Errorf("%s: sha256.%s used in %s; SHA-256 may only be read, in verifyLegacySHA256",
					fset.Position(sel.Pos()), sel.Sel.Name, fn.Name.Name)
			}
			return true
		})
	}
}

func TestArgon2idParamsFromEnv_Defaults(t *testing.T) {
	for _, key := range []string{"ARGON2ID_MEMORY_KIB", "ARGON2ID_TIME", "ARGON2ID_PARALLELISM"} {
		t.Setenv(key, "") // registers the restore
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}

	p, err := Argon2idParamsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p != DefaultArgon2idParams() {
		t.Errorf("unset environment must yield the defaults, got %+v", p)
	}
	if p.Memory != 65536 || p.Time != 3 || p.Threads != 4 || p.KeyLen != 32 || p.SaltLen != 16 {
		t.Errorf("DefaultArgon2idParams changed without this test: %+v", p)
	}
}

func TestArgon2idParamsFromEnv_Overrides(t *testing.T) {
	t.Setenv("ARGON2ID_MEMORY_KIB", "131072")
	t.Setenv("ARGON2ID_TIME", "4")
	t.Setenv("ARGON2ID_PARALLELISM", "2")

	p, err := Argon2idParamsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p.Memory != 131072 || p.Time != 4 || p.Threads != 2 {
		t.Errorf("overrides not applied: %+v", p)
	}
}

func TestArgon2idParamsFromEnv_RefusesWeakOrGarbage(t *testing.T) {
	for _, c := range []struct{ key, value string }{
		{"ARGON2ID_MEMORY_KIB", "1024"},
		{"ARGON2ID_MEMORY_KIB", "lots"},
		{"ARGON2ID_TIME", "1"},
		{"ARGON2ID_TIME", ""},
		{"ARGON2ID_PARALLELISM", "0"},
		{"ARGON2ID_PARALLELISM", "256"},
	} {
		t.Run(c.key+"="+c.value, func(t *testing.T) {
			t.Setenv(c.key, c.value)
			if _, err := Argon2idParamsFromEnv(); err == nil {
				t.Errorf("%s=%q accepted; a weak or malformed cost must stop the server", c.key, c.value)
			}
		})
	}
}

func TestLegacyHashCutoff_DefaultAndOverride(t *testing.T) {
	t.Setenv("PASSWORD_LEGACY_HASH_CUTOFF", "")
	got, err := LegacyHashCutoff()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC); !got.Equal(want) {
		t.Errorf("default cutoff = %s, want %s", got, want)
	}

	t.Setenv("PASSWORD_LEGACY_HASH_CUTOFF", "2027-03-31T00:00:00+01:00")
	got, err = LegacyHashCutoff()
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2027, 3, 30, 23, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("override cutoff = %s, want %s", got, want)
	}
}

func TestLegacyHashCutoff_Malformed(t *testing.T) {
	t.Setenv("PASSWORD_LEGACY_HASH_CUTOFF", "31/12/2026")
	if _, err := LegacyHashCutoff(); err == nil {
		t.Error("a malformed cutoff must be an error, not a silent default")
	}
	if LegacyHashCutoffOrDefault().IsZero() {
		t.Error("the use-case fallback must never leave the migration without a deadline")
	}
}

func TestLegacyHashExpired(t *testing.T) {
	cutoff := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	legacy := legacyDigest("p")
	current, _ := fastHasher().Hash("p")

	if LegacyHashExpired(legacy, cutoff.Add(-time.Second), cutoff) {
		t.Error("a legacy hash is still accepted before the cutoff")
	}
	if !LegacyHashExpired(legacy, cutoff.Add(time.Second), cutoff) {
		t.Error("a legacy hash must be refused after the cutoff")
	}
	if LegacyHashExpired(current, cutoff.Add(time.Hour), cutoff) {
		t.Error("the cutoff only ever applies to legacy hashes")
	}
}
