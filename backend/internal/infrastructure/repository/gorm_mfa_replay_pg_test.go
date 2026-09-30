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

// #849 — two requests racing with the same TOTP code cannot both win. This needs
// separate connections, so unlike the other _pg tests it writes one row to the
// real mfa_secrets table (migrated schema required) and deletes it afterwards.
func TestGormMFARepository_ConsumeTOTPStep_ConcurrentReplay_Postgres(t *testing.T) {
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
	require.NoError(t, db.Create(&domain.MFASecret{ID: id, UserID: user, TenantID: tenant, SecretEncrypted: "x", IsVerified: true}).Error)
	t.Cleanup(func() {
		// Scoped to the row this test created, by primary key and tenant.
		db.Unscoped().Where("id = ? AND tenant_id = ?", id, tenant).Delete(&domain.MFASecret{})
	})

	const racers = 20
	const step = int64(63_333_333)
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := repo.ConsumeTOTPStep(ctx, user, tenant, step)
			assert.NoError(t, err)
			if ok {
				wins.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.EqualValues(t, 1, wins.Load(), "exactly one of %d concurrent uses of one code may succeed", racers)

	// An earlier step is refused, a later one accepted.
	ok, err := repo.ConsumeTOTPStep(ctx, user, tenant, step-1)
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = repo.ConsumeTOTPStep(ctx, user, tenant, step+1)
	require.NoError(t, err)
	assert.True(t, ok)

	// Another tenant's id touches nothing.
	ok, err = repo.ConsumeTOTPStep(ctx, user, uuid.New(), step+5)
	require.NoError(t, err)
	assert.False(t, ok)

	// A Save of a stale struct must not lower the mark.
	stale := &domain.MFASecret{ID: id, UserID: user, TenantID: tenant, SecretEncrypted: "x", IsVerified: true}
	require.NoError(t, repo.UpdateMFASecret(ctx, stale))
	ok, err = repo.ConsumeTOTPStep(ctx, user, tenant, step+1)
	require.NoError(t, err)
	assert.False(t, ok, "UpdateMFASecret must leave last_totp_step alone")
}
