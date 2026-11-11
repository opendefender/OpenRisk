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

// #1001: the notice says what was deleted, why, and the real path to new codes.
func TestSendMFABackupCodesPurged_SaysWhatHappenedInBothLanguages(t *testing.T) {
	cases := []struct {
		locale, subject string
		mustSay         []string
	}{
		// #1032: the path is the regenerate button, not turning MFA off, which
		// a role that requires MFA cannot do.
		{"en", "backup codes", []string{"the same codes", "Settings › Security", "Regenerate backup codes", "authenticator app still works"}},
		{"fr", "codes de secours", []string{"les mêmes codes", "Paramètres › Sécurité", "Régénérer les codes de secours", "fonctionne toujours"}},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			s := &capturingSender{}
			if err := New(s).SendMFABackupCodesPurged(context.Background(), "awa@example.test", "Awa Diallo", tc.locale); err != nil {
				t.Fatal(err)
			}
			if s.to != "awa@example.test" {
				t.Fatalf("sent to %q", s.to)
			}
			if !strings.Contains(s.subject, tc.subject) {
				t.Fatalf("subject %q lacks %q", s.subject, tc.subject)
			}
			for _, want := range append([]string{"Awa Diallo"}, tc.mustSay...) {
				if !strings.Contains(s.body, want) {
					t.Fatalf("body lacks %q", want)
				}
			}
		})
	}
}

func TestSendMFABackupCodesPurged_NoSenderIsANoOp(t *testing.T) {
	if err := New(nil).SendMFABackupCodesPurged(context.Background(), "a@example.test", "", "en"); err != nil {
		t.Fatalf("an unwired mailer must not fail the boot: %v", err)
	}
}

// #1032: the regeneration notice says the old codes are dead and what to do if
// it was not the owner.
func TestSendMFABackupCodesRegenerated_SaysWhatHappenedInBothLanguages(t *testing.T) {
	cases := []struct {
		locale, subject string
		mustSay         []string
	}{
		{"en", "backup codes", []string{"previous backup codes no longer work", "someone holds both"}},
		{"fr", "codes de secours", []string{"anciens codes de secours ne fonctionnent plus", "détient votre application"}},
	}
	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			s := &capturingSender{}
			if err := New(s).SendMFABackupCodesRegenerated(context.Background(), "awa@example.test", "Awa Diallo", tc.locale); err != nil {
				t.Fatal(err)
			}
			if s.to != "awa@example.test" {
				t.Fatalf("sent to %q", s.to)
			}
			if !strings.Contains(s.subject, tc.subject) {
				t.Fatalf("subject %q lacks %q", s.subject, tc.subject)
			}
			for _, want := range append([]string{"Awa Diallo"}, tc.mustSay...) {
				if !strings.Contains(s.body, want) {
					t.Fatalf("body lacks %q", want)
				}
			}
		})
	}
}

func TestSendMFABackupCodesRegenerated_NoSenderIsANoOp(t *testing.T) {
	if err := New(nil).SendMFABackupCodesRegenerated(context.Background(), "a@example.test", "", "en"); err != nil {
		t.Fatalf("an unwired mailer must not fail: %v", err)
	}
}
