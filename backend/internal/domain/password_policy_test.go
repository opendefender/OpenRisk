// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestValidatePassword_Accepts(t *testing.T) {
	valid := []struct {
		name     string
		password string
	}{
		{"long passphrase with digit", "correct horse battery staple 7"},
		{"mixed classes", "Tr0ubador-Sunset"},
		{"exactly twelve", "Abcdefgh123!"},
		{"unicode counted as runes", "Éléphant-Bleu42"},
		{"symbols heavy", "!!Zebra-Crossing-9"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePassword(tc.password); err != nil {
				t.Errorf("ValidatePassword(%q) rejected a good password: %v", tc.password, err)
			}
		})
	}
}

func TestValidatePassword_RejectsTooShort(t *testing.T) {
	// Eleven characters, all four classes — previously accepted at the old
	// 8-character floor. This is the regression that finding F-05 is about.
	err := ValidatePassword("Abcdefg123!")
	if err == nil {
		t.Fatal("11-character password should be rejected under the 12-character policy")
	}
	if !strings.Contains(err.Error(), "12") {
		t.Errorf("error should state the required length, got %q", err.Error())
	}
}

func TestValidatePassword_RejectsEmpty(t *testing.T) {
	if err := ValidatePassword(""); err == nil {
		t.Fatal("empty password should be rejected")
	}
}

func TestValidatePassword_RequiresThreeCharacterClasses(t *testing.T) {
	cases := []struct {
		name     string
		password string
	}{
		{"lowercase only", "abcdefghijklmnop"},
		{"uppercase only", "ABCDEFGHIJKLMNOP"},
		{"digits only", "1234567890123456"},
		{"two classes only", "abcdefghijkl1234"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if err == nil {
				t.Fatalf("ValidatePassword(%q) should require more character classes", tc.password)
			}
			if !strings.Contains(err.Error(), "three") {
				t.Errorf("error should explain the class requirement, got %q", err.Error())
			}
		})
	}
}

// TestValidatePassword_RejectsDecoratedCommonPasswords is the interesting case:
// these all clear the length and class bars, so without the blocklist they would
// sail through — yet they are the first candidates any cracking dictionary tries.
func TestValidatePassword_RejectsDecoratedCommonPasswords(t *testing.T) {
	weak := []string{
		"P@ssw0rd1234",
		"Password1234!",
		"Welcome12345!",
		"ChangeMe1234!",
		"MotDePasse12!",
		"Adm1nistrator!",
		"OpenRisk1234!",
	}

	for _, password := range weak {
		t.Run(password, func(t *testing.T) {
			if err := ValidatePassword(password); err == nil {
				t.Errorf("ValidatePassword(%q) accepted a well-known weak password", password)
			}
		})
	}
}

// TestValidatePassword_BlocklistDoesNotOverreach guards the other direction: a
// legitimate password merely containing a blocklisted word must still pass, or
// the rule becomes an arbitrary annoyance.
func TestValidatePassword_BlocklistDoesNotOverreach(t *testing.T) {
	fine := []string{
		"Zebra-Password-Vault-9", // contains "password" but not led by it
		"MyDragonflyGarden42",    // contains "dragon" mid-string
	}

	for _, password := range fine {
		t.Run(password, func(t *testing.T) {
			if err := ValidatePassword(password); err != nil {
				t.Errorf("ValidatePassword(%q) rejected a reasonable password: %v", password, err)
			}
		})
	}
}

func TestValidatePassword_ReturnsValidationError(t *testing.T) {
	err := ValidatePassword("short")
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected *AppError so handlers map it to 400, got %T", err)
	}
	if !errors.Is(appErr, ErrValidation) {
		t.Errorf("expected a validation error, got %v", appErr.Err)
	}
	if appErr.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400, got %d", appErr.Code)
	}
}
