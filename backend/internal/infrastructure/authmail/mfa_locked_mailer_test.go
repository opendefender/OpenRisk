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

func TestSendMFALocked_SaysWhatHappenedInBothLanguages(t *testing.T) {
	cases := []struct {
		locale, subject, mustSay string
	}{
		{"en", "two-factor sign-in", "reset your password"},
		{"fr", "connexion à deux facteurs", "réinitialisez votre mot de passe"},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			s := &capturingSender{}
			if err := New(s).SendMFALocked(context.Background(), "awa@example.test", "Awa Diallo", tc.locale); err != nil {
				t.Fatal(err)
			}
			if s.to != "awa@example.test" {
				t.Fatalf("sent to %q", s.to)
			}
			if !strings.Contains(s.subject, tc.subject) {
				t.Fatalf("subject %q lacks %q", s.subject, tc.subject)
			}
			for _, want := range []string{"Awa Diallo", "15 minutes", tc.mustSay} {
				if !strings.Contains(s.body, want) {
					t.Fatalf("body lacks %q", want)
				}
			}
		})
	}
}

func TestSendMFALocked_NoSenderIsANoOp(t *testing.T) {
	if err := New(nil).SendMFALocked(context.Background(), "a@example.test", "", "en"); err != nil {
		t.Fatalf("an unwired mailer must not fail the challenge: %v", err)
	}
}

// #1392 — the in-session lock mail must not claim the password was used, and
// must send the owner to "sign out all other devices".
func TestSendMFAInSessionLocked_SaysWhatHappenedInBothLanguages(t *testing.T) {
	cases := []struct {
		locale, subject, remedy string
	}{
		{"en", "two-factor changes", "Sign out all other devices"},
		{"fr", "double authentification", "Déconnecter tous les autres appareils"},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			s := &capturingSender{}
			if err := New(s).SendMFAInSessionLocked(context.Background(), "awa@example.test", "Awa Diallo", tc.locale); err != nil {
				t.Fatal(err)
			}
			if s.to != "awa@example.test" || !strings.Contains(s.subject, tc.subject) {
				t.Fatalf("to %q subject %q", s.to, s.subject)
			}
			for _, want := range []string{"Awa Diallo", "24", tc.remedy} {
				if !strings.Contains(s.body, want) {
					t.Fatalf("body lacks %q", want)
				}
			}
			for _, never := range []string{"password correctly", "mot de passe correctement"} {
				if strings.Contains(s.body, never) {
					t.Fatalf("body claims the password was used: %q", never)
				}
			}
		})
	}
}

func TestSendMFAInSessionLocked_NoSenderIsANoOp(t *testing.T) {
	if err := New(nil).SendMFAInSessionLocked(context.Background(), "a@example.test", "", "en"); err != nil {
		t.Fatalf("an unwired mailer must not fail the request: %v", err)
	}
}
