// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"fmt"
	"strings"
)

// minAuditHashKeyLength is the shortest AUDIT_HASH_KEY production accepts. The
// installer generates 32 random bytes in base64 (44 characters).
const minAuditHashKeyLength = 32

// auditHashKey reads the key that hashes addresses in the auth audit trail and
// the per-address limits (D-065, #879).
//
// Production refuses to boot without one, naming the variable: the fallback
// key is in the public source, so a trail hashed with it can be reversed like
// the plain SHA-256 it replaces. Elsewhere the domain's development key is used
// and the caller is told to say so.
func auditHashKey(getenv func(string) string, production bool) (key []byte, warning string, err error) {
	raw := strings.TrimSpace(getenv("AUDIT_HASH_KEY"))
	switch {
	case raw == "" && production:
		return nil, "", fmt.Errorf("AUDIT_HASH_KEY is required in production: set it to a random secret of at least %d characters (for example `openssl rand -base64 32`), the same on every replica", minAuditHashKeyLength)
	case raw == "":
		return nil, "Warning: AUDIT_HASH_KEY not set — addresses in the auth audit trail are hashed with a public development key. Set it in production.", nil
	case production && len(raw) < minAuditHashKeyLength:
		return nil, "", fmt.Errorf("AUDIT_HASH_KEY is too short: %d characters, production needs at least %d", len(raw), minAuditHashKeyLength)
	}
	return []byte(raw), "", nil
}
