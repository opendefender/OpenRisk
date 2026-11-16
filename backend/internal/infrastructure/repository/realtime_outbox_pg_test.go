// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #344 — the realtime outbox on real Postgres. The claim takes row locks and
// uses SKIP LOCKED and window functions; sqlite can show none of it. Each test
// builds a schema of its own and drops it afterwards. Skipped without
// DATABASE_URL.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	apprealtime "github.com/opendefender/openrisk/internal/application/realtime"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
	"github.com/opendefender/openrisk/pkg/monitoring"
	"github.com/opendefender/openrisk/pkg/realtime"
)

const (
	rtLogMigration    = "../../../../migrations/0059_realtime_event_log"
	rtOutboxMigration = "../../../../migrations/0085_realtime_outbox"
)

// rtSchema opens a pool whose connections all see a fresh schema.
func rtSchema(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "rt344_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	admin, err := testdb.Open(t, postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return rtOpen(t, dsn, schema)
}

// rtOpen opens a new pool on an existing schema.
func rtOpen(t *testing.T, dsn, schema string) *gorm.DB {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := testdb.Open(t, postgres.Open(u.String()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
		// The pilot's Asset pulls in risks through its many-to-many link; the
		// tests are about events, so constraints toward tables they never touch
		// are left out.
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(30)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// reopened returns a fresh pool on the same schema. After a migration changes
// the shape of SELECT *, a connection that already prepared it fails once with
// "cached plan must not change result type"; a deployment runs the migration
// before the new binary starts, which is what a new pool models.
func reopened(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	var schema string
	require.NoError(t, db.Raw(`SELECT current_schema()`).Scan(&schema).Error)
	return rtOpen(t, os.Getenv("DATABASE_URL"), schema)
}

// rtOutboxDB builds the schema the way a compose boot does: AutoMigrate.
func rtOutboxDB(t *testing.T, models ...interface{}) (*gorm.DB, *GormRealtimeEventRepository) {
	t.Helper()
	db := rtSchema(t)
	models = append(models, &domain.RealtimeEvent{}, &domain.RealtimeEventSequence{})
	require.NoError(t, db.AutoMigrate(models...))
	return db, NewGormRealtimeEventRepository(db)
}

// enlistIn enlists n events for tenant in one transaction that commits.
func enlistIn(t *testing.T, db *gorm.DB, repo *GormRealtimeEventRepository, tenant uuid.UUID, n int) []*domain.RealtimeEvent {
	t.Helper()
	evs := make([]*domain.RealtimeEvent, n)
	for i := range evs {
		evs[i] = newEvent(t, tenant, realtime.AssetCreated, uuid.NewString())
	}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return repo.Bind(tx).Enlist(context.Background(), evs...) }))
	return evs
}

func rowOf(t *testing.T, db *gorm.DB, tenant, id uuid.UUID) domain.RealtimeEvent {
	t.Helper()
	var row domain.RealtimeEvent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", tenant, id).Take(&row).Error)
	return row
}

func pendingCount(t *testing.T, db *gorm.DB, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).
		Where("tenant_id = ? AND published_at IS NULL AND abandoned_at IS NULL", tenant).Count(&n).Error)
	return n
}

// capSink records every delivery in order, with the id and tenant.
type capSink struct {
	mu  sync.Mutex
	got []realtime.Envelope
}

func (s *capSink) Deliver(_ context.Context, env realtime.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, env)
	return nil
}

type togglingSink struct {
	mu   sync.Mutex
	fail bool
	got  []realtime.Envelope
}

func (s *togglingSink) setFail(f bool) { s.mu.Lock(); s.fail = f; s.mu.Unlock() }
func (s *togglingSink) Deliver(_ context.Context, env realtime.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return fmt.Errorf("redis down")
	}
	s.got = append(s.got, env)
	return nil
}

// --- AC1: crash after COMMIT, before any publication -------------------------

func TestRealtimeOutbox_CrashAfterCommit_RelayPublishesOnce(t *testing.T) {
	db, events := rtOutboxDB(t, &domain.Asset{})
	tenant := uuid.New()

	// The import commits through the real runner. No relay is running: that is
	// the process dying right after COMMIT.
	res, err := assetuc.NewImportAssetsUseCase(RunAssetTx(db, events), ListAssetNames(db)).
		Execute(context.Background(), tenant, assetuc.ImportAssetsInput{CSV: []byte("name\nA\nB\nC\nD\n")})
	require.NoError(t, err)
	require.Equal(t, 4, res.Created)

	var assets int64
	require.NoError(t, db.Model(&domain.Asset{}).Where("tenant_id = ?", tenant).Count(&assets).Error)
	require.EqualValues(t, 4, assets, "the business rows survived the crash")
	require.EqualValues(t, 4, pendingCount(t, db, tenant), "and so did their events, unpublished")

	// After the restart a NEW relay, with a real hub and one subscriber.
	hub := apprealtime.NewHub(apprealtime.HubOptions{})
	sub, err := hub.Subscribe(tenant, realtime.Filter{})
	require.NoError(t, err)
	defer hub.Unsubscribe(sub)
	relay := apprealtime.NewOutboxRelay(events, apprealtime.NewHubSink(hub, nil))

	for i := 0; i < 2; i++ { // twice: the second pass must find nothing
		_, _, err := relay.Pass(context.Background())
		require.NoError(t, err)
	}

	seen := map[string]int{}
	for len(sub.Events()) > 0 {
		env := <-sub.Events()
		seen[env.Aggregate.ID]++
		assert.Equal(t, string(realtime.AssetCreated), string(env.Type))
	}
	require.Len(t, seen, 4)
	for _, id := range res.AssetIDs {
		assert.Equal(t, 1, seen[id.String()], "asset %s announced exactly once", id)
	}
	assert.Zero(t, pendingCount(t, db, tenant))
	var unpublished int64
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).Where("tenant_id = ? AND published_at IS NULL", tenant).Count(&unpublished).Error)
	assert.Zero(t, unpublished, "every row ends with published_at set")
}

// --- AC3: a failing sink keeps the row pending, counted, backed off ----------

func TestRealtimeOutbox_RetryKeepsRowPending(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	ev := enlistIn(t, db, events, tenant, 1)[0]
	sink := &togglingSink{fail: true}
	relay := apprealtime.NewOutboxRelay(events, sink)
	before := testutil.ToFloat64(monitoring.RealtimeOutboxPublishFailuresTotal)

	_, _, err := relay.Pass(context.Background())
	require.NoError(t, err)

	row := rowOf(t, db, tenant, ev.ID)
	assert.Nil(t, row.PublishedAt, "a failed delivery leaves the row unpublished")
	assert.Equal(t, 1, row.Attempts, "the attempt is counted")
	require.NotNil(t, row.LockedUntil)
	assert.True(t, row.LockedUntil.After(time.Now()), "and held back until the backoff passes")
	assert.Equal(t, "redis down", row.LastError)
	assert.Equal(t, 1.0, testutil.ToFloat64(monitoring.RealtimeOutboxPublishFailuresTotal)-before)

	// Backed off: the next pass does not even claim it.
	claimed, _, err := relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, claimed, "a backed-off row is not claimed")

	// The backoff passes and the sink recovers: delivered, closed, attempts=2.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() - interval '1 second' WHERE tenant_id = ? AND id = ?`, tenant, ev.ID).Error)
	sink.setFail(false)
	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	row = rowOf(t, db, tenant, ev.ID)
	require.NotNil(t, row.PublishedAt)
	assert.Equal(t, 2, row.Attempts)
	assert.Nil(t, row.LockedUntil)
	require.Len(t, sink.got, 1)
	assert.Equal(t, ev.ID.String(), sink.got[0].ID)
}

func TestRealtimeOutbox_AbandonsAtAttemptCapAndKeepsTheRow(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	ev := enlistIn(t, db, events, tenant, 1)[0]
	sink := &togglingSink{fail: true}
	relay := apprealtime.NewOutboxRelay(events, sink, apprealtime.WithOutboxAttempts(2, time.Millisecond, time.Millisecond))
	before := testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)

	for i := 0; i < 2; i++ {
		_, _, err := relay.Pass(context.Background())
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond) // the 1ms backoff passes
	}
	row := rowOf(t, db, tenant, ev.ID)
	assert.NotNil(t, row.AbandonedAt, "given up at the cap")
	assert.Nil(t, row.PublishedAt)
	assert.Equal(t, 2, row.Attempts)
	assert.Equal(t, 1.0, testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)-before)
	assert.EqualValues(t, 0, pendingCount(t, db, tenant), "an abandoned row is no longer pending")

	replay, err := events.Replay(context.Background(), tenant, 0, 10)
	require.NoError(t, err)
	assert.Len(t, replay, 1, "a replay still serves it")
}

func TestRealtimeOutbox_SweepAbandonsARowThatCrashedTheRelayEveryTime(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	ev := enlistIn(t, db, events, tenant, 1)[0]
	// Claimed 20 times, lease long gone, no outcome ever recorded.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET attempts = 20, locked_until = now() - interval '1 minute' WHERE tenant_id = ? AND id = ?`, tenant, ev.ID).Error)

	n, err := events.AbandonExpired(context.Background(), 20)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NotNil(t, rowOf(t, db, tenant, ev.ID).AbandonedAt)
}

// --- AC4: two relays, each row once; redelivery keeps the id ----------------

func TestRealtimeOutbox_TwoRelaysClaimEachRowOnce(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenants := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	want := map[uuid.UUID][]uuid.UUID{}
	for _, tn := range tenants {
		for i := 0; i < 5; i++ { // several transactions per tenant
			for _, e := range enlistIn(t, db, events, tn, 6) {
				want[tn] = append(want[tn], e.ID)
			}
		}
	}

	sink := &capSink{}
	// Small batches force many passes and plenty of contention.
	mk := func() *apprealtime.OutboxRelay {
		return apprealtime.NewOutboxRelay(events, sink, apprealtime.WithOutboxBatch(2, 7))
	}
	var wg sync.WaitGroup
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func(relay *apprealtime.OutboxRelay) {
			defer wg.Done()
			idle := 0
			for idle < 5 {
				claimed, _, err := relay.Pass(context.Background())
				if err != nil {
					t.Errorf("pass: %v", err)
					return
				}
				if claimed == 0 {
					idle++
					time.Sleep(10 * time.Millisecond)
				} else {
					idle = 0
				}
			}
		}(mk())
	}
	wg.Wait()

	seen := map[string]int{}
	order := map[string][]int64{}
	for _, env := range sink.got {
		seen[env.ID]++
		order[env.TenantID] = append(order[env.TenantID], env.Sequence)
	}
	for _, tn := range tenants {
		require.Len(t, order[tn.String()], 30)
		for _, id := range want[tn] {
			assert.Equal(t, 1, seen[id.String()], "event %s delivered once", id)
		}
		for i := 1; i < len(order[tn.String()]); i++ {
			assert.Greater(t, order[tn.String()][i], order[tn.String()][i-1], "tenant %s delivered in sequence order", tn)
		}
		assert.Zero(t, pendingCount(t, db, tn))
	}
}

func TestRealtimeOutbox_LeaseExpiryRedeliversSameID(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	ev := enlistIn(t, db, events, tenant, 1)[0]

	// Relay 1 claims and dies before delivering.
	claimed, err := events.ClaimPending(context.Background(), 16, 100, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, ev.ID, claimed[0].ID)

	// Leased: relay 2 must not take it.
	sink := &capSink{}
	relay2 := apprealtime.NewOutboxRelay(events, sink)
	n, _, err := relay2.Pass(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n, "a leased row is claimed by one relay at a time")
	assert.Empty(t, sink.got)

	// The lease expires.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() - interval '1 second' WHERE tenant_id = ? AND id = ?`, tenant, ev.ID).Error)
	_, _, err = relay2.Pass(context.Background())
	require.NoError(t, err)
	require.Len(t, sink.got, 1)
	assert.Equal(t, ev.ID.String(), sink.got[0].ID, "the redelivery carries the SAME id, so consumers deduplicate")
	row := rowOf(t, db, tenant, ev.ID)
	assert.Equal(t, 2, row.Attempts)
	assert.NotNil(t, row.PublishedAt)
}

// --- AC5: per-tenant order and isolation ------------------------------------

func TestRealtimeOutbox_PerTenantOrderAndIsolation(t *testing.T) {
	db, events := rtOutboxDB(t)
	a, b := uuid.New(), uuid.New()
	ctx := context.Background()

	// Tenant A: tx1 enlists and stays open; tx2 enlists in a goroutine and must
	// wait for the counter.
	tx1 := db.Begin()
	require.NoError(t, tx1.Error)
	e1 := []*domain.RealtimeEvent{newEvent(t, a, realtime.AssetCreated, "a1"), newEvent(t, a, realtime.AssetCreated, "a2")}
	require.NoError(t, events.Bind(tx1).Enlist(ctx, e1...))

	tx2 := db.Begin()
	require.NoError(t, tx2.Error)
	e2 := []*domain.RealtimeEvent{newEvent(t, a, realtime.AssetCreated, "a3")}
	enlisted := make(chan error, 1)
	go func() { enlisted <- events.Bind(tx2).Enlist(ctx, e2...) }()
	select {
	case err := <-enlisted:
		t.Fatalf("tx2 must wait for tx1's counter lock, returned %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	// Tenant B has committed rows meanwhile. A relay pass while A's transaction
	// is open must deliver B and nothing of A (A's counter is locked).
	eb := enlistIn(t, db, events, b, 3)
	sink := &capSink{}
	relay := apprealtime.NewOutboxRelay(events, sink)
	_, _, err := relay.Pass(ctx)
	require.NoError(t, err)
	require.Len(t, sink.got, 3)
	for _, env := range sink.got {
		assert.Equal(t, b.String(), env.TenantID)
	}

	require.NoError(t, tx1.Commit().Error)
	require.NoError(t, <-enlisted)
	require.NoError(t, tx2.Commit().Error)
	assert.Equal(t, int64(1), e1[0].Sequence)
	assert.Equal(t, int64(2), e1[1].Sequence)
	assert.Equal(t, int64(3), e2[0].Sequence, "tx2 queued behind tx1 and took the next positions")

	// Back off A's head: the pass delivers other tenants only, none of A's.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() + interval '1 hour' WHERE tenant_id = ? AND id = ?`, a, e1[0].ID).Error)
	more := enlistIn(t, db, events, b, 2)
	sink.mu.Lock()
	sink.got = nil
	sink.mu.Unlock()
	_, _, err = relay.Pass(ctx)
	require.NoError(t, err)
	require.Len(t, sink.got, 2)
	for _, env := range sink.got {
		assert.Equal(t, b.String(), env.TenantID, "a backed-off head blocks the rest of its own tenant")
	}
	assert.Equal(t, more[0].ID.String(), sink.got[0].ID)
	assert.EqualValues(t, 3, pendingCount(t, db, a))

	// Expire it: A comes out strictly in ascending order.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() - interval '1 second' WHERE tenant_id = ? AND id = ?`, a, e1[0].ID).Error)
	sink.mu.Lock()
	sink.got = nil
	sink.mu.Unlock()
	_, _, err = relay.Pass(ctx)
	require.NoError(t, err)
	require.Len(t, sink.got, 3)
	for i, env := range sink.got {
		assert.Equal(t, a.String(), env.TenantID)
		assert.Equal(t, int64(i+1), env.Sequence)
	}
	_ = eb
}

// Every envelope the real HubSink delivers is published on the channel of the
// tenant its row belongs to, and a subscriber of one tenant never receives
// another tenant's event.
func TestRealtimeOutbox_HubSinkKeepsTenantsApart(t *testing.T) {
	db, events := rtOutboxDB(t)
	a, b := uuid.New(), uuid.New()
	ea := enlistIn(t, db, events, a, 3)
	enlistIn(t, db, events, b, 3)

	hub := apprealtime.NewHub(apprealtime.HubOptions{})
	subB, err := hub.Subscribe(b, realtime.Filter{})
	require.NoError(t, err)
	defer hub.Unsubscribe(subB)
	fan := &recordingFanout{}
	_, _, err = apprealtime.NewOutboxRelay(events, apprealtime.NewHubSink(hub, fan)).Pass(context.Background())
	require.NoError(t, err)

	require.Len(t, fan.calls, 6)
	for _, c := range fan.calls {
		tenant, err := uuid.Parse(c.env.TenantID)
		require.NoError(t, err)
		assert.Equal(t, apprealtime.TenantChannel(tenant), c.channel, "the channel is the row's tenant's")
	}
	aIDs := map[string]bool{}
	for _, e := range ea {
		aIDs[e.ID.String()] = true
	}
	require.Len(t, subB.Events(), 3)
	for len(subB.Events()) > 0 {
		env := <-subB.Events()
		assert.Equal(t, b.String(), env.TenantID)
		assert.False(t, aIDs[env.ID], "tenant B never receives tenant A's event")
	}
}

type fanCall struct {
	channel string
	env     realtime.Envelope
}
type recordingFanout struct {
	mu    sync.Mutex
	calls []fanCall
}

func (f *recordingFanout) Publish(_ context.Context, channel string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	env, _ := payload.(realtime.Envelope)
	f.calls = append(f.calls, fanCall{channel, env})
	return nil
}

// The first event of a fresh tenant has no head row to lock. Two transactions
// racing for it must still get 1 and 2, and both commit.
func TestRealtimeOutbox_FirstEventRace(t *testing.T) {
	db, events := rtOutboxDB(t)
	for round := 0; round < 15; round++ {
		tenant := uuid.New()
		var wg sync.WaitGroup
		seqs := make([]int64, 2)
		errs := make([]error, 2)
		start := make(chan struct{})
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e := newEvent(t, tenant, realtime.AssetCreated, uuid.NewString())
				<-start
				errs[i] = db.Transaction(func(tx *gorm.DB) error { return events.Bind(tx).Enlist(context.Background(), e) })
				seqs[i] = e.Sequence
			}(i)
		}
		close(start)
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		assert.ElementsMatch(t, []int64{1, 2}, seqs, "round %d: the racing first events take 1 and 2", round)
	}
}

// Enlisting and the inline path share one counter.
func TestRealtimeOutbox_InlineAndEnlistedShareTheCounter(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	var wg sync.WaitGroup
	const n = 12
	seqs := make(chan int64, 2*n)
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			e := newEvent(t, tenant, realtime.RiskUpdated, "r")
			assert.NoError(t, events.Append(context.Background(), e))
			seqs <- e.Sequence
		}()
		go func() {
			defer wg.Done()
			e := newEvent(t, tenant, realtime.AssetCreated, uuid.NewString())
			assert.NoError(t, db.Transaction(func(tx *gorm.DB) error { return events.Bind(tx).Enlist(context.Background(), e) }))
			seqs <- e.Sequence
		}()
	}
	wg.Wait()
	close(seqs)
	seen := map[int64]bool{}
	for s := range seqs {
		assert.False(t, seen[s], "sequence %d handed out twice", s)
		seen[s] = true
	}
	assert.Len(t, seen, 2*n)
	for i := int64(1); i <= 2*n; i++ {
		assert.True(t, seen[i], "no gap at %d", i)
	}
}

// The store's per-row statements are tenant-gated: tenant B cannot close,
// retry, release or abandon tenant A's row, even holding its id.
func TestRealtimeOutbox_StoreStatementsAreTenantGated(t *testing.T) {
	db, events := rtOutboxDB(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ev := enlistIn(t, db, events, a, 1)[0]
	claimed, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	n, err := events.MarkPublished(ctx, b, []uuid.UUID{ev.ID})
	require.NoError(t, err)
	assert.Zero(t, n)
	lease := *claimed[0].LockedUntil
	require.NoError(t, events.Retry(ctx, b, ev.ID, lease, time.Now().Add(time.Hour), "x"))
	require.NoError(t, events.Release(ctx, b, []uuid.UUID{ev.ID}, lease))
	require.NoError(t, events.Abandon(ctx, b, ev.ID, "x"))

	row := rowOf(t, db, a, ev.ID)
	assert.Nil(t, row.PublishedAt)
	assert.Nil(t, row.AbandonedAt)
	assert.Equal(t, 1, row.Attempts, "release by another tenant did not give the attempt back")
	require.NotNil(t, row.LockedUntil)
	assert.True(t, row.LockedUntil.After(time.Now().Add(30*time.Second)), "and the lease is untouched")

	// Fail closed without a tenant.
	_, err = events.MarkPublished(ctx, uuid.Nil, []uuid.UUID{ev.ID})
	require.Error(t, err)
	require.Error(t, events.Retry(ctx, uuid.Nil, ev.ID, lease, time.Now(), "x"))
	require.Error(t, events.Release(ctx, uuid.Nil, []uuid.UUID{ev.ID}, lease))
	require.Error(t, events.Abandon(ctx, uuid.Nil, ev.ID, "x"))

	// The owner can.
	n, err = events.MarkPublished(ctx, a, []uuid.UUID{ev.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
}

func TestRealtimeOutbox_ReleaseGivesTheAttemptBack(t *testing.T) {
	db, events := rtOutboxDB(t)
	ctx := context.Background()
	tenant := uuid.New()
	ev := enlistIn(t, db, events, tenant, 1)[0]
	claimed, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	require.NoError(t, events.Release(ctx, tenant, []uuid.UUID{ev.ID}, *claimed[0].LockedUntil))
	row := rowOf(t, db, tenant, ev.ID)
	assert.Equal(t, 0, row.Attempts)
	assert.Nil(t, row.LockedUntil)
	// And it is claimable again at once.
	again, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	assert.Len(t, again, 1)
}

// Relay A's lease expires and relay B claims the rows; A then fails to finish
// and calls Retry/Release with its old lease. B's lease and attempt count stay.
func TestRealtimeOutbox_StaleLeaseCannotTouchANewerClaim(t *testing.T) {
	db, events := rtOutboxDB(t)
	ctx := context.Background()
	tenant := uuid.New()
	evs := enlistIn(t, db, events, tenant, 2)

	first, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 2)
	staleLease := *first[0].LockedUntil

	// A's lease expires; B claims both rows.
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() - interval '1 second' WHERE tenant_id = ?`, tenant).Error)
	second, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 2)
	bLease := *second[0].LockedUntil
	require.True(t, bLease.After(staleLease))

	require.NoError(t, events.Retry(ctx, tenant, evs[0].ID, staleLease, time.Now().Add(time.Hour), "late"))
	require.NoError(t, events.Release(ctx, tenant, []uuid.UUID{evs[0].ID, evs[1].ID}, staleLease))

	for _, ev := range evs {
		row := rowOf(t, db, tenant, ev.ID)
		require.NotNil(t, row.LockedUntil)
		assert.WithinDuration(t, bLease, *row.LockedUntil, time.Millisecond, "B's lease is untouched")
		assert.Equal(t, 2, row.Attempts, "B's attempt is not given back")
		assert.Empty(t, row.LastError)
	}
	// B's own guarded calls still work.
	require.NoError(t, events.Release(ctx, tenant, []uuid.UUID{evs[1].ID}, bLease))
	assert.Nil(t, rowOf(t, db, tenant, evs[1].ID).LockedUntil)
}

// A tenant whose head is backed off must not fill the tenant LIMIT and starve
// the tenants behind it.
func TestRealtimeOutbox_BackedOffTenantDoesNotStarveOthers(t *testing.T) {
	db, events := rtOutboxDB(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ea := enlistIn(t, db, events, a, 1) // older: would be picked first
	eb := enlistIn(t, db, events, b, 1)
	require.NoError(t, db.Exec(`UPDATE realtime_events SET locked_until = now() + interval '1 hour' WHERE tenant_id = ? AND id = ?`, a, ea[0].ID).Error)

	claimed, err := events.ClaimPending(ctx, 1, 100, time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, eb[0].ID, claimed[0].ID)
	assert.Equal(t, b, claimed[0].TenantID)
}

// --- AC6: scrape-time gauges ------------------------------------------------

func TestRealtimeOutbox_MetricsTrackPendingAndLag(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	evs := enlistIn(t, db, events, tenant, 3)
	require.NoError(t, db.Exec(`UPDATE realtime_events SET created_at = now() - interval '10 seconds' WHERE tenant_id = ? AND id = ?`, tenant, evs[0].ID).Error)

	reg := prometheus.NewRegistry()
	reg.MustRegister(monitoring.NewRealtimeOutboxCollector(events.PendingStats))
	read := func() (pending, lag float64) {
		mfs, err := reg.Gather()
		require.NoError(t, err)
		require.Len(t, mfs, 2)
		for _, mf := range mfs {
			switch mf.GetName() {
			case "openrisk_realtime_outbox_pending":
				pending = mf.GetMetric()[0].GetGauge().GetValue()
			case "openrisk_realtime_outbox_oldest_pending_seconds":
				lag = mf.GetMetric()[0].GetGauge().GetValue()
			}
		}
		return
	}
	pending, lag := read()
	assert.Equal(t, 3.0, pending)
	assert.GreaterOrEqual(t, lag, 10.0)
	assert.LessOrEqual(t, lag, 12.0)

	_, _, err := apprealtime.NewOutboxRelay(events, &capSink{}).Pass(context.Background())
	require.NoError(t, err)
	pending, lag = read()
	assert.Zero(t, pending, "drained")
	assert.Zero(t, lag)
}

// --- AC7: the migration on a table that already holds rows ------------------

func readSQL(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func tableDigest(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var d string
	require.NoError(t, db.Raw(`SELECT COALESCE(md5(string_agg(t::text, '|' ORDER BY t.id)), '') FROM realtime_events t`).Scan(&d).Error)
	return d
}

func replayJSON(t *testing.T, events *GormRealtimeEventRepository, tenants ...uuid.UUID) string {
	t.Helper()
	var out []json.RawMessage
	for _, tn := range tenants {
		rows, err := events.Replay(context.Background(), tn, 0, 500)
		require.NoError(t, err)
		for _, r := range rows {
			b, err := json.Marshal(r.ToEnvelope())
			require.NoError(t, err)
			out = append(out, b)
		}
	}
	b, err := json.Marshal(out)
	require.NoError(t, err)
	return string(b)
}

func TestMigration0085_LegacyRowsReplayUnchanged(t *testing.T) {
	db := rtSchema(t)
	ctx := context.Background()
	require.NoError(t, db.Exec(readSQL(t, rtLogMigration+".up.sql")).Error)

	a, b := uuid.New(), uuid.New()
	actor := uuid.New()
	for i, tn := range []uuid.UUID{a, a, a, b, b} {
		require.NoError(t, db.Exec(`INSERT INTO realtime_events
			(id, tenant_id, sequence, type, version, envelope_version, aggregate_type, aggregate_id, actor_id, correlation_id, payload, occurred_at, created_at)
			VALUES (?, ?, ?, 'asset.created', 1, 1, 'asset', ?, ?, 'req', '{"k":"v"}', now() - interval '1 hour', now() - interval '1 hour')`,
			uuid.New(), tn, int64(i%3+1), uuid.NewString(), actor).Error)
	}
	before := replayJSON(t, NewGormRealtimeEventRepository(reopened(t, db)), a, b)
	require.NotEqual(t, "[]", before)

	up := readSQL(t, rtOutboxMigration+".up.sql")
	require.NoError(t, db.Exec(up).Error)

	db = reopened(t, db)
	events := NewGormRealtimeEventRepository(db)
	assert.Equal(t, before, replayJSON(t, events, a, b), "a replay reads legacy rows byte for byte as before")
	var unpublished int64
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).Where("published_at IS NULL").Count(&unpublished).Error)
	assert.Zero(t, unpublished, "every row that existed reads as published: the relay has nothing to republish")
	n, oldest, err := events.PendingStats(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Nil(t, oldest)
	claimed, err := events.ClaimPending(ctx, 16, 100, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, claimed)

	// Replayed: the file changes no row.
	digest := tableDigest(t, db)
	require.NoError(t, db.Exec(up).Error)
	assert.Equal(t, digest, tableDigest(t, db), "applying 0085 twice changes no row")

	// New rows are born pending; down, then up again.
	fresh := newEvent(t, a, realtime.AssetCreated, "x")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return events.Bind(tx).Enlist(ctx, fresh) }))
	assert.EqualValues(t, 1, pendingCount(t, db, a), "a row written after the migration is pending")

	require.NoError(t, db.Exec(readSQL(t, rtOutboxMigration+".down.sql")).Error)
	db = reopened(t, db)
	events = NewGormRealtimeEventRepository(db)
	assert.Contains(t, replayJSON(t, events, a, b), `"aggregate"`, "down leaves the events readable")
	require.NoError(t, db.Exec(up).Error)
	db = reopened(t, db)
	require.NoError(t, db.Model(&domain.RealtimeEvent{}).Where("published_at IS NULL").Count(&unpublished).Error)
	assert.Zero(t, unpublished, "up again after down: every row reads as published, the relay replays nothing")
}

// Reviewer check made executable: the up file adds and never removes.
func TestMigration0085_IsAdditiveOnly(t *testing.T) {
	up := readSQL(t, rtOutboxMigration+".up.sql")
	var code []string
	for _, line := range strings.Split(up, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		code = append(code, line)
	}
	body := strings.ToUpper(strings.Join(code, "\n"))
	for _, forbidden := range []string{`\bDROP\s+(TABLE|COLUMN|INDEX|SCHEMA|CONSTRAINT)\b`, `\bRENAME\b`, `\bDELETE\b`, `\bUPDATE\b`, `\bTRUNCATE\b`} {
		assert.False(t, regexp.MustCompile(forbidden).MatchString(body), "0085 up must not contain %s", forbidden)
	}
}

// Purge never takes a pending row on Postgres either.
func TestRealtimeOutbox_PurgeKeepsPendingRows(t *testing.T) {
	db, events := rtOutboxDB(t)
	tenant := uuid.New()
	evs := enlistIn(t, db, events, tenant, 2)
	require.NoError(t, events.Append(context.Background(), newEvent(t, tenant, realtime.RiskUpdated, "r")))
	require.NoError(t, db.Exec(`UPDATE realtime_events SET created_at = now() - interval '3 days' WHERE tenant_id = ?`, tenant).Error)
	n, err := events.PurgeBefore(context.Background(), time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.EqualValues(t, 2, pendingCount(t, db, tenant))
	_ = evs
}
