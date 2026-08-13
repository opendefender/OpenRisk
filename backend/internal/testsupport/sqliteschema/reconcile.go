// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package sqliteschema keeps hand-written sqlite test tables in step with the
// domain models they stand in for.
//
// THE PROBLEM IT SOLVES. Several models carry Postgres-only DDL — a
// `gen_random_uuid()` column default, `text[]` array columns — that GORM cannot
// emit for sqlite, so the tests that run against sqlite hand-wrote their CREATE
// TABLE statements. Each carried a comment asking the next person to keep it in
// sync with the model. Across `risks` and `assets` that promise was broken four
// separate times, and it always failed the same way: a change adds a field, and
// some unrelated test suite starts reporting `table X has no column named Y`
// from deep inside a repository INSERT, nowhere near the change that caused it.
//
// Reconcile removes the promise. A test creates whatever minimal table sqlite
// needs, then asks for the model's columns to be added. sqlite is dynamically
// typed, so an approximate affinity is all a fixture requires — what matters is
// that the column exists.
package sqliteschema

import (
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
)

// Reconcile adds every column declared by `model` that `table` does not already
// have. It never drops or alters an existing column, so a hand-written table can
// keep the definitions sqlite genuinely needs (primary keys, array stand-ins)
// while everything else follows the model.
func Reconcile(db *gorm.DB, table string, model any) error {
	existing, err := columnsOf(db, table)
	if err != nil {
		return err
	}

	// Parsed through gorm.Statement rather than by importing gorm.io/gorm/schema
	// directly: a direct import of that package makes `go mod tidy` pull in its
	// own test dependencies, which added an unrelated third-party module to
	// go.mod for no runtime benefit.
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return fmt.Errorf("parse %T: %w", model, err)
	}

	for _, f := range stmt.Schema.Fields {
		// Relations and computed (`gorm:"-"`) fields have no column.
		if f.DBName == "" || f.IgnoreMigration {
			continue
		}
		if existing[f.DBName] {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s", table, f.DBName, affinityFor(f.FieldType))
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("add %s.%s: %w", table, f.DBName, err)
		}
		existing[f.DBName] = true
	}
	return nil
}

func columnsOf(db *gorm.DB, table string) (map[string]bool, error) {
	rows, err := db.Raw(fmt.Sprintf("PRAGMA table_info(%q)", table)).Rows()
	if err != nil {
		return nil, fmt.Errorf("read %s columns: %w", table, err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notNull, pk int
			dflt        any
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan %s columns: %w", table, err)
		}
		out[name] = true
	}
	return out, rows.Err()
}

// affinityFor picks a permissive sqlite type. sqlite does not enforce column
// types; this only has to be close enough for the driver to scan a value back.
func affinityFor(rt reflect.Type) string {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt == reflect.TypeOf(time.Time{}) {
		return "DATETIME"
	}
	switch rt.Kind() {
	case reflect.Bool:
		return "BOOLEAN"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "INTEGER"
	case reflect.Float32, reflect.Float64:
		return "REAL"
	default:
		return "TEXT"
	}
}
