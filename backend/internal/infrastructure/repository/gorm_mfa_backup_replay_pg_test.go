// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #946 — N challenges racing with the same backup code: exactly one spends it.
// Like the #849 test, it needs separate connections, so it writes one row to
// the real mfa_backup_codes table (migrated schema required) and deletes it.
func TestGormMFARepository_MarkBackupCodeAsUsed_ConcurrentReplay_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(20)

	repo := NewGormMFARepository(db)
	ctx := context.Background()
	id, user, tenant := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.MFABackupCode{ID: id, UserID: user, TenantID: tenant, CodeHash: "x"}).Error)
	t.Cleanup(func() {
		// Scoped to the row this test created, by primary key and tenant.
		db.Unscoped().Where("id = ? AND tenant_id = ?", id, tenant).Delete(&domain.MFABackupCode{})
	})

	// Another tenant's or another user's id touches nothing.
	ok, err := repo.MarkBackupCodeAsUsed(ctx, id, user, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok, "another tenant must not spend the code")
	ok, err = repo.MarkBackupCodeAsUsed(ctx, id, uuid.New(), tenant)
	require.NoError(t, err)
	assert.False(t, ok, "another user must not spend the code")

	const racers = 20
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := repo.MarkBackupCodeAsUsed(ctx, id, user, tenant)
			assert.NoError(t, err)
			if ok {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 1, wins.Load(), "exactly one of %d concurrent uses of one backup code may succeed", racers)

	unused, err := repo.GetUnusedBackupCodes(ctx, user, tenant)
	require.NoError(t, err)
	assert.Empty(t, unused)
}
