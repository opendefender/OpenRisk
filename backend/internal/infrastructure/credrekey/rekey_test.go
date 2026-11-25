// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package credrekey

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/testsupport/testdb"
	"github.com/opendefender/openrisk/pkg/crypto"
)

// Only the columns the pass reads and writes: id, tenant_id, the ciphertext.
func rekeyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range Columns {
		if err := db.Exec(`CREATE TABLE ` + c.Table + ` (id TEXT PRIMARY KEY, tenant_id TEXT, ` + c.Column + ` TEXT)`).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func keyOf(t *testing.T, s string) []byte {
	t.Helper()
	k, err := crypto.DeriveKey([]byte(s), nil)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func add(t *testing.T, db *gorm.DB, c Column, value string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO `+c.Table+` (id, tenant_id, `+c.Column+`) VALUES (?, ?, ?)`, id, uuid.New(), value).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func addEnc(t *testing.T, db *gorm.DB, c Column, plain string, key []byte) uuid.UUID {
	t.Helper()
	enc, err := crypto.EncryptAES256GCM(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	return add(t, db, c, enc)
}

func stored(t *testing.T, db *gorm.DB, c Column, id uuid.UUID) string {
	t.Helper()
	var v []string
	if err := db.Raw(`SELECT `+c.Column+` FROM `+c.Table+` WHERE id = ?`, id).Scan(&v).Error; err != nil || len(v) != 1 {
		t.Fatalf("read %s %s: %v", c.Table, id, err)
	}
	return v[0]
}

// #1200: every column encrypted with SCANNER_CREDENTIAL_KEY moves from the
// public dev key to the real one; values already on the real key, values
// readable with neither key, and empty values are left alone.
func TestRekey_Success_MovesEveryColumnOffTheDevKey(t *testing.T) {
	db := rekeyDB(t)
	dev, real := keyOf(t, "openrisk-dev-scanner-credential-key-change-me"), keyOf(t, "the-real-key")
	type rows struct{ onDev, onReal, foreign, empty uuid.UUID }
	seeded := map[string]rows{}
	for _, c := range Columns {
		seeded[c.Table] = rows{
			onDev:   addEnc(t, db, c, `{"secret":"dev-`+c.Table+`"}`, dev),
			onReal:  addEnc(t, db, c, `{"secret":"real-`+c.Table+`"}`, real),
			foreign: addEnc(t, db, c, `{"secret":"foreign"}`, keyOf(t, "someone-else")),
			empty:   add(t, db, c, ""),
		}
	}

	results, err := Rekey(context.Background(), db, dev, real, Columns)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(Columns) {
		t.Fatalf("got %d results, want %d", len(results), len(Columns))
	}
	for i, c := range Columns {
		if r := results[i]; r.Table != c.Table || r.Rekeyed != 1 || r.Unreadable != 1 {
			t.Fatalf("%s: got %+v, want 1 rekeyed and 1 unreadable", c.Table, r)
		}
		s := seeded[c.Table]
		for id, want := range map[uuid.UUID]string{s.onDev: `{"secret":"dev-` + c.Table + `"}`, s.onReal: `{"secret":"real-` + c.Table + `"}`} {
			got, err := crypto.DecryptAES256GCM(stored(t, db, c, id), real)
			if err != nil || got != want {
				t.Fatalf("%s %s: got %q, %v; want %q under the real key", c.Table, id, got, err, want)
			}
		}
		if _, err := crypto.DecryptAES256GCM(stored(t, db, c, s.onDev), dev); err == nil {
			t.Fatalf("%s: the dev key still decrypts a rekeyed value", c.Table)
		}
		if got, _ := crypto.DecryptAES256GCM(stored(t, db, c, s.foreign), keyOf(t, "someone-else")); got != `{"secret":"foreign"}` {
			t.Fatalf("%s: a value readable with neither key was changed", c.Table)
		}
		if stored(t, db, c, s.empty) != "" {
			t.Fatalf("%s: an empty value was written", c.Table)
		}
	}

	again, err := Rekey(context.Background(), db, dev, real, Columns)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range again {
		if r.Rekeyed != 0 {
			t.Fatalf("second pass on %s: %+v; want nothing to do", r.Table, r)
		}
	}
}

// Nothing stored: the pass reports zero everywhere and fails on nothing.
func TestRekey_NotFound_EmptyTablesAreANoOp(t *testing.T) {
	db := rekeyDB(t)
	results, err := Rekey(context.Background(), db, keyOf(t, "dev"), keyOf(t, "real"), Columns)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Rekeyed != 0 || r.Unreadable != 0 {
			t.Fatalf("%s: got %+v on an empty table", r.Table, r)
		}
	}
}

// A value the old key cannot open (a key that changed, not the dev key) is
// never rewritten: the pass cannot replace a credential it does not hold the
// key for.
func TestRekey_Unauthorized_AValueUnderAnotherKeyIsNotTouched(t *testing.T) {
	db := rekeyDB(t)
	c := Columns[0]
	id := addEnc(t, db, c, `{"aws_secret":"x"}`, keyOf(t, "a-previous-real-key"))
	before := stored(t, db, c, id)

	results, err := Rekey(context.Background(), db, keyOf(t, "dev"), keyOf(t, "real"), Columns)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Unreadable != 1 || results[0].Rekeyed != 0 {
		t.Fatalf("got %+v, want 1 unreadable", results[0])
	}
	if stored(t, db, c, id) != before {
		t.Fatal("a value under another key was rewritten")
	}
}

type ghost struct{ ID uuid.UUID }

// A table that cannot be read stops the pass with the table named, so a boot
// never half-runs silently.
func TestRekey_ReportsTheTableThatFailed(t *testing.T) {
	db := rekeyDB(t)
	cols := []Column{{Model: &ghost{}, Table: "ghosts", Column: "encrypted_credentials"}}
	_, err := Rekey(context.Background(), db, keyOf(t, "dev"), keyOf(t, "real"), cols)
	if err == nil || !strings.Contains(err.Error(), "ghosts") {
		t.Fatalf("want an error naming ghosts, got %v", err)
	}
}
