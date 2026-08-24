// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/realtime"
)

// The schema comes from AutoMigrate on the model, never from hand-written DDL.
// A hand-written CREATE TABLE is how this repository's table would quietly drift
// away from the struct it is supposed to store — the exact failure the audit
// trail hit before its model was made sqlite-compatible.
func setupRealtimeRepo(t *testing.T) *GormRealtimeEventRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.RealtimeEvent{}))
	return NewGormRealtimeEventRepository(db)
}

func newEvent(t *testing.T, tenant uuid.UUID, typ realtime.EventType, aggID string) *domain.RealtimeEvent {
	t.Helper()
	desc, ok := realtime.Lookup(typ)
	require.True(t, ok, "test used an event type outside the catalog")
	return &domain.RealtimeEvent{
		ID:              uuid.New(),
		TenantID:        tenant,
		Type:            string(typ),
		Version:         desc.Version,
		EnvelopeVersion: realtime.EnvelopeVersion,
		AggregateType:   desc.Aggregate,
		AggregateID:     aggID,
		OccurredAt:      time.Now().UTC(),
	}
}

func TestRealtimeRepo_AssignsAMonotonicPerTenantSequence(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	for i := 1; i <= 5; i++ {
		e := newEvent(t, tenant, realtime.RiskCreated, uuid.NewString())
		// A caller trying to choose its own position must be overruled.
		e.Sequence = 999
		require.NoError(t, repo.Append(ctx, e))
		assert.Equal(t, int64(i), e.Sequence, "sequence must start at 1 and increment by 1")
	}
}

// Two tenants publishing side by side must each get their own 1..N, or a client
// could infer another tenant's activity from the gaps in its own sequence.
func TestRealtimeRepo_SequencesAreIndependentPerTenant(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	e1 := newEvent(t, a, realtime.RiskCreated, "r1")
	require.NoError(t, repo.Append(ctx, e1))
	e2 := newEvent(t, b, realtime.AssetCreated, "a1")
	require.NoError(t, repo.Append(ctx, e2))
	e3 := newEvent(t, a, realtime.RiskUpdated, "r1")
	require.NoError(t, repo.Append(ctx, e3))

	assert.Equal(t, int64(1), e1.Sequence)
	assert.Equal(t, int64(1), e2.Sequence, "tenant B starts its own count at 1")
	assert.Equal(t, int64(2), e3.Sequence)
}

// The mandatory cross-tenant property at the storage layer: a replay is driven
// by a client-supplied cursor, so it is exactly where a tenant boundary would be
// crossed if the query forgot its predicate.
func TestRealtimeRepo_ReplayNeverCrossesTheTenantBoundary(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	require.NoError(t, repo.Append(ctx, newEvent(t, a, realtime.RiskCreated, "risk-A")))
	require.NoError(t, repo.Append(ctx, newEvent(t, b, realtime.AssetCreated, "asset-B")))
	require.NoError(t, repo.Append(ctx, newEvent(t, b, realtime.AssetUpdated, "asset-B")))

	fromA, err := repo.Replay(ctx, a, 0, 100)
	require.NoError(t, err)
	require.Len(t, fromA, 1)
	assert.Equal(t, "risk-A", fromA[0].AggregateID)

	fromB, err := repo.Replay(ctx, b, 0, 100)
	require.NoError(t, err)
	require.Len(t, fromB, 2)
	for _, e := range fromB {
		assert.Equal(t, b, e.TenantID)
		assert.NotEqual(t, "risk-A", e.AggregateID)
	}
}

// A nil tenant must fail rather than return everything: an unscoped query here
// is the worst outcome this file can produce.
func TestRealtimeRepo_ReplayFailsClosedWithoutATenant(t *testing.T) {
	repo := setupRealtimeRepo(t)
	_, err := repo.Replay(context.Background(), uuid.Nil, 0, 10)
	require.Error(t, err)

	_, _, err = repo.Bounds(context.Background(), uuid.Nil)
	require.Error(t, err)
}

func TestRealtimeRepo_ReplayReturnsOldestFirstAfterTheCursor(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	for i := 0; i < 4; i++ {
		require.NoError(t, repo.Append(ctx, newEvent(t, tenant, realtime.RiskUpdated, "r1")))
	}

	got, err := repo.Replay(ctx, tenant, 2, 100)
	require.NoError(t, err)
	require.Len(t, got, 2, "a cursor of 2 must yield 3 and 4")
	assert.Equal(t, int64(3), got[0].Sequence)
	assert.Equal(t, int64(4), got[1].Sequence, "a replay must be applied in the order things happened")
}

func TestRealtimeRepo_ReplayLimitIsCapped(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	require.NoError(t, repo.Append(ctx, newEvent(t, tenant, realtime.RiskCreated, "r1")))

	// A client asking for a million rows gets the cap, not a million rows.
	got, err := repo.Replay(ctx, tenant, 0, 1_000_000)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestRealtimeRepo_BoundsDescribeTheReplayWindow(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	oldest, newest, err := repo.Bounds(ctx, tenant)
	require.NoError(t, err)
	assert.Zero(t, oldest)
	assert.Zero(t, newest, "a tenant with no events has an empty window, not an error")

	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Append(ctx, newEvent(t, tenant, realtime.RiskCreated, "r1")))
	}
	oldest, newest, err = repo.Bounds(ctx, tenant)
	require.NoError(t, err)
	assert.Equal(t, int64(1), oldest)
	assert.Equal(t, int64(3), newest)
}

// After a purge the window moves forward. A client holding a cursor below the
// new oldest must be told to resync — that decision needs Bounds to report the
// truth, which is what this asserts.
func TestRealtimeRepo_PurgeMovesTheReplayWindowForward(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	old := newEvent(t, tenant, realtime.RiskCreated, "r1")
	require.NoError(t, repo.Append(ctx, old))
	require.NoError(t, repo.db.Model(&domain.RealtimeEvent{}).
		Where("id = ?", old.ID).
		Update("created_at", time.Now().UTC().Add(-48*time.Hour)).Error)

	fresh := newEvent(t, tenant, realtime.RiskUpdated, "r1")
	require.NoError(t, repo.Append(ctx, fresh))

	n, err := repo.PurgeBefore(ctx, time.Now().UTC().Add(-domain.DefaultReplayRetention))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	oldest, newest, err := repo.Bounds(ctx, tenant)
	require.NoError(t, err)
	assert.Equal(t, int64(2), oldest, "the window now starts after the purged event")
	assert.Equal(t, int64(2), newest)

	// The sequence must NOT be reused after a purge: a client that saw event 1
	// and reconnects must not be handed a different event under the same number.
	next := newEvent(t, tenant, realtime.RiskDeleted, "r1")
	require.NoError(t, repo.Append(ctx, next))
	assert.Equal(t, int64(3), next.Sequence)
}

// Concurrent publishers must not collide on a position. The unique index is the
// backstop; if serialisation were broken this fails with a duplicate key.
func TestRealtimeRepo_ConcurrentAppendsProduceDistinctSequences(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	const n = 25
	var wg sync.WaitGroup
	errs := make([]error, n)
	seqs := make([]int64, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			e := newEvent(t, tenant, realtime.RiskUpdated, "r1")
			errs[i] = repo.Append(ctx, e)
			seqs[i] = e.Sequence
		}(i)
	}
	wg.Wait()

	seen := map[int64]bool{}
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.False(t, seen[seqs[i]], "sequence %d was handed out twice", seqs[i])
		seen[seqs[i]] = true
	}
	assert.Len(t, seen, n)
}

// A stored row must rebuild into the same envelope that went in: a replayed
// event and a live event have to be indistinguishable to a client.
func TestRealtimeRepo_StoredRowRebuildsAValidEnvelope(t *testing.T) {
	repo := setupRealtimeRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	actor := uuid.New()

	e := newEvent(t, tenant, realtime.RiskStatusChanged, "risk-1")
	e.ActorID = &actor
	e.CorrelationID = "req-42"
	e.CausationID = "audit-42"
	e.Payload = domain.JSONMap{"changedFields": []string{"status"}}
	require.NoError(t, repo.Append(ctx, e))

	rows, err := repo.Replay(ctx, tenant, 0, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	env := rows[0].ToEnvelope()
	require.NoError(t, env.Validate(), "a replayed event must satisfy the same contract as a live one")
	assert.Equal(t, e.ID.String(), env.ID)
	assert.Equal(t, tenant.String(), env.TenantID)
	assert.Equal(t, actor.String(), env.ActorID)
	assert.Equal(t, int64(1), env.Sequence)
	assert.Equal(t, "req-42", env.CorrelationID)
	assert.Equal(t, "audit-42", env.CausationID)
}
