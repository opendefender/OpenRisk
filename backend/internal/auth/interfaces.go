// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

// PasswordHasher defines the interface for password hashing implementations
type PasswordHasher interface {
	// Hash hashes a password and returns the hash string and any error
	Hash(password string) (string, error)
	// Verify verifies a password against a hash and returns true if they match
	Verify(hashedPassword, plainPassword string) bool
}
