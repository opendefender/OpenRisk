// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
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

// #1428 — AppendTx seals an entry on the caller's transaction. Appenders that
// interleave on one tenant, some in their own transaction (Append) and some
// inside a business transaction that stays open a while (AppendTx), must leave a
// chain whose sequence is contiguous and which verifies: the advisory lock makes
// a waiter read the head the committed transaction wrote, not the stale one it
// would have read before the wait. Skipped without DATABASE_URL; builds its own
// throwaway schema.
func TestAuditChain_InterleavedAppendAndAppendTx_Postgres(t *testing.T) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	open := func(dsn string) *gorm.DB {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		return db
	}
	schema := "appendtx1428_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	admin := open(base)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	u, err := url.Parse(base)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()
	db := open(u.String())
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&domain.AuditEvent{}))

	repo := NewGormAuditChainRepository(db)
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()
	ev := func(tn uuid.UUID, i int) *domain.AuditEvent {
		return &domain.AuditEvent{
			ID: uuid.New(), TenantID: tn, Action: domain.AuditActionUpdate, EntityType: "control_exception",
			EntityID: fmt.Sprint(i), Summary: "interleaved", Source: domain.AuditSourceExplicit,
			CreatedAt: time.Now().UTC(),
		}
	}

	const each = 12
	var wg sync.WaitGroup
	errs := make(chan error, each*3)
	start := make(chan struct{})
	for i := 0; i < each; i++ {
		wg.Add(3)
		go func() { // own transaction
			defer wg.Done()
			<-start
			errs <- repo.Append(ctx, ev(tenant, i))
		}()
		go func() { // inside a business transaction that holds on for a moment
			defer wg.Done()
			<-start
			errs <- db.Transaction(func(tx *gorm.DB) error {
				if err := repo.AppendTx(ctx, tx, ev(tenant, i)); err != nil {
					return err
				}
				time.Sleep(15 * time.Millisecond)
				return nil
			})
		}()
		go func() { // another tenant is never made to wait for this one
			defer wg.Done()
			<-start
			errs <- repo.Append(ctx, ev(other, i))
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	for tn, want := range map[uuid.UUID]int{tenant: each * 2, other: each} {
		evs, err := repo.ListAll(ctx, tn, domain.AuditEventFilter{})
		require.NoError(t, err)
		require.Len(t, evs, want)
		for i, e := range evs {
			assert.Equal(t, int64(i+1), e.Sequence, "sequence is contiguous")
		}
		rep := domain.VerifyAuditChain(tn, evs, nil)
		assert.True(t, rep.Valid, "chain verifies: %+v", rep.Breaks)
	}

	// A rolled-back business transaction leaves nothing in the trail and does not
	// burn a sequence number.
	before, err := repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
	require.NoError(t, err)
	rollback := fmt.Errorf("business write failed")
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := repo.AppendTx(ctx, tx, ev(tenant, 99)); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	after, err := repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before))
	require.NoError(t, repo.Append(ctx, ev(tenant, 100)))
	after, _ = repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
	assert.Equal(t, int64(len(before)+1), after[len(after)-1].Sequence)
}
