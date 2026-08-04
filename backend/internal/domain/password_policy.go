// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"unicode"
)

// MinPasswordLength is the enforced minimum.
//
// Audit finding F-05: the code enforced 8 characters while the README promised
// 12. Roadmap V6 §7.2 (ticket OR-006) settles the discrepancy at 12 — a security
// tool that accepts 8 loses credibility at the first customer audit.
const MinPasswordLength = 12

// requiredCharacterClasses is how many of {lowercase, uppercase, digit, symbol}
// a password must draw from.
//
// Three of four rather than all four: demanding every class is a well-documented
// way to push people toward predictable shapes like "Password1!" — the mandate
// gets satisfied by a capital at the front and a bang at the end. Three classes
// at twelve characters raises the floor without inviting that pattern.
const requiredCharacterClasses = 3

// commonWeakPasswords is a small blocklist of shapes that satisfy the length and
// class rules yet fall in seconds to any real cracking dictionary.
//
// This is a deliberate stopgap, not a substitute for strength estimation. Proper
// coverage means zxcvbn scoring and a HaveIBeenPwned k-anonymity lookup (both
// still open in OR-006); this list only closes the most embarrassing cases in
// the meantime.
var commonWeakPasswords = map[string]struct{}{
	"password":      {},
	"passw0rd":      {},
	"welcome":       {},
	"qwerty":        {},
	"azerty":        {},
	"letmein":       {},
	"admin":         {},
	"administrator": {},
	"changeme":      {},
	"motdepasse":    {},
	"iloveyou":      {},
	"monkey":        {},
	"dragon":        {},
	"sunshine":      {},
	"princess":      {},
	"football":      {},
	"baseball":      {},
	"trustno1":      {},
	"openrisk":      {},
	"opendefender":  {},
}

// ValidatePassword enforces the password policy.
//
// Returns a typed ValidationError whose message is actionable: telling someone
// their password is "invalid" without saying what is missing just produces
// repeated failed attempts.
func ValidatePassword(password string) error {
	if password == "" {
		return NewValidationError("password is required")
	}

	// Count runes, not bytes: "Ω" and "é" are one character to a user, several
	// bytes to Go, and len() would credit them as extra length.
	if len([]rune(password)) < MinPasswordLength {
		return NewValidationError("password must be at least 12 characters")
	}

	if classes := countCharacterClasses(password); classes < requiredCharacterClasses {
		return NewValidationError("password must combine at least three of: lowercase letters, uppercase letters, digits, symbols")
	}

	if isCommonWeakPassword(password) {
		return NewValidationError("password is too common — choose something less predictable")
	}

	return nil
}

func countCharacterClasses(password string) int {
	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			// Anything else — punctuation, symbols, spaces — counts as a symbol.
			hasSymbol = true
		}
	}

	classes := 0
	for _, present := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if present {
			classes++
		}
	}
	return classes
}

// isCommonWeakPassword matches the blocklist against the password stripped of
// the decoration people add to satisfy complexity rules. "P@ssw0rd123!" has to
// be recognised as "password", otherwise the blocklist is trivially sidestepped.
func isCommonWeakPassword(password string) bool {
	normalised := normaliseForBlocklist(password)
	if normalised == "" {
		return false
	}
	if _, blocked := commonWeakPasswords[normalised]; blocked {
		return true
	}

	// Also catch a blocklisted word padded to reach the length requirement, e.g.
	// "password" + "1234" — the padding adds no meaningful entropy.
	for weak := range commonWeakPasswords {
		if len(weak) >= 6 && strings.HasPrefix(normalised, weak) {
			return true
		}
	}
	return false
}

// leetReplacements maps the common character substitutions back to letters.
var leetReplacements = strings.NewReplacer(
	"@", "a", "4", "a",
	"3", "e",
	"1", "i", "!", "i",
	"0", "o",
	"$", "s", "5", "s",
	"7", "t",
)

func normaliseForBlocklist(password string) string {
	lowered := strings.ToLower(password)
	deleeted := leetReplacements.Replace(lowered)

	// Drop everything that is not a letter so trailing digits and symbols cannot
	// disguise the underlying word.
	var b strings.Builder
	for _, r := range deleeted {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
