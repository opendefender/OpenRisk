// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package otp

import (
	"strings"
	"testing"
)

// TestGenerateBackupCodes_ShapeAndAlphabet asserts the count, length and that
// every character is drawn from the documented alphabet.
func TestGenerateBackupCodes_ShapeAndAlphabet(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(codes) != backupCodeCount {
		t.Fatalf("expected %d codes, got %d", backupCodeCount, len(codes))
	}
	for i, c := range codes {
		if len(c) != backupCodeLength {
			t.Fatalf("code %d: expected length %d, got %d (%q)", i, backupCodeLength, len(c), c)
		}
		for _, r := range c {
			if !strings.ContainsRune(backupCodeAlphabet, r) {
				t.Fatalf("code %d contains char %q outside alphabet", i, r)
			}
		}
	}
}

// TestGenerateBackupCodes_UniqueWithinCall guards against the historical bug
// where all codes were identical/derived from one constant seed.
func TestGenerateBackupCodes_UniqueWithinCall(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	seen := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		if _, dup := seen[c]; dup {
			t.Fatalf("duplicate backup code within a single call: %q", c)
		}
		seen[c] = struct{}{}
	}
}

// TestGenerateBackupCodes_UniqueAcrossCalls is the regression test for the S0
// finding: two users (two calls) MUST NOT receive the same codes. With a fixed
// seed this failed (16/16 identical); with crypto/rand a collision is
// astronomically unlikely.
func TestGenerateBackupCodes_UniqueAcrossCalls(t *testing.T) {
	a, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error (a): %v", err)
	}
	b, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("unexpected error (b): %v", err)
	}

	all := make(map[string]struct{}, len(a)+len(b))
	for _, c := range append(append([]string{}, a...), b...) {
		if _, dup := all[c]; dup {
			t.Fatalf("backup code %q appeared in two independent generations — codes are not unique per user", c)
		}
		all[c] = struct{}{}
	}
	if len(all) != len(a)+len(b) {
		t.Fatalf("expected %d distinct codes across two calls, got %d", len(a)+len(b), len(all))
	}
}
