// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// D-065 (#879): the address hash is keyed, so a list of candidate addresses no
// longer reverses the audit trail.
func TestHashEmailForReset_IsKeyed(t *testing.T) {
	prev := configuredAddressHashKey.Load()
	t.Cleanup(func() { configuredAddressHashKey.Store(prev) })

	plain := sha256.Sum256([]byte("awa@example.test"))

	SetAddressHashKey([]byte("key-one-key-one-key-one-key-one!"))
	one := HashEmailForReset("awa@example.test")
	if one == hex.EncodeToString(plain[:]) {
		t.Fatal("still a plain SHA-256: anyone with a list of addresses can reverse it")
	}
	if len(one) != 64 {
		t.Fatalf("hash length %d, the email_hash column holds 64", len(one))
	}
	if got := HashEmailForReset("  AWA@Example.test "); got != one {
		t.Fatal("spellings of one address must hash alike, or the per-address limits are bypassed")
	}

	SetAddressHashKey([]byte("key-two-key-two-key-two-key-two!"))
	if HashEmailForReset("awa@example.test") == one {
		t.Fatal("the key does not change the hash")
	}
}

func TestSetAddressHashKey_CopiesTheKey(t *testing.T) {
	prev := configuredAddressHashKey.Load()
	t.Cleanup(func() { configuredAddressHashKey.Store(prev) })

	key := []byte("key-one-key-one-key-one-key-one!")
	SetAddressHashKey(key)
	before := HashEmailForReset("awa@example.test")
	key[0] = 'X'
	if HashEmailForReset("awa@example.test") != before {
		t.Fatal("mutating the caller's slice changed the installed key")
	}
}
