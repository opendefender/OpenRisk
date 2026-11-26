// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/opendefender/openrisk/internal/domain"
)

// minRSAKeyBits is the smallest JWT signing key production accepts.
const minRSAKeyBits = 2048

// knownDevPlaceholders are values that mean "nobody set this": the public
// development keys and the placeholders people paste from examples. Compared
// case-insensitively after trimming; any value CONTAINING change-me or
// changeme is refused as well (see isKnownDevDefault).
var knownDevPlaceholders = []string{
	"change-me", "changeme", "secret", "test", "password", "your-api-key",
	devMFAEncryptionKey,
}

// isKnownDevDefault reports whether v is a public development value or an
// example placeholder. Values used by CI and compose files (APP_ENV=test) are
// not production and are not listed here.
func isKnownDevDefault(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	if domain.IsDevAddressHashKey(strings.TrimSpace(v)) {
		return true
	}
	for _, p := range knownDevPlaceholders {
		if t == strings.ToLower(p) {
			return true
		}
	}
	return strings.Contains(t, "change-me") || strings.Contains(t, "changeme")
}

// checkProductionSecrets is the one production gate (#1317). Outside
// production it refuses nothing. In production it returns ONE error naming
// EVERY variable that is empty, too weak or a known development default, so an
// operator fixes them in a single pass. The message names variables and rules,
// never a value.
//
// SCANNER_CREDENTIAL_KEY is deliberately absent: #1200 owns its production
// refusal (scanner_key_wiring.go) and its rotation.
//
// The per-secret readers (mfaEncryptionKey, auditHashKey, trustedDeviceSigner)
// keep their own checks, which run later with the same rules as a backstop.
func checkProductionSecrets(getenv func(string) string, readFile func(string) ([]byte, error), production bool) error {
	if !production {
		return nil
	}
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	val := func(name string) string { return strings.TrimSpace(getenv(name)) }

	// JWT RS256 private key: file first, then inline, as pkg/auth loads it.
	if msg := checkRSAPrivateKey(val("RSA_PRIVATE_KEY_PATH"), getenv("RSA_PRIVATE_KEY"), readFile); msg != "" {
		add("%s", msg)
	}

	// Required.
	if v := val("MFA_ENCRYPTION_KEY"); v == "" {
		add("MFA_ENCRYPTION_KEY is required (a random secret, e.g. `openssl rand -hex 32`)")
	} else if isKnownDevDefault(v) {
		add("MFA_ENCRYPTION_KEY is a known development default or placeholder")
	}
	if v := val("AUDIT_HASH_KEY"); v == "" {
		add("AUDIT_HASH_KEY is required (at least %d characters, e.g. `openssl rand -base64 32`)", minAuditHashKeyLength)
	} else if isKnownDevDefault(v) {
		add("AUDIT_HASH_KEY is a known development default or placeholder")
	} else if len(v) < minAuditHashKeyLength {
		add("AUDIT_HASH_KEY is too short: %d characters, production needs at least %d", len(v), minAuditHashKeyLength)
	}

	// Optional: unset is fine, set must not be a development default.
	for _, name := range []string{"DEVICE_COOKIE_KEY", "DEVICE_COOKIE_PREVIOUS_KEY", "AUDIT_EXPORT_KEY", "MFA_ENCRYPTION_PREVIOUS_KEY"} {
		if v := val(name); v != "" && isKnownDevDefault(v) {
			add("%s is a known development default or placeholder (leave it unset instead)", name)
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("refusing to start in production, %d secret(s) unusable:\n  - %s", len(problems), strings.Join(problems, "\n  - "))
}

// checkRSAPrivateKey returns "" when the key is usable, else a reason that
// never contains key material.
func checkRSAPrivateKey(path, inline string, readFile func(string) ([]byte, error)) string {
	var pem string
	switch {
	case path != "":
		data, err := readFile(path)
		if err != nil {
			return "RSA_PRIVATE_KEY_PATH cannot be read"
		}
		pem = string(data)
	case strings.TrimSpace(inline) != "":
		pem = inline
	default:
		return "RSA_PRIVATE_KEY_PATH or RSA_PRIVATE_KEY is required (the RS256 JWT signing key)"
	}
	if !strings.Contains(pem, "-----BEGIN") {
		return "the RS256 JWT private key is a placeholder or not PEM (the example value of .env.example was left in)"
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(pem))
	if err != nil {
		return "the RS256 JWT private key is not a valid RSA private key"
	}
	return rsaBitsProblem(key)
}

func rsaBitsProblem(key *rsa.PrivateKey) string {
	if bits := key.N.BitLen(); bits < minRSAKeyBits {
		return fmt.Sprintf("the RS256 JWT private key is %d bits, production needs at least %d", bits, minRSAKeyBits)
	}
	return ""
}
