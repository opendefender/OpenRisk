// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package otp

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// #849 — MatchTOTPStep names the step a code belongs to, within ±1 step.
func TestMatchTOTPStep(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_900_000_015, 0) // mid-step, so ±1 is unambiguous
	current := now.Unix() / 30
	codeAt := func(step int64) string {
		c, err := totp.GenerateCode(secret, time.Unix(step*30, 0))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	for name, tc := range map[string]struct {
		step int64
		ok   bool
	}{
		"previous step": {current - 1, true},
		"current step":  {current, true},
		"next step":     {current + 1, true},
		"two back":      {current - 2, false},
		"two ahead":     {current + 2, false},
	} {
		t.Run(name, func(t *testing.T) {
			code := codeAt(tc.step)
			got, ok := MatchTOTPStep(secret, code, now)
			if ok != tc.ok {
				// A code two steps away can collide with a neighbour's by chance
				// (1 in a million); only flag it when it did not.
				if !tc.ok && (code == codeAt(current-1) || code == codeAt(current) || code == codeAt(current+1)) {
					t.Skip("code collides with an in-window step")
				}
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.step {
				t.Fatalf("step = %d, want %d", got, tc.step)
			}
		})
	}

	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := MatchTOTPStep(secret, bad, now); ok {
			t.Fatalf("%q must not match", bad)
		}
	}
}
