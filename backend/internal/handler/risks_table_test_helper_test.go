// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"testing"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// createRisksTable builds the `risks` table for a sqlite test database.
//
// Three test files in this package each hand-wrote a CREATE TABLE "kept in sync
// with domain.Risk", and it drifted three separate times — each time surfacing
// as a baffling `table risks has no column named X` from inside a repository
// INSERT, nowhere near the field somebody had added. This creates only what
// sqlite genuinely needs and lets sqliteschema.Reconcile take the rest off the
// model, so adding a field to domain.Risk cannot break these tests again.
func createRisksTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	const base = `CREATE TABLE IF NOT EXISTS risks (
		id TEXT PRIMARY KEY,
		deleted_at DATETIME
	)`
	if err := db.Exec(base).Error; err != nil {
		t.Fatalf("create risks table: %v", err)
	}
	if err := sqliteschema.Reconcile(db, "risks", &domain.Risk{}); err != nil {
		t.Fatalf("reconcile risks table with domain.Risk: %v", err)
	}
}
