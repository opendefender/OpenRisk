// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/infrastructure/credrekey"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
	"github.com/opendefender/openrisk/pkg/crypto"
)

func scannerEnv(value string) func(string) string {
	return func(k string) string {
		if k == "SCANNER_CREDENTIAL_KEY" {
			return value
		}
		return ""
	}
}

// #1200: production refuses an unset key, a blank one, the public constant,
// and any value that derives to the same AES key as the constant.
func TestScannerCredentialKey_ProductionRefusesMissingOrPublicKey(t *testing.T) {
	for _, v := range []string{
		"",
		"   ",
		devScannerCredentialKey,
		devScannerCredentialKey[:32] + "anything-after-the-32nd-byte",
	} {
		_, _, err := scannerCredentialKey(scannerEnv(v), true)
		if err == nil || !strings.Contains(err.Error(), "SCANNER_CREDENTIAL_KEY") {
			t.Fatalf("%q: want a refusal naming the variable, got %v", v, err)
		}
	}
}

// A key an install already runs on is used exactly as given: trimming or
// rejecting it would leave every stored credential undecryptable.
func TestScannerCredentialKey_ProductionKeepsAnExistingKeyVerbatim(t *testing.T) {
	for _, v := range []string{"short", " spaced-key ", strings.Repeat("k", 64)} {
		raw, warning, err := scannerCredentialKey(scannerEnv(v), true)
		if err != nil || warning != "" || raw != v {
			t.Fatalf("%q: raw=%q warning=%q err=%v", v, raw, warning, err)
		}
	}
}

func TestScannerCredentialKey_DevelopmentWarnsAndFallsBack(t *testing.T) {
	raw, warning, err := scannerCredentialKey(scannerEnv(""), false)
	if err != nil || raw != devScannerCredentialKey || !strings.Contains(warning, "SCANNER_CREDENTIAL_KEY") {
		t.Fatalf("raw=%q warning=%q err=%v", raw, warning, err)
	}
}

// Outside production the public constant is accepted, as before: a developer
// who exported it keeps their local data.
func TestScannerCredentialKey_DevelopmentAcceptsTheConstant(t *testing.T) {
	raw, _, err := scannerCredentialKey(scannerEnv(devScannerCredentialKey), false)
	if err != nil || raw != devScannerCredentialKey {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}

// credentialDB holds one dev-key value in every column credrekey covers.
func credentialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	dev, _ := crypto.DeriveKey([]byte(devScannerCredentialKey), nil)
	for _, c := range credrekey.Columns {
		if err := db.Exec(`CREATE TABLE ` + c.Table + ` (id TEXT PRIMARY KEY, tenant_id TEXT, ` + c.Column + ` TEXT)`).Error; err != nil {
			t.Fatal(err)
		}
		enc, err := crypto.EncryptAES256GCM("secret-"+c.Table, dev)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO `+c.Table+` (id, tenant_id, `+c.Column+`) VALUES ('00000000-0000-0000-0000-000000000001', 't', ?)`, enc).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// #1200: the boot wiring moves every dev-key value onto a real key.
func TestRekeyDevCredentials_RealKeyMovesEveryColumn(t *testing.T) {
	db := credentialDB(t)
	results, err := rekeyDevCredentials(context.Background(), db, "a-real-operator-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(credrekey.Columns) {
		t.Fatalf("got %d results, want one per column", len(results))
	}
	real, _ := crypto.DeriveKey([]byte("a-real-operator-key"), nil)
	for _, r := range results {
		if r.Rekeyed != 1 {
			t.Fatalf("%s: %+v, want 1 rekeyed", r.Table, r)
		}
		var v string
		if err := db.Raw(`SELECT ` + r.Column + ` FROM ` + r.Table).Row().Scan(&v); err != nil {
			t.Fatal(err)
		}
		if got, err := crypto.DecryptAES256GCM(v, real); err != nil || got != "secret-"+r.Table {
			t.Fatalf("%s: got %q, %v under the real key", r.Table, got, err)
		}
	}
}

// Running on the development key (or one deriving to it) touches nothing.
func TestRekeyDevCredentials_DevKeyIsANoOp(t *testing.T) {
	for _, k := range []string{devScannerCredentialKey, devScannerCredentialKey[:32] + "suffix"} {
		db := credentialDB(t)
		results, err := rekeyDevCredentials(context.Background(), db, k)
		if err != nil || results != nil {
			t.Fatalf("%q: results=%+v err=%v; want nothing done", k, results, err)
		}
		dev, _ := crypto.DeriveKey([]byte(devScannerCredentialKey), nil)
		c := credrekey.Columns[0]
		var v string
		if err := db.Raw(`SELECT ` + c.Column + ` FROM ` + c.Table).Row().Scan(&v); err != nil {
			t.Fatal(err)
		}
		if _, err := crypto.DecryptAES256GCM(v, dev); err != nil {
			t.Fatalf("%q: a dev-key value was rewritten", k)
		}
	}
}
