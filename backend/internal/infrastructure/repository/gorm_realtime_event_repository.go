// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRealtimeEventRepository is the durable, per-tenant ordered event log.
//
// Appending is serialised per tenant so two concurrent publishers can never
// claim the same sequence number — the same three-layer guarantee the audit
// chain uses, for the same reason:
//
//   - inside the process, a per-tenant mutex;
//   - inside the database, the append reads the tenant's current head
//     FOR UPDATE (Postgres);
//   - the unique index on (tenant_id, sequence) is the final backstop: a racing
//     insert fails loudly rather than silently forking the order.
//
// Honest limit: the FOR UPDATE row lock is what makes this correct across
// multiple API instances. On SQLite (tests) the locking clause is skipped and
// the process mutex plus the unique index carry the guarantee.
type GormRealtimeEventRepository struct {
	db *gorm.DB

	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
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

func (r *GormRealtimeEventRepository) supportsRowLock() bool {
	return r.db.Dialector.Name() == "postgres"
}

// maxReplayLimit caps one replay response.
//
// A reconnecting client asks for everything it missed; without a cap, a client
// that was away for the whole retention window would ask the database for the
// tenant's entire day in one query and then be handed it in one burst. The
// cursor makes paging free — the client simply reconnects with the last id it
// received — so the cap costs nothing and removes the burst.
const maxReplayLimit = 500

// Append assigns the next per-tenant sequence and stores the event.
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

	lock := r.tenantLock(e.TenantID)
	lock.Lock()
	defer lock.Unlock()

	// SkipHooks so appending an event never re-enters the audittrail GORM
	// plugin: the event log records that something changed, it is not itself a
	// business change worth auditing, and letting it feed the trail would make
	// every mutation produce two entries.
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Transaction(func(tx *gorm.DB) error {
			q := tx.Model(&domain.RealtimeEvent{}).Where("tenant_id = ?", e.TenantID)
			if r.supportsRowLock() {
				q = q.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			var head domain.RealtimeEvent
			err := q.Order("sequence DESC").Limit(1).Take(&head).Error
			switch {
			case err == nil:
				e.Sequence = head.Sequence + 1
			case err == gorm.ErrRecordNotFound:
				// Sequences start at 1 so that zero can mean "no cursor yet"
				// without ambiguity on the wire.
				e.Sequence = 1
			default:
				return fmt.Errorf("failed to read realtime log head: %w", err)
			}
			return tx.Create(e).Error
		})
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

// PurgeBefore drops events older than cutoff, across all tenants.
func (r *GormRealtimeEventRepository) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).
		Where("created_at < ?", cutoff.UTC()).
		Delete(&domain.RealtimeEvent{})
	if res.Error != nil {
		return 0, fmt.Errorf("failed to purge realtime events: %w", res.Error)
	}
	return res.RowsAffected, nil
}
