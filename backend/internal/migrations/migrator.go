// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package migrations

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// newMigrate builds a migrate handle from DATABASE_URL and MIGRATIONS_DIR
// (default "migrations", relative to the process working directory). The caller
// owns the returned handle and must Close it.
func newMigrate() (*migrate.Migrate, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errNoDatabaseURL
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	source := "file://" + dir
	m, err := migrate.New(source, dbURL)
	if err != nil {
		return nil, fmt.Errorf("migrations: initialize from %q: %w", source, err)
	}
	return m, nil
}

// errNoDatabaseURL is a sentinel so callers can distinguish "nothing to do"
// (no DATABASE_URL) from a real failure.
var errNoDatabaseURL = errors.New("migrations: DATABASE_URL not set")

// dirtyStateError renders an actionable operator message for a database left in
// golang-migrate's "dirty" state — a previous migration that failed mid-apply.
// The bare library error ("Dirty database version N. Fix and force version.")
// tells an operator nothing about how to recover, so the boot appeared to crash
// for no reason. This spells out the exact recovery path.
func dirtyStateError(version uint) error {
	return fmt.Errorf(`migrations: database is DIRTY at version %d — a previous migration failed mid-apply, so the schema state is unknown.

Recovery:
  1. Inspect the failed migration file (migrations/%04d_*.up.sql) and the DB, and finish or undo whatever it left half-done.
  2. Mark the last KNOWN-GOOD version as the current one:
        make migrate-force VERSION=<good_version>
     (or: cd backend && DATABASE_URL=... go run ./cmd/server migrate force <good_version>)
  3. Re-run the migrations / re-boot:
        make migrate

Boot aborts here on purpose rather than serving on a half-applied schema. See docs/runbooks/migrations.md`, version, version)
}

// RunMigrations applies the additive SQL migration layer on top of the schema
// GORM AutoMigrate has already built (AutoMigrate is the declared schema
// authority; this layer carries only what AutoMigrate cannot express — triggers,
// CHECK constraints, one-off data backfills). It FAILS LOUDLY: a real apply
// error aborts the boot rather than serving on a half-applied schema. The only
// non-errors are "no DATABASE_URL" and migrate.ErrNoChange.
//
// A DIRTY database is reported with an actionable recovery message
// (dirtyStateError) instead of the opaque library string.
func RunMigrations() error {
	m, err := newMigrate()
	if errors.Is(err, errNoDatabaseURL) {
		log.Println("migrations: DATABASE_URL not set — skipping SQL migration layer")
		return nil
	}
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	// Detect a dirty state up front so the operator gets the recovery path, not
	// the raw "Dirty database version N" string buried in an Up() failure.
	if v, dirty, verr := m.Version(); verr == nil && dirty {
		return dirtyStateError(v)
	} else if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
		return fmt.Errorf("migrations: read current version: %w", verr)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("migrations: already up to date")
			return nil
		}
		var dirtyErr migrate.ErrDirty
		if errors.As(err, &dirtyErr) {
			return dirtyStateError(uint(dirtyErr.Version))
		}
		return fmt.Errorf("migrations: apply: %w", err)
	}

	if v, dirty, verr := m.Version(); verr == nil {
		log.Printf("migrations: applied successfully (version=%d dirty=%t)", v, dirty)
	} else {
		log.Println("migrations: applied successfully")
	}
	return nil
}

// RunCLI implements the `migrate` subcommand of the server binary so that
// `make migrate`, `make migrate-rollback` and `make migrate-force` are real
// operations rather than (as before) a full server boot that ignored its args.
//
// Usage: <server> migrate <up|down|force|version>
//
//	up            apply all pending migrations
//	down          roll back exactly one migration
//	force <ver>   set the version and CLEAR the dirty flag (recovery)
//	version       print current version and dirty flag
//
// Returns nil on success so the caller can os.Exit(0).
func RunCLI(args []string) error {
	if len(args) == 0 {
		return errors.New("migrate: expected a subcommand: up | down | force <version> | version")
	}
	m, err := newMigrate()
	if errors.Is(err, errNoDatabaseURL) {
		return errors.New("migrate: DATABASE_URL not set")
	}
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate up: %w", err)
		}
		printVersion(m)
		return nil
	case "down":
		// One step only: a full down-migration is destructive and must be explicit.
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down: %w", err)
		}
		printVersion(m)
		return nil
	case "force":
		if len(args) < 2 {
			return errors.New("migrate force: expected a version number, e.g. `migrate force 47`")
		}
		v, convErr := strconv.Atoi(args[1])
		if convErr != nil {
			return fmt.Errorf("migrate force: invalid version %q: %w", args[1], convErr)
		}
		if err := m.Force(v); err != nil {
			return fmt.Errorf("migrate force %d: %w", v, err)
		}
		log.Printf("migrate: forced version to %d (dirty flag cleared)", v)
		return nil
	case "version":
		printVersion(m)
		return nil
	default:
		return fmt.Errorf("migrate: unknown subcommand %q (want up | down | force <version> | version)", args[0])
	}
}

func printVersion(m *migrate.Migrate) {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		log.Println("migrate: no migrations applied yet (version=nil)")
		return
	}
	if err != nil {
		log.Printf("migrate: could not read version: %v", err)
		return
	}
	log.Printf("migrate: version=%d dirty=%t", v, dirty)
}

func closeMigrate(m *migrate.Migrate) {
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		log.Printf("migrations: close: source=%v database=%v", srcErr, dbErr)
	}
}
