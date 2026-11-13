// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #344 — the transaction-scoped append and the sequence counter, on sqlite. The
// relay's claim needs Postgres and is tested in realtime_outbox_pg_test.go.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/pkg/realtime"
)

func TestRealtimeRepo_InlineAppendIsPublished(t *testing.T) {
	repo := setupRealtimeRepo(t)
	e := newEvent(t, uuid.New(), realtime.RiskCreated, "r1")
	require.NoError(t, repo.Append(context.Background(), e))

	var row domain.RealtimeEvent
	require.NoError(t, repo.db.Where("id = ? AND tenant_id = ?", e.ID, e.TenantID).Take(&row).Error)
	require.NotNil(t, row.PublishedAt, "the post-commit path stores the row already published: the relay must never see it")
	assert.Nil(t, row.AbandonedAt)
	assert.Zero(t, row.Attempts)
}

func TestRealtimeRepo_EnlistAssignsContiguousSequencesInTx(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	require.NoError(t, repo.Append(ctx, newEvent(t, tenant, realtime.RiskCreated, "r0")))

	var batch []*domain.RealtimeEvent
	for i := 0; i < 3; i++ {
		batch = append(batch, newEvent(t, tenant, realtime.AssetCreated, uuid.NewString()))
	}
	woken := 0
	repo.OnEnlistCommitted(func() { woken++ })
	err := repo.db.Transaction(func(tx *gorm.DB) error {
		return repo.Bind(tx).Enlist(ctx, batch...)
	})
	require.NoError(t, err)
	for i, e := range batch {
		assert.Equal(t, int64(2+i), e.Sequence, "after the inline event, enlisted rows take consecutive sequences in slice order")
		require.NotNil(t, e.ID)
	}

	rows, err := repo.Replay(ctx, tenant, 0, 10)
	require.NoError(t, err)
	require.Len(t, rows, 4, "Replay serves committed rows, pending or not")
	for _, r := range rows[1:] {
		assert.Nil(t, r.PublishedAt, "enlisted rows are born pending")
	}

	next := newEvent(t, tenant, realtime.RiskUpdated, "r0")
	require.NoError(t, repo.Append(ctx, next))
	assert.Equal(t, int64(5), next.Sequence)
	assert.Zero(t, woken, "Enlist itself never wakes the relay: only a commit does")
}

func TestRealtimeRepo_EnlistRollsBackWithTheCaller(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	err := repo.db.Transaction(func(tx *gorm.DB) error {
		if err := repo.Bind(tx).Enlist(ctx, newEvent(t, tenant, realtime.AssetCreated, "a1")); err != nil {
			return err
		}
		return errors.New("business failure")
	})
	require.Error(t, err)

	var n int64
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	assert.Zero(t, n, "a rolled-back transaction leaves no event")

	// The counter rolled back too: the next event is sequence 1, not 2.
	e := newEvent(t, tenant, realtime.AssetCreated, "a2")
	require.NoError(t, repo.Append(ctx, e))
	assert.Equal(t, int64(1), e.Sequence)
}

func TestRealtimeRepo_EnlistRefusesMixedTenants(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	err := repo.db.Transaction(func(tx *gorm.DB) error {
		return repo.Bind(tx).Enlist(ctx,
			newEvent(t, a, realtime.AssetCreated, "a1"), newEvent(t, b, realtime.AssetCreated, "b1"))
	})
	require.ErrorIs(t, err, domain.ErrValidation)

	var n int64
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).Where("tenant_id IN ?", []uuid.UUID{a, b}).Count(&n).Error)
	assert.Zero(t, n, "nothing is stored when the call is refused")
}

func TestRealtimeRepo_EnlistRefusesNilTenantNilIDAndNil(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	noTenant := newEvent(t, uuid.Nil, realtime.AssetCreated, "a1")
	noID := newEvent(t, uuid.New(), realtime.AssetCreated, "a1")
	noID.ID = uuid.Nil
	for name, ev := range map[string]*domain.RealtimeEvent{"nil tenant": noTenant, "nil id": noID, "nil event": nil} {
		err := repo.db.Transaction(func(tx *gorm.DB) error { return repo.Bind(tx).Enlist(ctx, ev) })
		require.ErrorIs(t, err, domain.ErrValidation, name)
	}
	// Zero events is a no-op, not an error.
	require.NoError(t, repo.db.Transaction(func(tx *gorm.DB) error { return repo.Bind(tx).Enlist(ctx) }))
}

func TestRealtimeRepo_PurgeKeepsPendingRows(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	pending := newEvent(t, tenant, realtime.AssetCreated, "p")
	abandoned := newEvent(t, tenant, realtime.AssetCreated, "x")
	require.NoError(t, repo.db.Transaction(func(tx *gorm.DB) error {
		return repo.Bind(tx).Enlist(ctx, pending, abandoned)
	}))
	published := newEvent(t, tenant, realtime.AssetCreated, "d")
	require.NoError(t, repo.Append(ctx, published))

	old := time.Now().UTC().Add(-72 * time.Hour)
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", tenant).Update("created_at", old).Error)
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).Where("id = ? AND tenant_id = ?", abandoned.ID, tenant).
		Update("abandoned_at", time.Now().UTC()).Error)

	n, err := repo.PurgeBefore(ctx, time.Now().UTC().Add(-domain.DefaultReplayRetention))
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "the published and the abandoned row go")

	var left []domain.RealtimeEvent
	require.NoError(t, repo.db.Where("tenant_id = ?", tenant).Find(&left).Error)
	require.Len(t, left, 1)
	assert.Equal(t, pending.ID, left[0].ID, "a pending row is never purged, whatever its age")
}

// The counter lives in its own table, so purging every row of a tenant does not
// make its sequence start over (a client holding sequence N must never be
// handed a different event under N).
func TestRealtimeRepo_SequenceSurvivesFullPurge(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Append(ctx, newEvent(t, tenant, realtime.RiskUpdated, "r1")))
	}
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", tenant).
		Update("created_at", time.Now().UTC().Add(-72*time.Hour)).Error)
	n, err := repo.PurgeBefore(ctx, time.Now().UTC().Add(-domain.DefaultReplayRetention))
	require.NoError(t, err)
	require.Equal(t, int64(3), n)

	e := newEvent(t, tenant, realtime.RiskUpdated, "r1")
	require.NoError(t, repo.Append(ctx, e))
	assert.Equal(t, int64(4), e.Sequence, "the sequence continues after a full purge")
}

// A tenant that has rows but no counter row (data from before the counter, or
// written by an older binary during a rolling deploy) is seeded from the rows.
func TestRealtimeRepo_CounterSeedsAndHealsFromExistingRows(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	legacy := newEvent(t, tenant, realtime.RiskCreated, "r1")
	legacy.Sequence = 7
	legacy.CreatedAt = time.Now().UTC()
	require.NoError(t, repo.db.Create(legacy).Error) // no counter row exists

	e := newEvent(t, tenant, realtime.RiskUpdated, "r1")
	require.NoError(t, repo.Append(ctx, e))
	assert.Equal(t, int64(8), e.Sequence)

	// An older binary appends by MAX+1 behind the counter's back.
	rogue := newEvent(t, tenant, realtime.RiskUpdated, "r1")
	rogue.Sequence = 20
	rogue.CreatedAt = time.Now().UTC()
	require.NoError(t, repo.db.Create(rogue).Error)

	next := newEvent(t, tenant, realtime.RiskUpdated, "r1")
	require.NoError(t, repo.Append(ctx, next))
	assert.Equal(t, int64(21), next.Sequence, "the counter never falls below the rows it has to sit after")
}

func TestRealtimeRepo_EnlistKeepsTenantsIndependent(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ea, eb := newEvent(t, a, realtime.AssetCreated, "a"), newEvent(t, b, realtime.AssetCreated, "b")
	require.NoError(t, repo.db.Transaction(func(tx *gorm.DB) error {
		o := repo.Bind(tx)
		if err := o.Enlist(ctx, ea); err != nil {
			return err
		}
		return o.Enlist(ctx, eb)
	}))
	assert.Equal(t, int64(1), ea.Sequence)
	assert.Equal(t, int64(1), eb.Sequence, "each tenant has its own 1..N")
}

// --- the pilot, real RunAssetTx on sqlite ------------------------------------

func assetImportDB(t *testing.T) (*gorm.DB, *GormRealtimeEventRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:rt_outbox_"+uuid.NewString()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
		criticality TEXT NOT NULL DEFAULT 'MEDIUM', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "assets", &domain.Asset{}))
	require.NoError(t, db.AutoMigrate(&domain.RealtimeEvent{}, &domain.RealtimeEventSequence{}))
	return db, NewGormRealtimeEventRepository(db)
}

func importUC(db *gorm.DB, runner assetuc.AssetTxRunner) *assetuc.ImportAssetsUseCase {
	return assetuc.NewImportAssetsUseCase(runner, ListAssetNames(db))
}

func TestImportAssets_EnlistsEventsInTheSameTransaction(t *testing.T) {
	db, events := assetImportDB(t)
	woken := 0
	events.OnEnlistCommitted(func() { woken++ })
	tenant := uuid.New()
	ctx := realtime.WithEnlistMark(context.Background())

	res, err := importUC(db, RunAssetTx(db, events)).Execute(ctx, tenant, assetuc.ImportAssetsInput{
		CSV: []byte("name\nA\nB\nC\n"), CorrelationID: "req-7",
	})
	require.NoError(t, err)
	require.Equal(t, 3, res.Created)

	var rows []domain.RealtimeEvent
	require.NoError(t, db.Where("tenant_id = ?", tenant).Order("sequence").Find(&rows).Error)
	require.Len(t, rows, 3)
	for i, r := range rows {
		assert.Equal(t, int64(i+1), r.Sequence)
		assert.Equal(t, res.AssetIDs[i].String(), r.AggregateID)
		assert.Nil(t, r.PublishedAt, "the relay, not the request, publishes")
		assert.Equal(t, "req-7", r.CorrelationID)
	}
	assert.Equal(t, 1, woken, "the relay is woken once, after the commit")
	assert.True(t, realtime.Enlisted(ctx))
}

func TestImportAssets_RollbackLeavesNoEvent(t *testing.T) {
	db, events := assetImportDB(t)
	woken := 0
	events.OnEnlistCommitted(func() { woken++ })
	tenant := uuid.New()
	ctx := realtime.WithEnlistMark(context.Background())

	// A failure AFTER the events were enlisted, as a commit-time error would be.
	real := RunAssetTx(db, events)
	failing := func(ctx context.Context, fn func(repo domain.AssetRepository, ev domain.RealtimeEventOutbox) error) error {
		return real(ctx, func(repo domain.AssetRepository, ev domain.RealtimeEventOutbox) error {
			if err := fn(repo, ev); err != nil {
				return err
			}
			return errors.New("commit failed")
		})
	}
	_, err := importUC(db, failing).Execute(ctx, tenant, assetuc.ImportAssetsInput{CSV: []byte("name\nA\nB\n")})
	require.Error(t, err)

	var assets, rows, counters int64
	require.NoError(t, db.Model(&domain.Asset{}).Where("tenant_id = ?", tenant).Count(&assets).Error)
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", tenant).Count(&rows).Error)
	require.NoError(t, db.Model(&domain.RealtimeEventSequence{}).Where("tenant_id = ?", tenant).Count(&counters).Error)
	assert.Zero(t, assets, "the assets rolled back")
	assert.Zero(t, rows, "and no realtime event outlived them")
	assert.Zero(t, counters, "nor did the sequence counter")
	assert.Zero(t, woken, "the relay is never woken for a rolled-back transaction")
	assert.False(t, realtime.Enlisted(ctx), "and the bridge is not told this request announced anything")
}

// Without an outbox the runner hands the use case nil, nothing is enlisted and
// the post-commit bridge keeps its single event.
func TestImportAssets_WithoutOutboxEnlistsNothing(t *testing.T) {
	db, _ := assetImportDB(t)
	tenant := uuid.New()
	ctx := realtime.WithEnlistMark(context.Background())
	_, err := importUC(db, RunAssetTx(db, nil)).Execute(ctx, tenant, assetuc.ImportAssetsInput{CSV: []byte("name\nA\n")})
	require.NoError(t, err)
	var rows int64
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", tenant).Count(&rows).Error)
	assert.Zero(t, rows)
	assert.False(t, realtime.Enlisted(ctx))
}
