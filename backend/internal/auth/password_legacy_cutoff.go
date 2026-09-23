// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// The end of the legacy password hash.
//
// Transparent upgrade only reaches accounts whose owner signs in. Accounts that
// never sign in would keep an unsalted SHA-256 digest in the database forever,
// and "forever" is the word that makes the whole migration worthless: the risk
// a weak hash carries is the risk of the database leaking, which does not care
// whether the account is active.
//
// So the migration has an end date. Before it, a legacy hash is accepted once
// and replaced. After it, the hash is refused and the account has to go through
// password reset, which writes Argon2id like every other write path.

// DefaultLegacyHashCutoffRFC3339 is the date after which a legacy hash stops
// being accepted: 2026-12-31T23:59:59Z, about three months after the upgrade
// shipped. Long enough for quarterly and seasonal users to sign in at least
// once, short enough to be a deadline rather than an intention.
const DefaultLegacyHashCutoffRFC3339 = "2026-12-31T23:59:59Z"

// LegacyHashCutoff resolves the cutoff, honouring PASSWORD_LEGACY_HASH_CUTOFF
// (RFC 3339) when it is set.
//
// An operator with a fleet of seasonal accounts can push the date out; an
// operator who has finished migrating can pull it in and shut the door early. A
// malformed value is an error rather than a silent fallback — a cutoff that
// quietly reverts to the default is how a deadline gets missed by six months.
func LegacyHashCutoff() (time.Time, error) {
	raw := strings.TrimSpace(os.Getenv("PASSWORD_LEGACY_HASH_CUTOFF"))
	if raw == "" {
		raw = DefaultLegacyHashCutoffRFC3339
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("PASSWORD_LEGACY_HASH_CUTOFF: %q is not an RFC 3339 timestamp", raw)
	}
	return t.UTC(), nil
}

// LegacyHashCutoffOrDefault is LegacyHashCutoff for call sites with nowhere to
// put an error — chiefly the login use case, which must have a deadline whether
// or not the deployment configured one.
//
// A malformed value still fails loudly, but at startup: main reads
// LegacyHashCutoff directly and refuses to boot on an error. What this function
// guarantees is the other half — that no construction path can end up with no
// deadline at all.
func LegacyHashCutoffOrDefault() time.Time {
	if t, err := LegacyHashCutoff(); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, DefaultLegacyHashCutoffRFC3339)
	return t.UTC()
}

// LegacyHashExpired reports whether a legacy hash may still be used to sign in.
//
// Pure, so the deadline behaviour is testable without waiting for December.
func LegacyHashExpired(hashed string, now, cutoff time.Time) bool {
	return IsLegacyHash(hashed) && now.After(cutoff)
}
