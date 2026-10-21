// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"strings"
	"testing"
)

func TestSendDuplicateSignup_GivesBothWaysBackInBothLanguages(t *testing.T) {
	cases := []struct {
		locale, subject, signIn, reset string
	}{
		{"en", "tried to create an OpenRisk account", "Sign in", "Reset my password"},
		{"fr", "tenté de créer un compte OpenRisk", "Se connecter", "Réinitialiser mon mot de passe"},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			s := &capturingSender{}
			err := New(s).SendDuplicateSignup(context.Background(), "awa@example.test", "Awa Diallo",
				"https://risk.example.test/login", "https://risk.example.test/forgot-password", tc.locale)
			if err != nil {
				t.Fatal(err)
			}
			if s.to != "awa@example.test" {
				t.Fatalf("sent to %q", s.to)
			}
			if !strings.Contains(s.subject, tc.subject) {
				t.Fatalf("subject %q lacks %q", s.subject, tc.subject)
			}
			for _, want := range []string{
				"Awa Diallo", tc.signIn, tc.reset, "24",
				`href="https://risk.example.test/login"`,
				`href="https://risk.example.test/forgot-password"`,
			} {
				if !strings.Contains(s.body, want) {
					t.Fatalf("body lacks %q", want)
				}
			}
		})
	}
}

func TestSendDuplicateSignup_NoSenderIsANoOp(t *testing.T) {
	if err := New(nil).SendDuplicateSignup(context.Background(), "a@example.test", "", "", "", "en"); err != nil {
		t.Fatalf("an unwired mailer must not fail the sign-up: %v", err)
	}
}
