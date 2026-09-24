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

// D-048: the owner refused the transparent migration. A SHA-256 digest from the
// first release verifies nothing, even with the right password; the account
// signs in again after a password reset.
func TestVerify_LegacySHA256_IsRefusedEvenWithTheRightPassword(t *testing.T) {
	h := fastHasher()
	stored := legacyDigest("legacy-password-1")

	if h.Verify(stored, "legacy-password-1") {
		t.Error("a legacy digest verified; D-048 requires a reset instead")
	}
	if h.Verify(strings.ToUpper(stored), "legacy-password-1") {
		t.Error("an upper-case legacy digest verified")
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

	if current.NeedsRehash(legacyDigest("p")) {
		t.Error("a legacy digest cannot verify, so it can never be rehashed at sign-in")
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
	}
}

// TestNoSHA256InThePasswordHasher pins the second half of the guard: the
// password hasher does not use SHA-256 at all, to write or to check.
// Reintroducing a SHA-256 hasher, or a SHA-256 check, fails here.
func TestNoSHA256InThePasswordHasher(t *testing.T) {
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
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "sha256" {
				t.Errorf("%s: sha256.%s used in %s; the password hasher must not use SHA-256",
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

func TestNewConfiguredArgon2idPasswordHasher_AppliesEnvironment(t *testing.T) {
	t.Setenv("ARGON2ID_MEMORY_KIB", "19456")
	t.Setenv("ARGON2ID_TIME", "2")
	t.Setenv("ARGON2ID_PARALLELISM", "1")

	stored, err := NewConfiguredArgon2idPasswordHasher().Hash("p")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("environment parameters not applied: %q", stored)
	}
}

func TestNewConfiguredArgon2idPasswordHasher_MalformedFallsBackToDefaults(t *testing.T) {
	t.Setenv("ARGON2ID_TIME", "1")
	if got := NewConfiguredArgon2idPasswordHasher().Params(); got != DefaultArgon2idParams() {
		t.Errorf("a refused value must fall back to the defaults, never to itself: %+v", got)
	}
}

// A refused legacy digest must cost about what a failed Argon2id check costs;
// otherwise response time alone tells an attacker which accounts exist and
// still hold an old hash. Without the fix the gap is four orders of
// magnitude, so a loose bound is enough and keeps the test stable.
func TestVerify_LegacySHA256_CostsAsMuchAsArgon2id(t *testing.T) {
	h := fastHasher()
	current, err := h.Hash("the-real-password")
	if err != nil {
		t.Fatal(err)
	}
	legacy := legacyDigest("the-real-password")

	measure := func(stored string) time.Duration {
		start := time.Now()
		for i := 0; i < 3; i++ {
			h.Verify(stored, "a-wrong-guess")
		}
		return time.Since(start)
	}
	argon := measure(current)
	old := measure(legacy)

	if old < argon*3/10 {
		t.Errorf("legacy check took %s against %s for Argon2id; the difference reveals the account", old, argon)
	}
}

func TestArgon2idMaxConcurrentFromEnv(t *testing.T) {
	t.Setenv("ARGON2ID_MAX_CONCURRENT", "8")
	if n, err := Argon2idMaxConcurrentFromEnv(); err != nil || n != 8 {
		t.Errorf("got %d, %v; want 8", n, err)
	}
	for _, bad := range []string{"0", "-1", "lots", ""} {
		t.Setenv("ARGON2ID_MAX_CONCURRENT", bad)
		if _, err := Argon2idMaxConcurrentFromEnv(); err == nil {
			t.Errorf("ARGON2ID_MAX_CONCURRENT=%q accepted", bad)
		}
	}
	t.Setenv("ARGON2ID_MAX_CONCURRENT", "")
	if err := os.Unsetenv("ARGON2ID_MAX_CONCURRENT"); err != nil {
		t.Fatal(err)
	}
	if n, err := Argon2idMaxConcurrentFromEnv(); err != nil || n != DefaultArgon2idMaxConcurrent {
		t.Errorf("unset: got %d, %v; want the default", n, err)
	}
}

// D-048, option C: no more than the cap run at once, however many sign-ins
// arrive together. The slots are filled by hand so the test does not depend on
// how fast this machine derives a key.
func TestDeriveArgon2id_WaitsForAFreeSlot(t *testing.T) {
	h := fastHasher()
	if _, err := h.Hash("warm-up"); err != nil { // initialises the process-wide cap
		t.Fatal(err)
	}
	for i := 0; i < cap(argon2Slots); i++ {
		argon2Slots <- struct{}{}
	}

	done := make(chan struct{})
	go func() {
		_, _ = h.Hash("blocked")
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("a derivation ran while every slot was taken")
	case <-time.After(200 * time.Millisecond):
	}

	<-argon2Slots // free one slot
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the waiting derivation never ran after a slot was freed")
	}
	for i := 0; i < cap(argon2Slots)-1; i++ {
		<-argon2Slots
	}
}
