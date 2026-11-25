// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"crypto/sha256"
	"errors"
	"strings"
)

// devMFAEncryptionKey is what a server started without MFA_ENCRYPTION_KEY
// encrypts TOTP secrets with. It is in the public source, so production
// refuses it (#1062), and a server given a real key re-encrypts whatever was
// left under it (authmfa.RekeySecrets).
const devMFAEncryptionKey = "openrisk-dev-mfa-encryption-key-change-me"

// mfaEncryptionKey reads the key that encrypts TOTP secrets at rest.
//
// No minimum length, unlike AUDIT_HASH_KEY: the value is hashed to 32 bytes,
// and an install already running on a short key would lose every enrolment if
// it had to change it.
func mfaEncryptionKey(getenv func(string) string, production bool) (raw, warning string, err error) {
	raw = strings.TrimSpace(getenv("MFA_ENCRYPTION_KEY"))
	switch {
	case (raw == "" || raw == devMFAEncryptionKey) && production:
		return "", "", errors.New("MFA_ENCRYPTION_KEY is required in production: set it to a random secret (for example `openssl rand -hex 32`), the same on every replica and never changed afterwards. Secrets encrypted without it are re-encrypted with it at boot; nobody has to enrol again")
	case raw == "":
		return devMFAEncryptionKey, "Warning: MFA_ENCRYPTION_KEY not set — two-factor secrets are encrypted with a public development key. Set it in production.", nil
	}
	return raw, "", nil
}

// mfaPreviousKey reads MFA_ENCRYPTION_PREVIOUS_KEY (#1317): the key being
// rotated away from. ok is false when it is unset, equals the current key
// (nothing to move) or is the public dev key (already handled by the dev-key
// pass). The 32-byte AES key is derived like the current one.
func mfaPreviousKey(getenv func(string) string, currentRaw string) (key []byte, ok bool) {
	raw := strings.TrimSpace(getenv("MFA_ENCRYPTION_PREVIOUS_KEY"))
	if raw == "" || raw == currentRaw || raw == devMFAEncryptionKey {
		return nil, false
	}
	k := sha256.Sum256([]byte(raw))
	return k[:], true
}
