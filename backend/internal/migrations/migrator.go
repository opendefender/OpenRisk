// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package migrations

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies the additive SQL migration layer on top of the schema
// GORM AutoMigrate has already built (see cmd/server/main.go: AutoMigrate is the
// declared schema authority; this layer carries only what AutoMigrate cannot
// express — triggers, CHECK constraints, and one-off data backfills). It reads
// the directory named by MIGRATIONS_DIR (default "migrations", relative to the
// process working directory) and expects DATABASE_URL in the
// postgres://user:pass@host:port/dbname?sslmode=... form.
//
// It FAILS LOUDLY: a real initialization or apply error is returned so the
// caller can abort the boot. A boot that silently continued on a half-applied
// schema — the previous behaviour, which swallowed every error into a log line —
// is exactly how a production database ends up in an unknowable state. The only
// non-errors are "no DATABASE_URL" (nothing to do) and migrate.ErrNoChange
// (already up to date).
func RunMigrations() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("migrations: DATABASE_URL not set — skipping SQL migration layer")
		return nil
	}

	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	source := "file://" + dir

	m, err := migrate.New(source, dbURL)
	if err != nil {
		return fmt.Errorf("migrations: initialize from %q: %w", source, err)
	}
	defer func() {
		// Close releases the source and database handles; report but do not mask.
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("migrations: close: source=%v database=%v", srcErr, dbErr)
		}
	}()

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("migrations: already up to date")
			return nil
		}
		return fmt.Errorf("migrations: apply from %q: %w", source, err)
	}

	if v, dirty, verr := m.Version(); verr == nil {
		log.Printf("migrations: applied successfully (version=%d dirty=%t)", v, dirty)
	} else {
		log.Println("migrations: applied successfully")
	}
	return nil
}
