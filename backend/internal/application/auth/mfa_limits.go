// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"time"
)

// MFAAttemptStore holds the counters and locks that bound guessing at the MFA
// challenge (#689). It must be shared across instances: a per-instance counter
// lets an attacker multiply the budget by the number of replicas.
type MFAAttemptStore interface {
	// Increment adds one to key and returns the new count. The key expires ttl
	// after its first increment; later increments do not move that deadline.
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// Count returns the current value of key, 0 when it does not exist.
	Count(ctx context.Context, key string) (int64, error)
	// Lock sets key for ttl unless it is already set, and reports whether this
	// call set it. Only the call that sets the lock audits it and mails the
	// owner, so a burst of refused attempts produces one notice, not fifty.
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// LockedFor returns how long the lock on key still holds, 0 when unlocked.
	LockedFor(ctx context.Context, key string) (time.Duration, error)
	// Delete removes key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
}

// TokenRevoker blacklists a token's JTI until the token would have expired
// anyway. *pkg/auth.TokenBlacklistManager satisfies it.
type TokenRevoker interface {
	BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error
}
