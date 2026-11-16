// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// The relay's side of the realtime outbox (ADR 0010). It satisfies
// application/realtime.OutboxStore. Postgres only: the claim needs row locks,
// window functions and SKIP LOCKED, and its tests run against a real database.
//
// Tenant filtering. Every per-row statement below filters on tenant_id. Three
// statements are cross-tenant on purpose, as PurgeBefore is, and each is
// instance-level and leads only into a gated statement:
//
//   - AbandonExpired: the poison-row sweep;
//   - ClaimPending statement 1: returns tenant ids only;
//   - PendingStats: aggregates only.
//
// ClaimPending statement 2 is gated by the tenant list statement 1 returned.

// requirePostgres refuses the relay statements on any other dialect.
func (r *GormRealtimeEventRepository) requirePostgres() error {
	if r.db.Dialector.Name() != "postgres" {
		return fmt.Errorf("realtime outbox relay requires postgres, got %s", r.db.Dialector.Name())
	}
	return nil
}

// AbandonExpired gives up on rows that were claimed maxAttempts times and never
// came back with an outcome: they crashed the relay each time. It returns how
// many were abandoned. Cross-tenant by design (instance-level poison sweep).
func (r *GormRealtimeEventRepository) AbandonExpired(ctx context.Context, maxAttempts int) (int64, error) {
	if err := r.requirePostgres(); err != nil {
		return 0, err
	}
	// tenant: instance-level poison-row sweep, cross-tenant by design; touches only relay bookkeeping columns.
	res := tenantguard.CrossTenant(r.db.WithContext(ctx), "realtime outbox poison-row sweep across every tenant; touches only relay bookkeeping columns").Session(&gorm.Session{SkipHooks: true}).Exec(`
		UPDATE realtime_events
		   SET abandoned_at = now(), locked_until = NULL,
		       last_error = 'abandoned: no outcome recorded after the last attempt (the relay never returned)'
		 WHERE published_at IS NULL AND abandoned_at IS NULL AND attempts >= ? AND locked_until < now()`,
		maxAttempts)
	return res.RowsAffected, res.Error
}

// lockClaimableTenants is ClaimPending's statement 1. It is a function of its
// own so the cursor is closed by defer before statement 2 runs on the same tx.
func lockClaimableTenants(tx *gorm.DB, maxTenants int) ([]uuid.UUID, error) {
	// tenant: returns tenant ids only, cross-tenant by design; every later statement is gated by that list.
	cur, err := tenantguard.CrossTenant(tx, "outbox claim statement 1 returns tenant ids only; every later statement is gated by that list").Raw(`
		SELECT s.tenant_id
		  FROM realtime_event_sequences s
		  JOIN (SELECT DISTINCT ON (tenant_id) tenant_id, created_at AS oldest, locked_until
		          FROM realtime_events
		         WHERE published_at IS NULL AND abandoned_at IS NULL
		         ORDER BY tenant_id, sequence) p ON p.tenant_id = s.tenant_id
		 WHERE p.locked_until IS NULL OR p.locked_until <= now()
		 ORDER BY p.oldest
		 LIMIT ?
		   FOR UPDATE OF s SKIP LOCKED`, maxTenants).Rows()
	if err != nil {
		return nil, err
	}
	defer cur.Close()
	var tenants []uuid.UUID
	for cur.Next() {
		var id uuid.UUID
		if err := cur.Scan(&id); err != nil {
			return nil, err
		}
		tenants = append(tenants, id)
	}
	return tenants, cur.Err()
}

// ClaimPending leases pending rows for delivery, per tenant in sequence order.
//
// Statement 1 considers only tenants whose first pending row (by sequence) is
// claimable now, neither leased nor backed off: a tenant whose head is blocked
// would fill the LIMIT, claim nothing in statement 2 and starve the tenants
// behind it. It locks up to maxTenants of them through their counter rows with
// SKIP LOCKED: a tenant another relay is working on, or a tenant with a
// transaction still in flight (which holds the same counter row), is skipped.
// That lock is what guarantees no uncommitted lower sequence exists for a
// tenant whose rows are claimed. Statement 2 is a new statement, so READ
// COMMITTED gives it a fresh snapshot taken after the locks, and claims at most
// perTenant rows of each locked tenant, stopping at the first row that is
// leased or backed off: that row blocks itself and everything after it.
func (r *GormRealtimeEventRepository) ClaimPending(ctx context.Context, maxTenants, perTenant int, lease time.Duration) ([]domain.RealtimeEvent, error) {
	if err := r.requirePostgres(); err != nil {
		return nil, err
	}
	var rows []domain.RealtimeEvent
	err := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Transaction(func(tx *gorm.DB) error {
		tenants, err := lockClaimableTenants(tx, maxTenants)
		if err != nil {
			return err
		}
		if len(tenants) == 0 {
			return nil
		}
		return tx.Raw(`
			WITH pending AS (
			  SELECT e.id, e.tenant_id,
			         row_number() OVER w AS turn,
			         bool_or(e.locked_until IS NOT NULL AND e.locked_until > now()) OVER w AS blocked
			    FROM realtime_events e
			   WHERE e.tenant_id IN ? AND e.published_at IS NULL AND e.abandoned_at IS NULL
			  WINDOW w AS (PARTITION BY e.tenant_id ORDER BY e.sequence ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
			)
			UPDATE realtime_events r
			   SET locked_until = now() + make_interval(secs => ?), attempts = r.attempts + 1
			  FROM pending p
			 WHERE r.id = p.id AND r.tenant_id = p.tenant_id AND NOT p.blocked AND p.turn <= ?
			RETURNING r.*`,
			tenants, lease.Seconds(), perTenant).Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("failed to claim realtime outbox rows: %w", err)
	}
	return rows, nil
}

// MarkPublished closes delivered rows of one tenant and returns how many it
// closed. Fewer than len(ids) means a lease expired and another relay got
// there first (at-least-once), which is benign.
func (r *GormRealtimeEventRepository) MarkPublished(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (int64, error) {
	if tenantID == uuid.Nil {
		return 0, fmt.Errorf("tenant_id is required")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := r.requirePostgres(); err != nil {
		return 0, err
	}
	res := r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Exec(`
		UPDATE realtime_events
		   SET published_at = now(), locked_until = NULL, last_error = ''
		 WHERE tenant_id = ? AND id IN ? AND published_at IS NULL AND abandoned_at IS NULL`,
		tenantID, ids)
	return res.RowsAffected, res.Error
}

// Retry records a failed delivery: the row stays pending and is held back until
// notBefore. The attempt was already counted when it was claimed. It acts only
// while the row still carries the lease this relay was given (locked_until =
// lease, the deadline ClaimPending returned): if the lease expired and another
// relay claimed the row, that relay's lease is left alone.
func (r *GormRealtimeEventRepository) Retry(ctx context.Context, tenantID, id uuid.UUID, lease, notBefore time.Time, reason string) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if err := r.requirePostgres(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Exec(`
		UPDATE realtime_events
		   SET locked_until = ?, last_error = ?
		 WHERE tenant_id = ? AND id = ? AND locked_until = ? AND published_at IS NULL AND abandoned_at IS NULL`,
		notBefore.UTC(), reason, tenantID, id, lease.UTC()).Error
}

// Release puts claimed rows back without counting the claim as an attempt: they
// were not tried (they sat behind a failed head, or the relay was stopping).
// Like Retry it acts only on rows still carrying the lease deadline this relay
// was given (one ClaimPending statement stamps one deadline on all its rows),
// so a row another relay has since claimed keeps its lease and its attempt.
func (r *GormRealtimeEventRepository) Release(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID, lease time.Time) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if len(ids) == 0 {
		return nil
	}
	if err := r.requirePostgres(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Exec(`
		UPDATE realtime_events
		   SET locked_until = NULL, attempts = GREATEST(attempts - 1, 0)
		 WHERE tenant_id = ? AND id IN ? AND locked_until = ? AND published_at IS NULL AND abandoned_at IS NULL`,
		tenantID, ids, lease.UTC()).Error
}

// Abandon gives up on one row for good. The row stays in the table with its
// reason, and a replay still serves it.
func (r *GormRealtimeEventRepository) Abandon(ctx context.Context, tenantID, id uuid.UUID, reason string) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if err := r.requirePostgres(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Exec(`
		UPDATE realtime_events
		   SET abandoned_at = now(), locked_until = NULL, last_error = ?
		 WHERE tenant_id = ? AND id = ? AND published_at IS NULL AND abandoned_at IS NULL`,
		reason, tenantID, id).Error
}

// PendingStats reports how many rows wait for the relay and when the oldest was
// written. Cross-tenant and aggregate-only, served by the partial index. Every
// instance reports the same global value: aggregate it with max(), never sum().
func (r *GormRealtimeEventRepository) PendingStats(ctx context.Context) (int64, *time.Time, error) {
	var res struct {
		N      int64
		Oldest *time.Time
	}
	// tenant: aggregate-only gauge over the whole instance, no row data leaves.
	err := tenantguard.CrossTenant(r.db.WithContext(ctx), "aggregate-only gauge over the whole instance; no row data leaves").Raw(`
		SELECT COUNT(*) AS n, MIN(created_at) AS oldest
		  FROM realtime_events
		 WHERE published_at IS NULL AND abandoned_at IS NULL`).Scan(&res).Error
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read realtime outbox stats: %w", err)
	}
	return res.N, res.Oldest, nil
}
