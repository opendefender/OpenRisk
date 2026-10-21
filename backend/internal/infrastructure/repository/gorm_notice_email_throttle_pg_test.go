// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #997 — the notice e-mail throttle's claim row, on real Postgres: ON CONFLICT
// ... DO UPDATE ... WHERE is what keeps two replicas from both sending, and
// sqlite cannot show it. Each test runs migration 0081 in a schema of its own
// and drops the schema afterwards.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

const throttleMigration = "../../../../migrations/0081_notice_email_throttle"

// throttleDB opens a pool whose every connection sees only a fresh schema, with
// the up migration applied.
func throttleDB(t *testing.T) (*gorm.DB, string) {
	return throttleSchema(t, false)
}

// throttleSchema builds the table the way a deployment does: AutoMigrate first
// when autoMigrate is set (the compose boot has no SQL layer), then 0081.
func throttleSchema(t *testing.T, autoMigrate bool) (*gorm.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "throttle_997_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(20)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if autoMigrate {
		require.NoError(t, db.AutoMigrate(&domain.NoticeEmailClaim{}))
	}
	up, err := os.ReadFile(throttleMigration + ".up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	return db, schema
}

func throttleKey() domain.NoticeEmailClaim {
	return domain.NoticeEmailClaim{
		TenantID: uuid.New(), UserID: uuid.New(),
		Type:         domain.NotificationTypeActionAssigned,
		ResourceType: "mitigation", ResourceID: uuid.New(),
	}
}

func TestGormNoticeEmailThrottle_Success(t *testing.T) {
	db, _ := throttleDB(t)
	repo := NewGormNoticeEmailThrottle(db)
	ctx := context.Background()
	key := throttleKey()

	first, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, first, "a new key is claimed")
	require.False(t, first.SentAt.IsZero())

	held, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.Nil(t, held, "inside the window the slot is held")

	// The window has passed for this row.
	require.NoError(t, db.Exec(`UPDATE notice_email_throttle SET sent_at = now() - interval '2 hours' WHERE tenant_id = ?`, key.TenantID).Error)
	again, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, again, "past the window the slot is restamped")
	require.True(t, again.SentAt.After(first.SentAt.Add(-time.Second)))
}

// Releasing a claim that is not the current one removes nothing.
func TestGormNoticeEmailThrottle_NotFound(t *testing.T) {
	db, _ := throttleDB(t)
	repo := NewGormNoticeEmailThrottle(db)
	ctx := context.Background()
	key := throttleKey()

	claim, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, claim)

	stale := *claim
	stale.SentAt = claim.SentAt.Add(-time.Minute)
	require.NoError(t, repo.Release(ctx, stale))
	held, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.Nil(t, held, "a stale release left the live claim alone")

	// The real one gives the slot back.
	require.NoError(t, repo.Release(ctx, *claim))
	retry, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, retry, "a released claim does not hold the retry")
}

// Tenant isolation: the same user, type and resource in another tenant is a
// different slot, and another tenant's release cannot free this one.
func TestGormNoticeEmailThrottle_Unauthorized(t *testing.T) {
	db, _ := throttleDB(t)
	repo := NewGormNoticeEmailThrottle(db)
	ctx := context.Background()
	key := throttleKey()

	claim, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, claim)

	other := key
	other.TenantID = uuid.New()
	otherClaim, err := repo.Claim(ctx, other, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, otherClaim, "another tenant does not share the slot")

	forged := *claim
	forged.TenantID = other.TenantID
	require.NoError(t, repo.Release(ctx, forged))
	held, err := repo.Claim(ctx, key, time.Hour)
	require.NoError(t, err)
	require.Nil(t, held, "another tenant's release frees nothing here")
}

// #997 criterion 7 — replicas racing on one key: exactly one gets the claim.
func TestGormNoticeEmailThrottle_ConcurrentClaims_Postgres(t *testing.T) {
	db, _ := throttleDB(t)
	repo := NewGormNoticeEmailThrottle(db)
	ctx := context.Background()

	for round := range 5 {
		key := throttleKey()
		const racers = 20
		var wins atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for range racers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				c, err := repo.Claim(ctx, key, time.Hour)
				assert.NoError(t, err)
				if c != nil {
					wins.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		require.EqualValues(t, 1, wins.Load(), "round %d: exactly one of %d concurrent claims sends", round, racers)
	}
}

// #997 criterion 7 — the down step drops the table.
func TestNoticeEmailThrottleMigration_DownDropsTable(t *testing.T) {
	db, schema := throttleDB(t)
	exists := func() bool {
		var ok bool
		require.NoError(t, db.Raw(`SELECT to_regclass(?) IS NOT NULL`, schema+".notice_email_throttle").Scan(&ok).Error)
		return ok
	}
	require.True(t, exists(), "up creates the table")

	down, err := os.ReadFile(throttleMigration + ".down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
	require.False(t, exists(), "down drops the table")
}

// The table AutoMigrate builds from domain.NoticeEmailClaim has the key the
// claim's ON CONFLICT needs, and 0081 on top of it changes nothing.
func TestNoticeEmailThrottle_AutoMigrateTableTakesClaims(t *testing.T) {
	db, _ := throttleSchema(t, true)
	repo := NewGormNoticeEmailThrottle(db)
	key := throttleKey()

	first, err := repo.Claim(context.Background(), key, time.Hour)
	require.NoError(t, err)
	require.NotNil(t, first)
	held, err := repo.Claim(context.Background(), key, time.Hour)
	require.NoError(t, err)
	require.Nil(t, held)
}
