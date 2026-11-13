// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/monitoring"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// fakeStore serves the rows it was given as one claim and records every call.
type fakeStore struct {
	mu        sync.Mutex
	claim     []domain.RealtimeEvent
	expired   int64
	markN     *int64 // overrides what MarkPublished reports
	marked    map[uuid.UUID][]uuid.UUID
	retries   []retryCall
	released  []uuid.UUID
	abandoned []abandonCall
}

type retryCall struct {
	tenant, id uuid.UUID
	notBefore  time.Time
	reason     string
}

type abandonCall struct {
	tenant, id uuid.UUID
	reason     string
}

func (s *fakeStore) AbandonExpired(context.Context, int) (int64, error) { return s.expired, nil }

func (s *fakeStore) ClaimPending(context.Context, int, int, time.Duration) ([]domain.RealtimeEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := s.claim
	s.claim = nil
	return rows, nil
}

func (s *fakeStore) MarkPublished(_ context.Context, tenant uuid.UUID, ids []uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.marked == nil {
		s.marked = map[uuid.UUID][]uuid.UUID{}
	}
	s.marked[tenant] = append(s.marked[tenant], ids...)
	if s.markN != nil {
		return *s.markN, nil
	}
	return int64(len(ids)), nil
}

func (s *fakeStore) Retry(_ context.Context, tenant, id uuid.UUID, notBefore time.Time, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retries = append(s.retries, retryCall{tenant, id, notBefore, reason})
	return nil
}

func (s *fakeStore) Release(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.released = append(s.released, ids...)
	return nil
}

func (s *fakeStore) Abandon(_ context.Context, tenant, id uuid.UUID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abandoned = append(s.abandoned, abandonCall{tenant, id, reason})
	return nil
}

// recordingSink records what it delivered; failWith makes every call fail.
type recordingSink struct {
	mu       sync.Mutex
	got      []rt.Envelope
	failWith error
}

func (s *recordingSink) Deliver(_ context.Context, env rt.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return s.failWith
	}
	s.got = append(s.got, env)
	return nil
}

func outboxRow(tenant uuid.UUID, seq int64, attempts int) domain.RealtimeEvent {
	return domain.RealtimeEvent{
		ID: uuid.New(), TenantID: tenant, Sequence: seq, Attempts: attempts,
		Type: string(rt.AssetCreated), Version: 1, EnvelopeVersion: rt.EnvelopeVersion,
		AggregateType: "asset", AggregateID: uuid.NewString(), OccurredAt: time.Now().UTC(),
	}
}

func fixedClock(t time.Time) OutboxOption { return WithOutboxClock(func() time.Time { return t }) }

func TestOutboxRelay_Success(t *testing.T) {
	tenant := uuid.New()
	// Handed back out of order on purpose: RETURNING order is not trusted.
	rows := []domain.RealtimeEvent{outboxRow(tenant, 3, 1), outboxRow(tenant, 1, 1), outboxRow(tenant, 2, 1)}
	store := &fakeStore{claim: rows}
	sink := &recordingSink{}
	before := testutil.ToFloat64(monitoring.RealtimeOutboxPublishedTotal)

	claimed, full, err := NewOutboxRelay(store, sink).Pass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, claimed)
	assert.False(t, full)

	require.Len(t, sink.got, 3)
	for i, env := range sink.got {
		assert.Equal(t, int64(i+1), env.Sequence, "delivered in ascending sequence")
		assert.Equal(t, tenant.String(), env.TenantID)
	}
	assert.Len(t, store.marked[tenant], 3, "closed in one statement per tenant")
	assert.Equal(t, 3.0, testutil.ToFloat64(monitoring.RealtimeOutboxPublishedTotal)-before)
	assert.Empty(t, store.retries)
}

// NotFound: another relay already published the row (the mark closes fewer rows
// than asked). Benign: no error, nothing is delivered a second time.
func TestOutboxRelay_NotFound(t *testing.T) {
	tenant := uuid.New()
	zero := int64(0)
	store := &fakeStore{claim: []domain.RealtimeEvent{outboxRow(tenant, 1, 1)}, markN: &zero}
	sink := &recordingSink{}
	relay := NewOutboxRelay(store, sink)

	_, _, err := relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Len(t, sink.got, 1)
	assert.Empty(t, store.retries)
	assert.Empty(t, store.abandoned)

	// Nothing left to claim: nothing is re-delivered in the next pass.
	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Len(t, sink.got, 1)
}

// Unauthorized: a row with no tenant is never handed to the sink.
func TestOutboxRelay_Unauthorized(t *testing.T) {
	bad := outboxRow(uuid.Nil, 1, 1)
	store := &fakeStore{claim: []domain.RealtimeEvent{bad}}
	sink := &recordingSink{}
	before := testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)

	_, _, err := NewOutboxRelay(store, sink).Pass(context.Background())
	require.NoError(t, err)
	assert.Empty(t, sink.got, "a row with no tenant must never reach a sink")
	require.Len(t, store.abandoned, 1)
	assert.Equal(t, "no tenant", store.abandoned[0].reason)
	assert.Equal(t, bad.ID, store.abandoned[0].id)
	assert.Equal(t, 1.0, testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)-before)
}

func TestOutboxRelay_SinkErrorRetriesWithBackoff(t *testing.T) {
	tenant := uuid.New()
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	head, second := outboxRow(tenant, 1, 1), outboxRow(tenant, 2, 1)
	store := &fakeStore{claim: []domain.RealtimeEvent{head, second}}
	sink := &recordingSink{failWith: errors.New("redis down")}
	relay := NewOutboxRelay(store, sink, fixedClock(now))
	before := testutil.ToFloat64(monitoring.RealtimeOutboxPublishFailuresTotal)

	_, _, err := relay.Pass(context.Background())
	require.NoError(t, err)

	require.Len(t, store.retries, 1)
	assert.Equal(t, head.ID, store.retries[0].id)
	assert.Equal(t, now.Add(time.Second), store.retries[0].notBefore, "first failure backs off one second")
	assert.Equal(t, "redis down", store.retries[0].reason)
	assert.Equal(t, []uuid.UUID{second.ID}, store.released, "the later row is released, not delivered")
	assert.Empty(t, store.marked)
	assert.Equal(t, 1.0, testutil.ToFloat64(monitoring.RealtimeOutboxPublishFailuresTotal)-before)

	// Second failure of the same row: attempts is now 2, so the delay doubles.
	again := head
	again.Attempts = 2
	store.claim = []domain.RealtimeEvent{again}
	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	require.Len(t, store.retries, 2)
	assert.Equal(t, now.Add(2*time.Second), store.retries[1].notBefore)
	assert.Equal(t, 2.0, testutil.ToFloat64(monitoring.RealtimeOutboxPublishFailuresTotal)-before)

	// The sink recovers: the row is delivered and closed.
	sink.mu.Lock()
	sink.failWith = nil
	sink.mu.Unlock()
	third := head
	third.Attempts = 3
	store.claim = []domain.RealtimeEvent{third}
	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{head.ID}, store.marked[tenant])
	require.Len(t, sink.got, 1)
	assert.Equal(t, head.ID.String(), sink.got[0].ID, "the redelivery carries the row's own id")
}

// A failing fanout behind the real HubSink fails the delivery, and the local
// clients still got the event once.
func TestOutboxRelay_FanoutErrorThroughHubSink(t *testing.T) {
	tenant := uuid.New()
	hub := NewHub(HubOptions{})
	sub, err := hub.Subscribe(tenant, rt.Filter{})
	require.NoError(t, err)
	defer hub.Unsubscribe(sub)

	row := outboxRow(tenant, 1, 1)
	store := &fakeStore{claim: []domain.RealtimeEvent{row}}
	fan := &failingFanout{err: errors.New("redis down")}
	relay := NewOutboxRelay(store, NewHubSink(hub, fan), fixedClock(time.Now()))

	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	require.Len(t, store.retries, 1, "a fanout error keeps the row pending")
	assert.Empty(t, store.marked)
	assert.Equal(t, TenantChannel(tenant), fan.channel)

	// Retry after the fanout recovers: the hub's dedup keeps the local client
	// at one copy while the fanout now succeeds.
	fan.err = nil
	retry := row
	retry.Attempts = 2
	store.claim = []domain.RealtimeEvent{retry}
	_, _, err = relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{row.ID}, store.marked[tenant])
	require.Len(t, sub.Events(), 1, "the local client sees the event once across both attempts")
	assert.Equal(t, row.ID.String(), (<-sub.Events()).ID)
}

type failingFanout struct {
	err     error
	channel string
}

func (f *failingFanout) Publish(_ context.Context, channel string, _ interface{}) error {
	f.channel = channel
	return f.err
}

func TestOutboxRelay_AbandonsAtAttemptCap(t *testing.T) {
	tenant := uuid.New()
	row := outboxRow(tenant, 1, 20) // the claim counted the 20th attempt
	store := &fakeStore{claim: []domain.RealtimeEvent{row}}
	sink := &recordingSink{failWith: errors.New("redis down")}
	before := testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)

	_, _, err := NewOutboxRelay(store, sink).Pass(context.Background())
	require.NoError(t, err)
	assert.Empty(t, store.retries)
	require.Len(t, store.abandoned, 1)
	assert.Equal(t, row.ID, store.abandoned[0].id)
	assert.Equal(t, 1.0, testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)-before)
}

func TestOutboxRelay_CountsRowsAbandonedByTheSweep(t *testing.T) {
	store := &fakeStore{expired: 3}
	before := testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)
	_, _, err := NewOutboxRelay(store, &recordingSink{}).Pass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3.0, testutil.ToFloat64(monitoring.RealtimeOutboxAbandonedTotal)-before)
}

func TestOutboxRelay_ReasonNeverExceedsTheCap(t *testing.T) {
	tenant := uuid.New()
	store := &fakeStore{claim: []domain.RealtimeEvent{outboxRow(tenant, 1, 1)}}
	sink := &recordingSink{failWith: errors.New(string(make([]byte, 5000)))}
	_, _, err := NewOutboxRelay(store, sink).Pass(context.Background())
	require.NoError(t, err)
	require.Len(t, store.retries, 1)
	assert.LessOrEqual(t, len(store.retries[0].reason), maxOutboxReasonBytes)
}

// A cancelled context releases the undelivered rows instead of delivering them.
func TestOutboxRelay_CancelledContextReleasesTheRest(t *testing.T) {
	tenant := uuid.New()
	a, b := outboxRow(tenant, 1, 1), outboxRow(tenant, 2, 1)
	store := &fakeStore{claim: []domain.RealtimeEvent{a, b}}
	sink := &recordingSink{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The claim itself is served by the fake despite the cancelled context.
	_, _, err := NewOutboxRelay(store, sink).Pass(ctx)
	require.NoError(t, err)
	assert.Empty(t, sink.got)
	assert.ElementsMatch(t, []uuid.UUID{a.ID, b.ID}, store.released)
}

// Two tenants in one claim: each tenant's rows are delivered in sequence order
// and closed under its own tenant id; a tenant's failure does not stop another.
func TestOutboxRelay_TenantsAreDeliveredAndClosedSeparately(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	store := &fakeStore{claim: []domain.RealtimeEvent{
		outboxRow(a, 2, 1), outboxRow(b, 1, 1), outboxRow(a, 1, 1), outboxRow(b, 2, 1),
	}}
	sink := &recordingSink{}
	_, _, err := NewOutboxRelay(store, sink).Pass(context.Background())
	require.NoError(t, err)
	assert.Len(t, store.marked[a], 2)
	assert.Len(t, store.marked[b], 2)
	last := map[string]int64{}
	for _, env := range sink.got {
		assert.Greater(t, env.Sequence, last[env.TenantID], "per-tenant ascending order")
		last[env.TenantID] = env.Sequence
	}
}

// A 1000-row burst against a subscriber that never reads: the hub drops, and a
// resync is signalled, so the client refetches once. Never silent.
func TestOutboxRelay_BurstOverBufferRaisesResync(t *testing.T) {
	tenant := uuid.New()
	hub := NewHub(HubOptions{})
	sub, err := hub.Subscribe(tenant, rt.Filter{})
	require.NoError(t, err)
	defer hub.Unsubscribe(sub)

	rows := make([]domain.RealtimeEvent, 1000)
	for i := range rows {
		rows[i] = outboxRow(tenant, int64(i+1), 1)
	}
	store := &fakeStore{claim: rows}
	relay := NewOutboxRelay(store, NewHubSink(hub, nil), WithOutboxBatch(16, 1000))
	claimed, _, err := relay.Pass(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1000, claimed)
	assert.Len(t, store.marked[tenant], 1000, "every row is still delivered and closed")

	assert.Len(t, sub.Events(), DefaultBufferSize, "the buffer holds what fits")
	assert.Positive(t, sub.Dropped())
	select {
	case <-sub.Resync():
	default:
		t.Fatal("a subscriber that lost events must be told to resync")
	}
}

func TestOutboxRelay_BackoffIsCapped(t *testing.T) {
	r := NewOutboxRelay(&fakeStore{}, &recordingSink{})
	assert.Equal(t, time.Second, r.backoff(1))
	assert.Equal(t, 2*time.Second, r.backoff(2))
	assert.Equal(t, 4*time.Second, r.backoff(3))
	assert.Equal(t, 5*time.Minute, r.backoff(19))
	assert.Equal(t, 5*time.Minute, r.backoff(200))
}

func TestHubSink_RefusesAnEnvelopeWithoutATenant(t *testing.T) {
	hub := NewHub(HubOptions{})
	err := NewHubSink(hub, nil).Deliver(context.Background(), rt.Envelope{ID: uuid.NewString()})
	require.Error(t, err)
}
