// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRealtimeEventRepository is the durable, per-tenant ordered event log, and
// the outbox store for use cases that enlist their events (ADR 0010).
//
// Sequences come from one counter row per tenant (realtime_event_sequences),
// allocated by a single upsert inside the writing transaction:
//
//   - on Postgres the upsert holds the counter row lock until that transaction
//     ends, so two transactions of one tenant serialise there, including the
//     very first event of a tenant, where there is no head row to lock;
//   - the counter is seeded from MAX(sequence) of the tenant's rows and never
//     falls below it, so a legacy tenant, or a rolling deploy where an older
//     binary still appends by MAX+1, heals itself;
//   - the unique index on (tenant_id, sequence) is the final backstop.
//
// The inline Append also keeps a per-tenant process mutex, which only spares
// the database a futile round of lock waits inside one process; it is not what
// makes the order correct, and the transaction-bound Enlist does not use it (a
// process mutex must never wrap a business transaction).
type GormRealtimeEventRepository struct {
	db *gorm.DB

	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex

	// wake is called after a transaction that enlisted events has committed.
	wake atomic.Pointer[func()]
}

// NewGormRealtimeEventRepository builds the durable event log.
func NewGormRealtimeEventRepository(db *gorm.DB) *GormRealtimeEventRepository {
	return &GormRealtimeEventRepository{db: db, locks: map[uuid.UUID]*sync.Mutex{}}
}

func (r *GormRealtimeEventRepository) tenantLock(tenantID uuid.UUID) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.locks[tenantID]
	if !ok {
		l = &sync.Mutex{}
		r.locks[tenantID] = l
	}
	return l
}

// maxReplayLimit caps one replay response.
//
// A reconnecting client asks for everything it missed; without a cap, a client
// that was away for the whole retention window would ask the database for the
// tenant's entire day in one query and then be handed it in one burst. The
// cursor makes paging free — the client simply reconnects with the last id it
// received — so the cap costs nothing and removes the burst.
const maxReplayLimit = 500

// Append assigns the next per-tenant sequence and stores the event, already
// marked published: this is the post-commit path, the publisher delivers it
// right after, and the relay must never see it.
//
// The caller's Sequence is ignored and overwritten: a publisher can never choose
// its own position in a tenant's order.
func (r *GormRealtimeEventRepository) Append(ctx context.Context, e *domain.RealtimeEvent) error {
	if e == nil {
		return fmt.Errorf("nil realtime event")
	}
	if e.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if e.ID == uuid.Nil {
		return fmt.Errorf("event id is required")
	}
	now := time.Now().UTC()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = now
	}
	e.CreatedAt = now
	e.PublishedAt = &now
	e.AbandonedAt, e.LockedUntil, e.Attempts, e.LastError = nil, nil, 0, ""

	lock := r.tenantLock(e.TenantID)
	lock.Lock()
	defer lock.Unlock()

	// SkipHooks so appending an event never re-enters the audittrail GORM
	// plugin: the event log records that something changed, it is not itself a
	// business change worth auditing, and letting it feed the trail would make
	// every mutation produce two entries.
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Transaction(func(tx *gorm.DB) error {
			first, err := allocateRealtimeSequence(tx, e.TenantID, 1)
			if err != nil {
				return err
			}
			e.Sequence = first
			return tx.Create(e).Error
		})
}

// allocateRealtimeSequence reserves n consecutive sequences for a tenant inside
// tx and returns the first. The upsert takes the counter row lock, which tx
// holds until it ends.
func allocateRealtimeSequence(tx *gorm.DB, tenantID uuid.UUID, n int) (int64, error) {
	greatest := "GREATEST"
	if tx.Dialector.Name() == "sqlite" {
		greatest = "MAX"
	}
	var last int64
	err := tx.Raw(`INSERT INTO realtime_event_sequences (tenant_id, last_sequence, updated_at)
VALUES (?, COALESCE((SELECT MAX(sequence) FROM realtime_events WHERE tenant_id = ?), 0) + ?, ?)
ON CONFLICT (tenant_id) DO UPDATE
   SET last_sequence = `+greatest+`(realtime_event_sequences.last_sequence,
                                  COALESCE((SELECT MAX(sequence) FROM realtime_events WHERE tenant_id = ?), 0)) + ?,
       updated_at = excluded.updated_at
RETURNING last_sequence`,
		tenantID, tenantID, n, time.Now().UTC(), tenantID, n).Scan(&last).Error
	if err != nil {
		return 0, fmt.Errorf("failed to allocate realtime sequence: %w", err)
	}
	return last - int64(n) + 1, nil
}

// OnEnlistCommitted registers the function called after a transaction that
// enlisted events has committed (it wakes the relay). Set once at boot.
func (r *GormRealtimeEventRepository) OnEnlistCommitted(wake func()) {
	if wake == nil {
		r.wake.Store(nil)
		return
	}
	r.wake.Store(&wake)
}

// notifyEnlistCommitted calls the registered wake function, if any. The
// function must not block.
func (r *GormRealtimeEventRepository) notifyEnlistCommitted() {
	if w := r.wake.Load(); w != nil {
		(*w)()
	}
}

// Bind returns the outbox bound to tx, the caller's open transaction.
func (r *GormRealtimeEventRepository) Bind(tx *gorm.DB) *TxRealtimeOutbox {
	return &TxRealtimeOutbox{tx: tx}
}

// TxRealtimeOutbox writes events inside the caller's transaction. It implements
// domain.RealtimeEventOutbox.
type TxRealtimeOutbox struct {
	tx       *gorm.DB
	enlisted int
}

// Enlisted is how many events this transaction has enlisted so far.
func (o *TxRealtimeOutbox) Enlisted() int { return o.enlisted }

// Enlist stores events as unpublished rows in the caller's transaction, with
// consecutive per-tenant sequences in slice order. Every event must carry an id
// and the same non-nil tenant; anything else is a validation error, which rolls
// the caller back rather than storing a row in the wrong place.
//
// It must be the LAST write of the transaction: the tenant's sequence counter
// stays locked until commit.
func (o *TxRealtimeOutbox) Enlist(ctx context.Context, events ...*domain.RealtimeEvent) error {
	if len(events) == 0 {
		return nil
	}
	var tenant uuid.UUID
	for i, e := range events {
		if e == nil || e.ID == uuid.Nil || e.TenantID == uuid.Nil {
			return domain.NewValidationError("realtime outbox: every event needs an id and a tenant")
		}
		if i == 0 {
			tenant = e.TenantID
		} else if e.TenantID != tenant {
			return domain.NewValidationError("realtime outbox: all events of one call must belong to one tenant")
		}
	}
	// SkipHooks: see Append. Same transaction as the caller.
	s := o.tx.WithContext(ctx).Session(&gorm.Session{SkipHooks: true})
	first, err := allocateRealtimeSequence(s, tenant, len(events))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i, e := range events {
		e.Sequence = first + int64(i)
		e.CreatedAt = now
		if e.OccurredAt.IsZero() {
			e.OccurredAt = now
		}
		e.PublishedAt, e.AbandonedAt, e.LockedUntil = nil, nil, nil
		e.Attempts, e.LastError = 0, ""
	}
	if err := s.CreateInBatches(events, 200).Error; err != nil {
		return fmt.Errorf("failed to enlist realtime events: %w", err)
	}
	o.enlisted += len(events)
	return nil
}

// Replay returns the tenant's events after a cursor, oldest first.
func (r *GormRealtimeEventRepository) Replay(ctx context.Context, tenantID uuid.UUID, after int64, limit int) ([]domain.RealtimeEvent, error) {
	if tenantID == uuid.Nil {
		// Fail closed. A nil tenant here would mean an unscoped query returning
		// every tenant's history, which is the single worst outcome this file
		// can produce.
		return nil, fmt.Errorf("tenant_id is required")
	}
	if limit <= 0 || limit > maxReplayLimit {
		limit = maxReplayLimit
	}
	if after < 0 {
		after = 0
	}
	var rows []domain.RealtimeEvent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND sequence > ?", tenantID, after).
		Order("sequence ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to replay realtime events: %w", err)
	}
	return rows, nil
}

// Bounds returns the oldest and newest sequence still stored for a tenant.
// Both are zero when the tenant has no events at all.
func (r *GormRealtimeEventRepository) Bounds(ctx context.Context, tenantID uuid.UUID) (int64, int64, error) {
	if tenantID == uuid.Nil {
		return 0, 0, fmt.Errorf("tenant_id is required")
	}
	var res struct {
		Oldest *int64
		Newest *int64
	}
	err := r.db.WithContext(ctx).
		Model(&domain.RealtimeEvent{}).
		Select("MIN(sequence) AS oldest, MAX(sequence) AS newest").
		Where("tenant_id = ?", tenantID).
		Scan(&res).Error
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read realtime log bounds: %w", err)
	}
	var oldest, newest int64
	if res.Oldest != nil {
		oldest = *res.Oldest
	}
	if res.Newest != nil {
		newest = *res.Newest
	}
	return oldest, newest, nil
}

// PurgeBefore drops events older than cutoff, across all tenants. A row still
// waiting for the relay is never purged, whatever its age: it is an
// announcement not yet made, and legacy rows read as published (migration 0085).
func (r *GormRealtimeEventRepository) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	// tenant: instance-level retention sweep, cross-tenant by design (see the port).
	res := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Where("created_at < ? AND (published_at IS NOT NULL OR abandoned_at IS NOT NULL)", cutoff.UTC()).
		Delete(&domain.RealtimeEvent{})
	if res.Error != nil {
		return 0, fmt.Errorf("failed to purge realtime events: %w", res.Error)
	}
	return res.RowsAffected, nil
}
