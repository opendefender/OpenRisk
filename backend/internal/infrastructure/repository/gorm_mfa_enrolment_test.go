// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #889, #714 — StartMFAEnrolment creates a secret or gives an unfinished
// enrolment a new key, replaces the backup codes, and does both or neither.

func newCodes(n int) []*domain.MFABackupCode {
	codes := make([]*domain.MFABackupCode, n)
	for i := range codes {
		codes[i] = &domain.MFABackupCode{CodeHash: uuid.NewString()}
	}
	return codes
}

func codeHashes(t *testing.T, db *gorm.DB, userID, tenantID uuid.UUID) []string {
	t.Helper()
	var hashes []string
	require.NoError(t, db.Model(&domain.MFABackupCode{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Order("code_hash").Pluck("code_hash", &hashes).Error)
	return hashes
}

func hashesOf(codes []*domain.MFABackupCode) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = c.CodeHash
	}
	return out
}

func secretOf(t *testing.T, db *gorm.DB, userID uuid.UUID) *domain.MFASecret {
	t.Helper()
	var s domain.MFASecret
	err := db.Where("user_id = ?", userID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	require.NoError(t, err)
	return &s
}

func checkStartEnrolment(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := NewGormMFARepository(db)
	ctx := context.Background()
	user, tenant := uuid.New(), uuid.New()

	// First enrolment creates the row and the codes.
	first := newCodes(8)
	ok, err := repo.StartMFAEnrolment(ctx, user, tenant, "k1", first)
	require.NoError(t, err)
	assert.True(t, ok)
	s := secretOf(t, db, user)
	require.NotNil(t, s)
	assert.Equal(t, "k1", s.SecretEncrypted)
	assert.Equal(t, tenant, s.TenantID)
	assert.False(t, s.IsVerified)
	assert.ElementsMatch(t, hashesOf(first), codeHashes(t, db, user, tenant))

	// The abandoned key had been used: its replay step must not carry over.
	step, used := int64(42), time.Now()
	require.NoError(t, db.Model(&domain.MFASecret{}).Where("user_id = ? AND tenant_id = ?", user, tenant).
		Updates(map[string]any{"last_totp_step": step, "last_used_at": used}).Error)

	// Started over N times: one row, the newest key, only the newest codes.
	var last []*domain.MFABackupCode
	for i := range 3 {
		last = newCodes(8)
		ok, err = repo.StartMFAEnrolment(ctx, user, tenant, fmt.Sprintf("k%d", i+2), last)
		require.NoError(t, err)
		assert.True(t, ok)
	}
	var rows int64
	require.NoError(t, db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", user).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)
	s = secretOf(t, db, user)
	assert.Equal(t, "k4", s.SecretEncrypted)
	assert.Equal(t, s.TenantID, tenant)
	assert.Nil(t, s.LastTOTPStep, "the abandoned key's replay step must not carry over")
	assert.Nil(t, s.LastUsedAt)
	assert.ElementsMatch(t, hashesOf(last), codeHashes(t, db, user, tenant), "only the latest set of codes survives")

	// Another tenant cannot reach the row, nor the codes.
	ok, err = repo.StartMFAEnrolment(ctx, user, uuid.New(), "intruder", newCodes(8))
	require.NoError(t, err)
	assert.False(t, ok, "tenant-scoped: another tenant replaces nothing")
	assert.Equal(t, "k4", secretOf(t, db, user).SecretEncrypted)
	assert.ElementsMatch(t, hashesOf(last), codeHashes(t, db, user, tenant))

	// A verified secret is never replaced, and its codes stay.
	require.NoError(t, db.Model(&domain.MFASecret{}).Where("user_id = ? AND tenant_id = ?", user, tenant).
		Update("is_verified", true).Error)
	ok, err = repo.StartMFAEnrolment(ctx, user, tenant, "attack", newCodes(8))
	require.NoError(t, err)
	assert.False(t, ok, "a verified secret is never replaced")
	s = secretOf(t, db, user)
	assert.Equal(t, "k4", s.SecretEncrypted)
	assert.True(t, s.IsVerified)
	assert.ElementsMatch(t, hashesOf(last), codeHashes(t, db, user, tenant), "a refused enrolment must not touch the codes")
}

// checkStartEnrolmentRollsBack fails the backup-code insert, the last write of
// the transaction, and expects every earlier write undone.
func checkStartEnrolmentRollsBack(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := NewGormMFARepository(db)
	ctx := context.Background()

	pending, tenant := uuid.New(), uuid.New()
	kept := newCodes(8)
	ok, err := repo.StartMFAEnrolment(ctx, pending, tenant, "before", kept)
	require.NoError(t, err)
	require.True(t, ok)

	boom := errors.New("backup code insert failed")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_codes", func(tx *gorm.DB) {
		if tx.Statement.Table == "mfa_backup_codes" {
			_ = tx.AddError(boom)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:fail_codes") })

	// Replacing an unfinished enrolment: the old key and codes survive.
	_, err = repo.StartMFAEnrolment(ctx, pending, tenant, "after", newCodes(8))
	require.ErrorIs(t, err, boom)
	assert.Equal(t, "before", secretOf(t, db, pending).SecretEncrypted, "the new key must roll back with the codes")
	assert.ElementsMatch(t, hashesOf(kept), codeHashes(t, db, pending, tenant), "the code delete must roll back")

	// A first enrolment: no secret is left behind.
	fresh := uuid.New()
	_, err = repo.StartMFAEnrolment(ctx, fresh, tenant, "orphan", newCodes(8))
	require.ErrorIs(t, err, boom)
	assert.Nil(t, secretOf(t, db, fresh), "a secret without its codes must not be stored")
}

// sqliteEnrolmentRepo adds the unique user_id index production has
// (domain.MFASecret's uniqueIndex): the upsert targets it.
func sqliteEnrolmentRepo(t *testing.T) *gorm.DB {
	t.Helper()
	_, db := setupMFARepo(t)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_mfa_secrets_user_id ON mfa_secrets (user_id)`).Error)
	return db
}

func TestGormMFARepository_StartMFAEnrolment(t *testing.T) {
	checkStartEnrolment(t, sqliteEnrolmentRepo(t))
}

func TestGormMFARepository_StartMFAEnrolment_RollsBackWhenTheCodesFail(t *testing.T) {
	checkStartEnrolmentRollsBack(t, sqliteEnrolmentRepo(t))
}

func openMFAPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db
}

// inTempMFATables runs check against session-private copies of the MFA tables,
// inside a transaction that is always rolled back.
func inTempMFATables(t *testing.T, db *gorm.DB, check func(*testing.T, *gorm.DB)) {
	t.Helper()
	rollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE mfa_secrets (LIKE public.mfa_secrets INCLUDING ALL) ON COMMIT DROP`).Error)
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE mfa_backup_codes (LIKE public.mfa_backup_codes INCLUDING ALL) ON COMMIT DROP`).Error)
		check(t, tx)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

func TestGormMFARepository_StartMFAEnrolment_Postgres(t *testing.T) {
	inTempMFATables(t, openMFAPostgres(t), checkStartEnrolment)
}

func TestGormMFARepository_StartMFAEnrolment_RollsBackWhenTheCodesFail_Postgres(t *testing.T) {
	inTempMFATables(t, openMFAPostgres(t), checkStartEnrolmentRollsBack)
}

// Two setups racing on an account with no secret yet both land, and leave one
// row and one set of codes — before #714 the second hit the unique user_id.
// This needs separate connections, so like the #849 race test it writes to the
// real tables (migrated schema required) and deletes its rows afterwards.
func TestGormMFARepository_StartMFAEnrolment_ConcurrentFirstSetup_Postgres(t *testing.T) {
	db := openMFAPostgres(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)

	repo := NewGormMFARepository(db)
	ctx := context.Background()
	user, tenant := uuid.New(), uuid.New()
	t.Cleanup(func() {
		// Scoped to the user and tenant this test made up.
		db.Unscoped().Where("user_id = ? AND tenant_id = ?", user, tenant).Delete(&domain.MFASecret{})
		db.Where("user_id = ? AND tenant_id = ?", user, tenant).Delete(&domain.MFABackupCode{})
	})

	const racers = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := repo.StartMFAEnrolment(ctx, user, tenant, "k"+uuid.NewString(), newCodes(8))
			assert.NoError(t, err, "racer %d", i)
			assert.True(t, ok, "racer %d", i)
		}()
	}
	close(start)
	wg.Wait()

	var rows int64
	require.NoError(t, db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ? AND tenant_id = ?", user, tenant).Count(&rows).Error)
	assert.EqualValues(t, 1, rows)
	assert.Len(t, codeHashes(t, db, user, tenant), 8, "one set of codes, not a mix of several")
}
