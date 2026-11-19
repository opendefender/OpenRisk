// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

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
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #1428 — the repository on the database it runs on. Tenant B reads and revokes
// tenant A's exception and gets ErrNotFound; a create writes the row and its
// audit entry in one transaction; a failed audit append leaves no row. Skipped
// without DATABASE_URL. Each test builds a throwaway schema, so the database can
// start empty and nothing public is touched.

var (
	pgTenantA = uuid.MustParse("00000000-0000-4000-8428-00000000000a")
	pgTenantB = uuid.MustParse("00000000-0000-4000-8428-00000000000b")
)

type pgFixture struct {
	db       *gorm.DB
	chain    *repository.GormAuditChainRepository
	repo     *Repository
	controlA uuid.UUID
	now      time.Time
}

func openPG(t *testing.T) *pgFixture {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	open := func(dsn string) *gorm.DB {
		db, err := testdb.Open(t, postgres.Open(dsn), &gorm.Config{
			Logger:                                   logger.Default.LogMode(logger.Silent),
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		require.NoError(t, err)
		return db
	}
	schema := "ctlexc1428_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
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
	require.NoError(t, db.AutoMigrate(&controlchain.ControlRow{}, &ExceptionRow{}, &domain.AuditEvent{}))
	require.NoError(t, db.AutoMigrate(controlstatus.Models()...))

	f := &pgFixture{db: db, controlA: uuid.New(), now: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}
	f.chain = repository.NewGormAuditChainRepository(db)
	f.repo = NewRepository(db, f.chain)
	f.seedControl(t, pgTenantA, f.controlA)
	return f
}

func (f *pgFixture) seedControl(t *testing.T, tenant, id uuid.UUID) {
	t.Helper()
	require.NoError(t, f.db.Create(&controlchain.ControlRow{
		ID: id, TenantID: tenant, Source: "openrisk_user", SourceID: id.String(), Confidence: 0.7,
		Provenance: "{}", Name: "A.5.1", Applicability: "applicable", DeclaredStatus: "implemented", Tags: "[]",
	}).Error)
}

func (f *pgFixture) exception(tenant uuid.UUID) (*domain.ControlException, *domain.AuditEvent) {
	exp := f.now.Add(72 * time.Hour)
	ex, err := domain.NewControlException(domain.NewControlExceptionInput{
		TenantID: tenant, ControlID: f.controlA, OwnerID: uuid.New(), ApprovedBy: uuid.New(),
		Reason: "vendor patch pending", ExpiresAt: &exp,
	}, f.now)
	if err != nil {
		panic(err)
	}
	return ex, f.event(tenant, ex.ID, domain.AuditActionCreate)
}

func (f *pgFixture) event(tenant, entity uuid.UUID, action domain.AuditAction) *domain.AuditEvent {
	actor := uuid.New()
	return &domain.AuditEvent{
		ID: uuid.New(), TenantID: tenant, ActorID: &actor, ActorType: domain.AuditActorUser, Action: action,
		EntityType: "control_exception", EntityID: entity.String(), Summary: "control exception",
		After: domain.JSONMap{"status": "open"}, Source: domain.AuditSourceExplicit, CreatedAt: f.now,
	}
}

func (f *pgFixture) count(t *testing.T, model any) int64 {
	t.Helper()
	var n int64
	// tenant: test helper counting every row across tenants, to prove nothing was written.
	require.NoError(t, f.db.Model(model).Count(&n).Error)
	return n
}

func (f *pgFixture) verifyChain(t *testing.T, tenant uuid.UUID, want int) {
	t.Helper()
	evs, err := f.chain.ListAll(context.Background(), tenant, domain.AuditEventFilter{})
	require.NoError(t, err)
	require.Len(t, evs, want)
	for i, e := range evs {
		assert.Equal(t, int64(i+1), e.Sequence, "sequence is contiguous")
	}
	rep := domain.VerifyAuditChain(tenant, evs, nil)
	assert.True(t, rep.Valid, "chain verifies: %+v", rep.Breaks)
}

func TestControlExceptionRepository_CrossTenantIsNotFound(t *testing.T) {
	f := openPG(t)
	ctx := context.Background()
	ex, ev := f.exception(pgTenantA)
	require.NoError(t, f.repo.Create(ctx, ex, ev))

	// Tenant B cannot see tenant A's control.
	ok, err := f.repo.ControlExists(ctx, pgTenantB, f.controlA)
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = f.repo.ControlExists(ctx, pgTenantA, f.controlA)
	require.NoError(t, err)
	assert.True(t, ok)

	// Nor list its exceptions.
	got, err := f.repo.ListByControl(ctx, pgTenantB, f.controlA)
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = f.repo.ListByControl(ctx, pgTenantA, f.controlA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, ex.ID, got[0].ID)

	// Nor revoke it: ErrNotFound, apply never runs, the row and the trail are untouched.
	called := false
	_, err = f.repo.Revoke(ctx, pgTenantB, ex.ID, func(*domain.ControlException) (*domain.AuditEvent, error) {
		called = true
		return f.event(pgTenantB, ex.ID, domain.AuditActionUpdate), nil
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.False(t, errors.Is(err, domain.ErrForbidden))
	assert.False(t, called)
	got, _ = f.repo.ListByControl(ctx, pgTenantA, f.controlA)
	assert.Nil(t, got[0].RevokedAt)
	f.verifyChain(t, pgTenantA, 1)
	evsB, err := f.chain.ListAll(ctx, pgTenantB, domain.AuditEventFilter{})
	require.NoError(t, err)
	assert.Empty(t, evsB, "tenant B's trail holds nothing")

	// A fabricated id answers the same.
	_, err = f.repo.Revoke(ctx, pgTenantA, uuid.New(), func(*domain.ControlException) (*domain.AuditEvent, error) { return nil, nil })
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Tenant A can.
	now := f.now.Add(time.Hour)
	by := uuid.New()
	rev, err := f.repo.Revoke(ctx, pgTenantA, ex.ID, func(cur *domain.ControlException) (*domain.AuditEvent, error) {
		if err := cur.Revoke(by, "fixed", now); err != nil {
			return nil, err
		}
		return f.event(pgTenantA, cur.ID, domain.AuditActionUpdate), nil
	})
	require.NoError(t, err)
	assert.Equal(t, domain.ControlExceptionRevoked, rev.StatusAt(now))
	got, _ = f.repo.ListByControl(ctx, pgTenantA, f.controlA)
	require.NotNil(t, got[0].RevokedAt)
	assert.Equal(t, "fixed", got[0].RevokeReason)
	f.verifyChain(t, pgTenantA, 2)
}

func TestControlExceptionRepository_CreateWritesRowAndAuditInOneTransaction(t *testing.T) {
	f := openPG(t)
	ex, ev := f.exception(pgTenantA)
	require.NoError(t, f.repo.Create(context.Background(), ex, ev))

	assert.Equal(t, int64(1), f.count(t, &ExceptionRow{}))
	evs, err := f.chain.ListAll(context.Background(), pgTenantA, domain.AuditEventFilter{EntityType: "control_exception", EntityID: ex.ID.String()})
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, int64(1), evs[0].Sequence)
	assert.Equal(t, domain.GenesisHash, evs[0].PrevHash)
	f.verifyChain(t, pgTenantA, 1)
}

// failingAudit stands for an audit store that is down.
type failingAudit struct{}

func (failingAudit) AppendTx(context.Context, *gorm.DB, *domain.AuditEvent) error {
	return errors.New("audit store down")
}

func TestControlExceptionRepository_AuditFailureLeavesNoRow(t *testing.T) {
	f := openPG(t)
	ctx := context.Background()

	// A failing store: nothing is created.
	down := NewRepository(f.db, failingAudit{})
	ex, ev := f.exception(pgTenantA)
	require.Error(t, down.Create(ctx, ex, ev))
	assert.Zero(t, f.count(t, &ExceptionRow{}), "no exception without its audit entry")
	assert.Zero(t, f.count(t, &domain.AuditEvent{}))

	// The real store refuses an entry with no tenant: same outcome.
	ex2, ev2 := f.exception(pgTenantA)
	ev2.TenantID = uuid.Nil
	require.Error(t, f.repo.Create(ctx, ex2, ev2))
	assert.Zero(t, f.count(t, &ExceptionRow{}))

	// A revoke whose audit append fails is not stored either.
	ex3, ev3 := f.exception(pgTenantA)
	require.NoError(t, f.repo.Create(ctx, ex3, ev3))
	_, err := down.Revoke(ctx, pgTenantA, ex3.ID, func(cur *domain.ControlException) (*domain.AuditEvent, error) {
		require.NoError(t, cur.Revoke(uuid.New(), "", f.now.Add(time.Hour)))
		return f.event(pgTenantA, cur.ID, domain.AuditActionUpdate), nil
	})
	require.Error(t, err)
	got, err := f.repo.ListByControl(ctx, pgTenantA, f.controlA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Nil(t, got[0].RevokedAt, "the revocation rolled back with the audit failure")
	f.verifyChain(t, pgTenantA, 1)
}

func TestControlExceptionRepository_RefusesBadRowsAtTheDatabase(t *testing.T) {
	f := openPG(t)
	good, _ := f.exception(pgTenantA)

	bad := rowOf(good)
	bad.ID = uuid.New()
	bad.ExpiresAt = bad.StartsAt
	assert.Error(t, f.db.Create(bad).Error, "expires_at must be after starts_at")

	bad = rowOf(good)
	bad.ID = uuid.New()
	bad.Reason = "   "
	assert.Error(t, f.db.Create(bad).Error, "a blank reason is refused")

	bad = rowOf(good)
	bad.ID = uuid.New()
	at := f.now
	bad.RevokedAt = &at
	assert.Error(t, f.db.Create(bad).Error, "revoked_at without revoked_by is refused")
}
