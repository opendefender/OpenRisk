// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// PasswordHashCensus counts stored password hashes by algorithm, to tell an
// operator how far the Argon2id migration (#484) still has to go.
//
// Cross-tenant by design, and the one query in this package that is: a user
// row is an identity that may belong to several organisations or none
// (users.tenant_id is NULL for system-wide accounts), and "how many legacy
// hashes are left in this database" has no per-tenant answer. It returns
// counts only — never a row, an id, an email or a hash — so nothing it produces
// can cross a tenant boundary.
type PasswordHashCensus struct {
	db *gorm.DB
	// classify maps a stored hash to its algorithm label. Injected rather than
	// imported because internal/auth already depends on this package; main
	// passes auth.HashAlgorithm, so the census and the login path can never
	// disagree about what counts as legacy.
	classify func(hashed string) string
}

// NewPasswordHashCensus builds a census over the users table.
func NewPasswordHashCensus(db *gorm.DB, classify func(hashed string) string) *PasswordHashCensus {
	return &PasswordHashCensus{db: db, classify: classify}
}

// Count returns the number of accounts per state ("active" / "deleted") and
// algorithm.
//
// Soft-deleted rows are included, under their own state: their hashes are
// still in the table and in every dump of it, and they will never sign in to
// be upgraded. Rows are streamed so a large user table is never held in memory.
func (c *PasswordHashCensus) Count(ctx context.Context) (map[string]map[string]int64, error) {
	rows, err := c.db.WithContext(ctx).
		Table("users").
		Select("password, deleted_at IS NOT NULL AS deleted").
		Rows()
	if err != nil {
		return nil, fmt.Errorf("password hash census: %w", err)
	}
	defer rows.Close()

	counts := map[string]map[string]int64{
		"active":  {},
		"deleted": {},
	}
	for rows.Next() {
		var (
			hashed  *string
			deleted bool
		)
		if err := rows.Scan(&hashed, &deleted); err != nil {
			return nil, fmt.Errorf("password hash census: %w", err)
		}
		state := "active"
		if deleted {
			state = "deleted"
		}
		value := ""
		if hashed != nil {
			value = *hashed
		}
		counts[state][c.classify(value)]++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("password hash census: %w", err)
	}
	return counts, nil
}
