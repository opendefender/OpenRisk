// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmfa

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/pkg/crypto"
)

func rekeyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE mfa_secrets (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := sqliteschema.Reconcile(db, "mfa_secrets", &domain.MFASecret{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func keyOf(s string) []byte { k := sha256.Sum256([]byte(s)); return k[:] }

func addSecret(t *testing.T, db *gorm.DB, plain string, key []byte, deleted bool) uuid.UUID {
	t.Helper()
	enc, err := crypto.EncryptAES256GCM(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	row := &domain.MFASecret{ID: uuid.New(), UserID: uuid.New(), TenantID: uuid.New(), SecretEncrypted: enc, IsVerified: true}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if deleted {
		if err := db.Delete(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return row.ID
}

func secretFor(t *testing.T, db *gorm.DB, id uuid.UUID, key []byte) (string, error) {
	t.Helper()
	var enc []string
	if err := db.Unscoped().Model(&domain.MFASecret{}).Where("id = ?", id).Pluck("secret_encrypted", &enc).Error; err != nil || len(enc) != 1 {
		t.Fatalf("read secret %s: %v", id, err)
	}
	return crypto.DecryptAES256GCM(enc[0], key)
}

// #1062: secrets left under the public dev key move to the real key, so an
// install that gets its first MFA_ENCRYPTION_KEY loses no enrolment.
func TestRekeySecrets_MovesDevKeySecretsToTheRealKey(t *testing.T) {
	db := rekeyDB(t)
	dev, real := keyOf("openrisk-dev-mfa-encryption-key-change-me"), keyOf("the-real-key")
	onDev := addSecret(t, db, "JBSWY3DPEHPK3PXP", dev, false)
	onDevDeleted := addSecret(t, db, "KRSXG5CTMVRXEZLU", dev, true)
	onReal := addSecret(t, db, "MFRGGZDFMZTWQ2LK", real, false)
	foreign := addSecret(t, db, "GEZDGNBVGY3TQOJQ", keyOf("someone-else"), false)

	res, err := RekeySecrets(context.Background(), db, dev, real)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rekeyed != 2 || res.Unreadable != 1 {
		t.Fatalf("got %+v, want 2 rekeyed and 1 unreadable", res)
	}
	for id, want := range map[uuid.UUID]string{onDev: "JBSWY3DPEHPK3PXP", onDevDeleted: "KRSXG5CTMVRXEZLU", onReal: "MFRGGZDFMZTWQ2LK"} {
		got, err := secretFor(t, db, id, real)
		if err != nil || got != want {
			t.Fatalf("secret %s: got %q, %v; want %q under the real key", id, got, err, want)
		}
	}
	if _, err := secretFor(t, db, onDev, dev); err == nil {
		t.Fatal("the dev key still decrypts a rekeyed secret")
	}
	if got, _ := secretFor(t, db, foreign, keyOf("someone-else")); got != "GEZDGNBVGY3TQOJQ" {
		t.Fatal("a secret readable with neither key was changed")
	}

	again, err := RekeySecrets(context.Background(), db, dev, real)
	if err != nil || again.Rekeyed != 0 {
		t.Fatalf("second pass: %+v, %v; want nothing to do", again, err)
	}
}

// A row rewritten by another replica between the read and the write is left
// to that replica: the update is conditional on the ciphertext read.
func TestRekeySecrets_DoesNotOverwriteAConcurrentRekey(t *testing.T) {
	db := rekeyDB(t)
	dev, real := keyOf("dev"), keyOf("real")
	id := addSecret(t, db, "JBSWY3DPEHPK3PXP", dev, false)
	var before []string
	_ = db.Model(&domain.MFASecret{}).Where("id = ?", id).Pluck("secret_encrypted", &before).Error

	// Another replica got there first.
	enc, _ := crypto.EncryptAES256GCM("JBSWY3DPEHPK3PXP", real)
	_ = db.Model(&domain.MFASecret{}).Where("id = ?", id).UpdateColumn("secret_encrypted", enc).Error
	upd := db.Model(&domain.MFASecret{}).
		Where("id = ? AND secret_encrypted = ?", id, before[0]).
		UpdateColumn("secret_encrypted", "stale-write")
	if upd.RowsAffected != 0 {
		t.Fatal("the conditional update overwrote a newer ciphertext")
	}
}
