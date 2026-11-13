// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package realtime

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/monitoring"
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// The realtime outbox relay (#344, ADR 0010).
//
// A use case that enlists its events writes them to realtime_events inside its
// own transaction, unpublished. This relay publishes them after commit:
//
//   - AT LEAST ONCE. A row is marked published only after the sink accepted it.
//     A crash between the two, or a lease that expires while a delivery is in
//     flight, delivers it again. Every redelivery carries the SAME id (the row
//     id, minted once when the event was prepared), which is the key consumers
//     deduplicate on. Exactly-once is never claimed.
//   - ORDERED PER TENANT by sequence. A tenant's rows are delivered one after
//     the other, and a row that fails, is backed off or is leased by another
//     relay blocks every later row of that tenant (the claim SQL enforces it).
//     There is no order across tenants.
//   - ISOLATED. Rows are claimed and closed per tenant, and a row whose tenant
//     is missing, or differs from the envelope's, is refused and abandoned.
// ---------------------------------------------------------------------------

// OutboxStore is what the relay needs from the database. Declared here, where it
// is consumed; satisfied by the GORM repository.
type OutboxStore interface {
	// AbandonExpired gives up on rows claimed maxAttempts times that never
	// reported an outcome, and returns how many.
	AbandonExpired(ctx context.Context, maxAttempts int) (int64, error)
	// ClaimPending leases up to perTenant pending rows of up to maxTenants
	// tenants, in sequence order within a tenant.
	ClaimPending(ctx context.Context, maxTenants, perTenant int, lease time.Duration) ([]domain.RealtimeEvent, error)
	// MarkPublished closes delivered rows of one tenant and returns how many it
	// closed.
	MarkPublished(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (int64, error)
	// Retry keeps a failed row pending, held back until notBefore.
	Retry(ctx context.Context, tenantID, id uuid.UUID, notBefore time.Time, reason string) error
	// Release puts claimed, untried rows back without counting an attempt.
	Release(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error
	// Abandon gives up on one row for good.
	Abandon(ctx context.Context, tenantID, id uuid.UUID, reason string) error
}

// Sink delivers one envelope to this instance's clients and to the other
// instances. An error means the delivery did not happen and must be retried.
type Sink interface {
	Deliver(ctx context.Context, env rt.Envelope) error
}

// HubSink is the production Sink: the local hub, then the Redis fanout.
type HubSink struct {
	hub    *Hub
	fanout Fanout
}

// NewHubSink builds the sink. fanout may be nil in a deployment without Redis:
// the clients of this instance still get the event, and a replay serves the rest.
func NewHubSink(hub *Hub, fanout Fanout) *HubSink {
	return &HubSink{hub: hub, fanout: fanout}
}

// Deliver dispatches to the local hub first, then publishes to the tenant's
// fanout channel and returns that error.
//
// The hub cannot fail. On a retry its recent-id window suppresses the second
// local dispatch, so local clients see the event once; the other instances get
// it when the fanout finally succeeds. Hub.Dispatch has no failure mode and none
// is invented here: the only failure the relay knows about is the one this
// method returns, which in production is the Redis publish.
func (s *HubSink) Deliver(ctx context.Context, env rt.Envelope) error {
	tenant, err := uuid.Parse(env.TenantID)
	if err != nil || tenant == uuid.Nil {
		return fmt.Errorf("realtime outbox: envelope has no valid tenant")
	}
	s.hub.Dispatch(env)
	if s.fanout == nil {
		return nil
	}
	return s.fanout.Publish(ctx, TenantChannel(tenant), env)
}

// Relay tunables. The defaults are what production runs; tests shrink them.
const (
	defaultOutboxMaxTenants   = 16
	defaultOutboxPerTenant    = 100
	defaultOutboxLease        = 30 * time.Second
	defaultOutboxMaxAttempts  = 20
	defaultOutboxBackoffBase  = time.Second
	defaultOutboxBackoffMax   = 5 * time.Minute
	defaultOutboxConcurrency  = 8
	maxOutboxReasonBytes      = 512
	outboxBookkeepingDeadline = 5 * time.Second
)

// OutboxRelay publishes enlisted events after their transaction committed.
type OutboxRelay struct {
	store OutboxStore
	sink  Sink

	maxTenants  int
	perTenant   int
	lease       time.Duration
	maxAttempts int
	backoffBase time.Duration
	backoffMax  time.Duration
	concurrency int
	now         func() time.Time
}

// OutboxOption overrides a relay default, for tests.
type OutboxOption func(*OutboxRelay)

// WithOutboxBatch sets how many tenants one pass takes and how many rows of each.
func WithOutboxBatch(maxTenants, perTenant int) OutboxOption {
	return func(r *OutboxRelay) {
		if maxTenants > 0 {
			r.maxTenants = maxTenants
		}
		if perTenant > 0 {
			r.perTenant = perTenant
		}
	}
}

// WithOutboxAttempts sets the attempt cap and the backoff bounds.
func WithOutboxAttempts(maxAttempts int, base, ceiling time.Duration) OutboxOption {
	return func(r *OutboxRelay) {
		if maxAttempts > 0 {
			r.maxAttempts = maxAttempts
		}
		if base > 0 {
			r.backoffBase = base
		}
		if ceiling > 0 {
			r.backoffMax = ceiling
		}
	}
}

// WithOutboxLease sets the claim lease.
func WithOutboxLease(d time.Duration) OutboxOption {
	return func(r *OutboxRelay) {
		if d > 0 {
			r.lease = d
		}
	}
}

// WithOutboxClock overrides the clock.
func WithOutboxClock(now func() time.Time) OutboxOption {
	return func(r *OutboxRelay) {
		if now != nil {
			r.now = now
		}
	}
}

// NewOutboxRelay builds the relay.
func NewOutboxRelay(store OutboxStore, sink Sink, opts ...OutboxOption) *OutboxRelay {
	r := &OutboxRelay{
		store: store, sink: sink,
		maxTenants: defaultOutboxMaxTenants, perTenant: defaultOutboxPerTenant,
		lease: defaultOutboxLease, maxAttempts: defaultOutboxMaxAttempts,
		backoffBase: defaultOutboxBackoffBase, backoffMax: defaultOutboxBackoffMax,
		concurrency: defaultOutboxConcurrency,
		now:         func() time.Time { return time.Now().UTC() },
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// backoff is the delay after the attempt-th failed attempt: base, 2x base, 4x
// base, ... capped.
func (r *OutboxRelay) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := r.backoffBase
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= r.backoffMax || d <= 0 {
			return r.backoffMax
		}
	}
	if d > r.backoffMax {
		return r.backoffMax
	}
	return d
}

// Pass runs one relay pass: sweep poison rows, claim, deliver, close.
//
// ctx governs delivery and the claim. The bookkeeping after a delivery (mark,
// retry, release, abandon) runs on a context that outlives ctx's cancellation
// for a few seconds, so a relay being stopped still records what it did.
//
// It returns how many rows it claimed and whether the batch was full, in which
// case the caller should run another pass at once.
func (r *OutboxRelay) Pass(ctx context.Context) (claimed int, full bool, err error) {
	abandoned, err := r.store.AbandonExpired(ctx, r.maxAttempts)
	if err != nil {
		return 0, false, fmt.Errorf("realtime outbox: poison sweep failed: %w", err)
	}
	if abandoned > 0 {
		monitoring.RealtimeOutboxAbandonedTotal.Add(float64(abandoned))
		log.Printf("realtime outbox: abandoned %d row(s) that never reported an outcome", abandoned)
	}

	rows, err := r.store.ClaimPending(ctx, r.maxTenants, r.perTenant, r.lease)
	if err != nil {
		return 0, false, fmt.Errorf("realtime outbox: claim failed: %w", err)
	}
	if len(rows) == 0 {
		return 0, false, nil
	}

	// RETURNING order is not trusted: group by tenant, then sort by sequence.
	groups := map[uuid.UUID][]domain.RealtimeEvent{}
	for _, row := range rows {
		groups[row.TenantID] = append(groups[row.TenantID], row)
	}
	full = len(rows) >= r.maxTenants*r.perTenant
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup
	for tenant, group := range groups {
		sort.SliceStable(group, func(i, j int) bool { return group[i].Sequence < group[j].Sequence })
		if len(group) >= r.perTenant {
			full = true
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(tenant uuid.UUID, group []domain.RealtimeEvent) {
			defer wg.Done()
			defer func() { <-sem }()
			r.deliverTenant(ctx, tenant, group)
		}(tenant, group)
	}
	wg.Wait()
	return len(rows), full, nil
}

// deliverTenant delivers one tenant's claimed rows strictly in order and stops
// at the first failure.
func (r *OutboxRelay) deliverTenant(ctx context.Context, tenant uuid.UUID, rows []domain.RealtimeEvent) {
	book, cancel := context.WithTimeout(context.WithoutCancel(ctx), outboxBookkeepingDeadline)
	defer cancel()

	var delivered []domain.RealtimeEvent
	var untried []uuid.UUID
	var failedRow *domain.RealtimeEvent
	var failure error

	for i := range rows {
		row := rows[i]
		if ctx.Err() != nil {
			// Stopping: put the rest back untouched, due now.
			for _, rest := range rows[i:] {
				untried = append(untried, rest.ID)
			}
			break
		}
		env := row.ToEnvelope()
		// Fail closed. A row with no tenant, or whose envelope names another
		// one, is never handed to the sink: it would be delivered to some
		// tenant's stream. It is abandoned and logged loudly.
		if row.TenantID == uuid.Nil || env.TenantID != row.TenantID.String() {
			log.Printf("realtime outbox: SECURITY refused row %s: tenant missing or mismatched, abandoning", row.ID)
			if err := r.store.Abandon(book, row.TenantID, row.ID, "no tenant"); err != nil {
				log.Printf("realtime outbox: could not abandon row %s: %v", row.ID, err)
			}
			monitoring.RealtimeOutboxAbandonedTotal.Inc()
			continue
		}
		if err := r.sink.Deliver(ctx, env); err != nil {
			monitoring.RealtimeOutboxPublishFailuresTotal.Inc()
			failedRow, failure = &row, err
			for _, rest := range rows[i+1:] {
				untried = append(untried, rest.ID)
			}
			break
		}
		delivered = append(delivered, row)
	}

	if len(delivered) > 0 {
		r.markDelivered(book, tenant, delivered)
	}
	if failedRow != nil {
		r.recordFailure(book, tenant, *failedRow, failure)
	}
	if len(untried) > 0 && tenant != uuid.Nil {
		if err := r.store.Release(book, tenant, untried); err != nil {
			log.Printf("realtime outbox: could not release %d row(s): %v", len(untried), err)
		}
	}
}

func (r *OutboxRelay) markDelivered(ctx context.Context, tenant uuid.UUID, delivered []domain.RealtimeEvent) {
	ids := make([]uuid.UUID, len(delivered))
	for i, row := range delivered {
		ids[i] = row.ID
	}
	n, err := r.store.MarkPublished(ctx, tenant, ids)
	if err != nil {
		// The rows keep their lease and will be delivered again when it expires:
		// at-least-once, same ids.
		log.Printf("realtime outbox: delivered %d row(s) but could not mark them: %v", len(ids), err)
		return
	}
	monitoring.RealtimeOutboxPublishedTotal.Add(float64(n))
	if n < int64(len(ids)) {
		// A lease expired and another relay delivered and closed some of these.
		// Benign: it is the at-least-once window, and the id is the same.
		log.Printf("realtime outbox: %d of %d row(s) were already closed by another relay", int64(len(ids))-n, len(ids))
		return
	}
	for _, row := range delivered {
		origin := "unknown"
		if desc, ok := rt.Lookup(rt.EventType(row.Type)); ok {
			origin = string(desc.Origin)
		}
		monitoring.RealtimeEventsPublishedTotal.WithLabelValues(row.Type, origin).Inc()
	}
}

func (r *OutboxRelay) recordFailure(ctx context.Context, tenant uuid.UUID, row domain.RealtimeEvent, cause error) {
	// The reason is the error text only, truncated: it never carries a payload.
	reason := cause.Error()
	if len(reason) > maxOutboxReasonBytes {
		reason = reason[:maxOutboxReasonBytes]
	}
	if row.Attempts >= r.maxAttempts {
		if err := r.store.Abandon(ctx, tenant, row.ID, reason); err != nil {
			log.Printf("realtime outbox: could not abandon row %s: %v", row.ID, err)
			return
		}
		monitoring.RealtimeOutboxAbandonedTotal.Inc()
		log.Printf("realtime outbox: abandoned row %s after %d attempts", row.ID, row.Attempts)
		return
	}
	notBefore := r.now().Add(r.backoff(row.Attempts))
	if err := r.store.Retry(ctx, tenant, row.ID, notBefore, reason); err != nil {
		log.Printf("realtime outbox: could not schedule retry of row %s: %v", row.ID, err)
	}
}
