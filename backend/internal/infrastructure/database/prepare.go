// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package database

import (
	"fmt"

	"gorm.io/gorm"
)

// PrepareForAutoMigrate performs the data repairs that AutoMigrate cannot do
// for itself, and MUST run before it.
//
// WHY THIS EXISTS. AutoMigrate creates the indexes declared by the model tags,
// but it will not touch row data. When a model gains a UNIQUE index over a
// column that already exists and is populated — as audit_events did when the
// hash chain introduced `sequence` — AutoMigrate tries to create the index,
// Postgres rejects it because the legacy rows collide, and the whole boot
// fails with "duplicated key not allowed". The server then refuses to start
// against any database that has pre-existing data.
//
// The SQL migrations do handle this (0048 backfills sequences before creating
// the index). But AutoMigrate runs on every boot regardless of whether
// golang-migrate has been able to apply them, so a deployment whose migration
// chain is behind — or blocked — cannot come up at all. That is a bad failure:
// the operator sees a database error, not the migration problem behind it.
//
// Everything here must be IDEMPOTENT and safe to run on a fresh database.
func PrepareForAutoMigrate(db *gorm.DB) error {
	if err := backfillAuditSequences(db); err != nil {
		return fmt.Errorf("audit_events: %w", err)
	}
	if err := backfillRefreshTokenFamilies(db); err != nil {
		return fmt.Errorf("refresh_tokens: %w", err)
	}
	return nil
}

// backfillRefreshTokenFamilies gives every pre-existing refresh token a family
// lineage before AutoMigrate makes family_id NOT NULL.
//
// The reuse-detection work (W0-03) adds family_id (NOT NULL) and rotated_at to
// refresh_tokens. AutoMigrate would try `ADD family_id uuid NOT NULL` on a table
// whose existing rows have no value and fail with 23502. We add the columns
// nullable first and seed each live session as its own family (family_id = id),
// so AutoMigrate's later SET NOT NULL succeeds. Mirrors
// migrations/0057_refresh_token_family_rotation.up.sql. Idempotent; a no-op on a
// fresh database where the table does not exist yet.
func backfillRefreshTokenFamilies(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	if !db.Migrator().HasTable("refresh_tokens") {
		return nil
	}
	stmts := []string{
		`ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS family_id uuid`,
		`ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS rotated_at timestamptz`,
		`UPDATE refresh_tokens SET family_id = id WHERE family_id IS NULL`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("prepare family_id: %w", err)
		}
	}
	return nil
}

// backfillAuditSequences numbers the audit entries written before the hash
// chain existed, so the unique (tenant_id, sequence) index can be created.
//
// Legacy rows are numbered per tenant in creation order and marked
// source='legacy'. They are NOT given hashes: chain verification must be able
// to tell "never chained" apart from "chain broken", and back-filling plausible
// hashes would be the one lie an audit trail cannot tell. This mirrors
// migrations/0048_audit_trail_hash_chain.up.sql exactly.
func backfillAuditSequences(db *gorm.DB) error {
	// Nothing to do when the table or the column is not there yet — a fresh
	// database creates both correctly during AutoMigrate.
	if !db.Migrator().HasTable("audit_events") || !db.Migrator().HasColumn("audit_events", "sequence") {
		return nil
	}

	var unnumbered int64
	if err := db.Raw(`SELECT COUNT(*) FROM audit_events WHERE sequence = 0`).Scan(&unnumbered).Error; err != nil {
		return fmt.Errorf("count unnumbered entries: %w", err)
	}
	if unnumbered == 0 {
		return nil
	}

	// `source` arrived with the same change; tolerate its absence so this works
	// against a database sitting between the two states.
	setSource := ""
	if db.Migrator().HasColumn("audit_events", "source") {
		setSource = ", source = COALESCE(e.source, 'legacy')"
	}

	stmt := fmt.Sprintf(`
		WITH numbered AS (
			SELECT id,
			       ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY created_at, id) AS seq
			FROM audit_events
			WHERE sequence = 0
		)
		UPDATE audit_events e
		SET sequence = numbered.seq%s
		FROM numbered
		WHERE e.id = numbered.id`, setSource)

	// This legacy backfill normally runs before the append-only trigger (0055)
	// exists, but wrap it in a maintenance-authorised transaction so it can never
	// be blocked by the trigger if the two ever coexist. SET LOCAL is tx-scoped.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SET LOCAL openrisk.audit_maintenance = 'on'").Error; err != nil {
				return err
			}
		}
		return tx.Exec(stmt).Error
	}); err != nil {
		return fmt.Errorf("backfill sequences for %d legacy entries: %w", unnumbered, err)
	}
	return nil
}
