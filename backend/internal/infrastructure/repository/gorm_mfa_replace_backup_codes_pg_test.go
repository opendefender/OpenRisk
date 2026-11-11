// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #1032 — ReplaceBackupCodes on the real schema: all or nothing, only under a
// verified secret of that tenant, and never leaving codes without a secret
// when it races DisableMFA.

func openReplacePG(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)
	return db
}

// seedFactor writes a secret and one "old" code, and removes both, by user and
// tenant, when the test ends.
func seedFactor(t *testing.T, db *gorm.DB, verified bool) (user, tenant uuid.UUID) {
	t.Helper()
	user, tenant = uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: user, TenantID: tenant, SecretEncrypted: "x", IsVerified: verified,
	}).Error)
	require.NoError(t, db.Create(&domain.MFABackupCode{ID: uuid.New(), UserID: user, TenantID: tenant, CodeHash: "old"}).Error)
	t.Cleanup(func() {
		db.Unscoped().Where("user_id = ? AND tenant_id = ?", user, tenant).Delete(&domain.MFABackupCode{})
		db.Unscoped().Where("user_id = ? AND tenant_id = ?", user, tenant).Delete(&domain.MFASecret{})
	})
	return user, tenant
}

func replacementCodes(n int) []*domain.MFABackupCode {
	out := make([]*domain.MFABackupCode, n)
	for i := range out {
		out[i] = &domain.MFABackupCode{CodeHash: "new-" + uuid.NewString()}
	}
	return out
}

func storedHashes(t *testing.T, db *gorm.DB, user, tenant uuid.UUID) []string {
	t.Helper()
	var hashes []string
	require.NoError(t, db.Model(&domain.MFABackupCode{}).
		Where("user_id = ? AND tenant_id = ?", user, tenant).Pluck("code_hash", &hashes).Error)
	return hashes
}

func TestGormMFARepository_ReplaceBackupCodes_Postgres(t *testing.T) {
	db := openReplacePG(t)
	repo := NewGormMFARepository(db)
	ctx := context.Background()

	t.Run("replaces every code under a verified secret", func(t *testing.T) {
		user, tenant := seedFactor(t, db, true)
		ok, err := repo.ReplaceBackupCodes(ctx, user, tenant, replacementCodes(8))
		require.NoError(t, err)
		assert.True(t, ok)
		hashes := storedHashes(t, db, user, tenant)
		assert.Len(t, hashes, 8)
		assert.NotContains(t, hashes, "old")
	})

	t.Run("another tenant writes nothing", func(t *testing.T) {
		user, tenant := seedFactor(t, db, true)
		ok, err := repo.ReplaceBackupCodes(ctx, user, uuid.New(), replacementCodes(8))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, []string{"old"}, storedHashes(t, db, user, tenant))
	})

	t.Run("an unverified secret writes nothing", func(t *testing.T) {
		user, tenant := seedFactor(t, db, false)
		ok, err := repo.ReplaceBackupCodes(ctx, user, tenant, replacementCodes(8))
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, []string{"old"}, storedHashes(t, db, user, tenant))
	})

	t.Run("a failed insert rolls the delete back", func(t *testing.T) {
		user, tenant := seedFactor(t, db, true)
		dup := uuid.New()
		codes := []*domain.MFABackupCode{{ID: dup, CodeHash: "a"}, {ID: dup, CodeHash: "b"}}
		_, err := repo.ReplaceBackupCodes(ctx, user, tenant, codes)
		require.Error(t, err, "a duplicate primary key must fail the insert")
		assert.Equal(t, []string{"old"}, storedHashes(t, db, user, tenant), "the old codes stay")
	})

	t.Run("racing DisableMFA never leaves codes without a secret", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			user, tenant := seedFactor(t, db, true)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, err := repo.ReplaceBackupCodes(ctx, user, tenant, replacementCodes(8))
				assert.NoError(t, err)
			}()
			go func() {
				defer wg.Done()
				assert.NoError(t, repo.DisableMFA(ctx, user, tenant))
			}()
			wg.Wait()

			var secrets int64
			require.NoError(t, db.Unscoped().Model(&domain.MFASecret{}).
				Where("user_id = ? AND tenant_id = ?", user, tenant).Count(&secrets).Error)
			require.Zero(t, secrets)
			assert.Empty(t, storedHashes(t, db, user, tenant), "codes outlived their secret (round %d)", i)
		}
	})
}
