// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #1428 — AppendTx is a shared mechanism, tested on its own with a stand-in
// business table: the audit entry and the business row commit or roll back
// together, in both directions, on PostgreSQL; and on sqlite (where the advisory
// lock does not exist) Append and AppendTx still work.

type appendTxWidget struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;not null"`
	Name     string
}

func (appendTxWidget) TableName() string { return "appendtx_widgets" }

func appendTxEvent(tenant uuid.UUID, entity string) *domain.AuditEvent {
	return &domain.AuditEvent{
		ID: uuid.New(), TenantID: tenant, Action: domain.AuditActionCreate, EntityType: "widget",
		EntityID: entity, Summary: "widget", Source: domain.AuditSourceExplicit, CreatedAt: time.Now().UTC(),
	}
}

// throwawayPG opens a database pinned to a schema of its own.
func throwawayPG(t *testing.T) *gorm.DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	open := func(dsn string) *gorm.DB {
		db, err := testdb.Open(t, postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		return db
	}
	schema := "appendtxatom_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
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
	require.NoError(t, db.AutoMigrate(&domain.AuditEvent{}, &appendTxWidget{}))
	return db
}

func countModelRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	// tenant: test helper counting every row of a table to prove an all-or-nothing outcome.
	require.NoError(t, db.Model(model).Count(&n).Error)
	return n
}

func TestAppendTx_Postgres_AtomicInBothDirections(t *testing.T) {
	db := throwawayPG(t)
	repo := NewGormAuditChainRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	// 1. The audit append fails: the business row rolls back.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&appendTxWidget{ID: uuid.New(), TenantID: tenant, Name: "a"}).Error; err != nil {
			return err
		}
		return repo.AppendTx(ctx, tx, appendTxEvent(uuid.Nil, "x")) // no tenant: refused
	})
	require.Error(t, err)
	assert.Zero(t, countModelRows(t, db, &appendTxWidget{}), "audit failure rolls the business row back")
	assert.Zero(t, countModelRows(t, db, &domain.AuditEvent{}))

	// 2. The business write fails after the audit append: the audit row rolls back.
	dup := uuid.New()
	require.NoError(t, db.Create(&appendTxWidget{ID: dup, TenantID: tenant, Name: "first"}).Error)
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := repo.AppendTx(ctx, tx, appendTxEvent(tenant, dup.String())); err != nil {
			return err
		}
		return tx.Create(&appendTxWidget{ID: dup, TenantID: tenant, Name: "duplicate key"}).Error
	})
	require.Error(t, err)
	assert.Equal(t, int64(1), countModelRows(t, db, &appendTxWidget{}))
	assert.Zero(t, countModelRows(t, db, &domain.AuditEvent{}), "business failure rolls the audit row back")

	// 3. Both succeed: both are there, and the chain verifies.
	id := uuid.New()
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&appendTxWidget{ID: id, TenantID: tenant, Name: "ok"}).Error; err != nil {
			return err
		}
		return repo.AppendTx(ctx, tx, appendTxEvent(tenant, id.String()))
	}))
	assert.Equal(t, int64(2), countModelRows(t, db, &appendTxWidget{}))
	evs, err := repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, int64(1), evs[0].Sequence, "the rolled-back entries burned no sequence number")
	assert.True(t, domain.VerifyAuditChain(tenant, evs, nil).Valid)
}

func TestAppendTx_SQLite_SkipsAdvisoryLockAndStillWorks(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open("file:appendtx"+uuid.NewString()+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.AuditEvent{}, &appendTxWidget{}))
	repo := NewGormAuditChainRepository(db)
	require.False(t, repo.supportsRowLock(), "sqlite has neither FOR UPDATE nor advisory locks")
	ctx := context.Background()
	tenant := uuid.New()

	require.NoError(t, repo.Append(ctx, appendTxEvent(tenant, "1")))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&appendTxWidget{ID: uuid.New(), TenantID: tenant, Name: "w"}).Error; err != nil {
			return err
		}
		return repo.AppendTx(ctx, tx, appendTxEvent(tenant, "2"))
	}))
	boom := errors.New("business failure")
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := repo.AppendTx(ctx, tx, appendTxEvent(tenant, "3")); err != nil {
			return err
		}
		return boom
	}), boom)
	require.NoError(t, repo.Append(ctx, appendTxEvent(tenant, "4")))

	evs, err := repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
	require.NoError(t, err)
	require.Len(t, evs, 3, "the rolled-back entry is absent")
	for i, e := range evs {
		assert.Equal(t, int64(i+1), e.Sequence)
	}
	assert.True(t, domain.VerifyAuditChain(tenant, evs, nil).Valid)
	assert.Equal(t, int64(1), countModelRows(t, db, &appendTxWidget{}))

	require.Error(t, repo.AppendTx(ctx, nil, appendTxEvent(tenant, "5")), "a nil transaction is refused")
}
