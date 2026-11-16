// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package testdb

import "gorm.io/gorm"

// Probe is a non-test file that issues an unscoped query on a table of db.
// testdb_test.go uses it to prove the guard still sees production callers:
// if frame resolution ever breaks, the suite turns red instead of green.
func Probe(db *gorm.DB, model any) error {
	return db.Find(model).Error
}
