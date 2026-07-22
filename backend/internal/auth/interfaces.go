// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

// PasswordHasher defines the interface for password hashing implementations
type PasswordHasher interface {
	// Hash hashes a password and returns the hash string and any error
	Hash(password string) (string, error)
	// Verify verifies a password against a hash and returns true if they match
	Verify(hashedPassword, plainPassword string) bool
}
