// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormNoticeEmailOutbox is the notice e-mail outbox and its throttle (#997,
// migration 0081). Postgres only: the decision "send now or add to the digest"
// takes a row lock, and the worker claims with SKIP LOCKED.
//
// Enqueue and the per-row updates are scoped by the row's tenant_id. ClaimDue
// and Prune are system jobs that read across tenants by necessity, like the
// deadline sweep: ClaimDue returns rows that each carry their own tenant, and
// Prune deletes by age and status alone, returning nothing.
type GormNoticeEmailOutbox struct {
	db *gorm.DB
}

func NewGormNoticeEmailOutbox(db *gorm.DB) *GormNoticeEmailOutbox {
	return &GormNoticeEmailOutbox{db: db}
}

// Enqueue writes one notice to the outbox and returns the row's kind and when
// it is due.
//
// A notice that is not grouped (no item, or a producer that deduplicates) is a
// single row due now. Otherwise, under the throttle row's lock for (tenant,
// user, item): a pending digest takes the event; else an e-mail sent less than
// window ago opens a digest due when the window ends; else the notice goes out
// now and restamps the throttle.
func (r *GormNoticeEmailOutbox) Enqueue(ctx context.Context, row domain.NoticeEmailOutbox, window time.Duration) (string, time.Time, error) {
	if row.TenantID == uuid.Nil || row.UserID == uuid.Nil || len(row.Events) == 0 {
		return "", time.Time{}, domain.NewValidationError("tenant, user and event are required")
	}
	now := time.Now()
	row.ID = uuid.New()
	row.Status = domain.NoticeEmailPending
	row.CreatedAt = now

	if !row.Grouped || row.ResourceID == nil || *row.ResourceID == uuid.Nil {
		row.Kind, row.DueAt, row.Grouped = domain.NoticeEmailSingle, now, false
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return "", time.Time{}, err
		}
		return row.Kind, row.DueAt, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The throttle row is created if missing ("epoch": never e-mailed),
		// then locked: every enqueue for this key waits for the one before.
		key := []interface{}{row.TenantID, row.UserID, row.ResourceType, *row.ResourceID}
		if err := tx.Exec(`
			INSERT INTO notice_email_throttle (tenant_id, user_id, resource_type, resource_id, sent_at)
			VALUES (?, ?, ?, ?, 'epoch')
			ON CONFLICT DO NOTHING`, key...).Error; err != nil {
			return err
		}
		var sentAt []time.Time
		if err := tx.Raw(`
			SELECT sent_at FROM notice_email_throttle
			 WHERE tenant_id = ? AND user_id = ? AND resource_type = ? AND resource_id = ?
			   FOR UPDATE`, key...).Scan(&sentAt).Error; err != nil {
			return err
		}

		// A pending digest takes the event. The status check in the UPDATE
		// matters: the worker may have claimed the digest since the SELECT.
		var digest []struct {
			ID    uuid.UUID
			DueAt time.Time
		}
		if err := tx.Raw(`
			SELECT id, due_at FROM notice_email_outbox
			 WHERE tenant_id = ? AND user_id = ? AND resource_type = ? AND resource_id = ?
			   AND kind = ? AND status = ?
			 ORDER BY due_at LIMIT 1`,
			append(key, domain.NoticeEmailDigest, domain.NoticeEmailPending)...).Scan(&digest).Error; err != nil {
			return err
		}
		claimedMeanwhile := false
		if len(digest) == 1 {
			events, err := json.Marshal(row.Events)
			if err != nil {
				return err
			}
			// Past the cap the oldest change is counted instead of kept, so
			// the e-mail stays a size any relay accepts.
			res := tx.Exec(`
				UPDATE notice_email_outbox
				   SET events = CASE WHEN jsonb_array_length(events) >= ? THEN (events - 0) || ?::jsonb
				                     ELSE events || ?::jsonb END,
				       omitted = omitted + CASE WHEN jsonb_array_length(events) >= ? THEN 1 ELSE 0 END,
				       resource_title = ?, locale = ?
				 WHERE id = ? AND tenant_id = ? AND status = ?`,
				domain.NoticeEmailDigestCap, string(events), string(events), domain.NoticeEmailDigestCap,
				row.ResourceTitle, row.Locale, digest[0].ID, row.TenantID, domain.NoticeEmailPending)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 1 {
				row.Kind, row.DueAt = domain.NoticeEmailDigest, digest[0].DueAt
				return nil
			}
			claimedMeanwhile = true
		}

		last := time.Time{}
		if len(sentAt) == 1 {
			last = sentAt[0]
		}
		switch {
		case claimedMeanwhile:
			// That digest is going out now; this event waits a full window.
			row.Kind, row.DueAt = domain.NoticeEmailDigest, now.Add(window)
		case now.Sub(last) < window:
			row.Kind, row.DueAt = domain.NoticeEmailDigest, last.Add(window)
		default:
			row.Kind, row.DueAt = domain.NoticeEmailSingle, now
			if err := tx.Exec(`
				UPDATE notice_email_throttle SET sent_at = ?
				 WHERE tenant_id = ? AND user_id = ? AND resource_type = ? AND resource_id = ?`,
				append([]interface{}{now}, key...)...).Error; err != nil {
				return err
			}
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return row.Kind, row.DueAt, nil
}

// ClaimDue takes up to limit rows that are due, plus rows whose lease expired
// (a replica died mid-send), and leases them for lease. SKIP LOCKED lets
// replicas split the work, and rows are taken in turn across tenants (see
// below). Each grouped row claimed restamps its throttle, so a notice arriving
// while it is sent waits for the next window.
//
// A row whose lease expired after maxAttempts claims never came back from its
// send: it crashed the process each time. It is marked failed, kept and logged
// by the caller, rather than taken again forever.
func (r *GormNoticeEmailOutbox) ClaimDue(ctx context.Context, limit int, lease time.Duration, maxAttempts int) ([]domain.NoticeEmailOutbox, error) {
	if err := r.db.WithContext(ctx).Exec(`
		UPDATE notice_email_outbox
		   SET status = ?, locked_until = NULL,
		       last_error = 'no outcome recorded after the last attempt (the send never returned)'
		 WHERE status = ? AND locked_until < now() AND attempts >= ?`,
		domain.NoticeEmailFailed, domain.NoticeEmailSending, maxAttempts).Error; err != nil {
		return nil, err
	}
	// Fair between organizations: due rows are ranked within their tenant by
	// due_at, and taken rank by rank, so every tenant's oldest e-mail goes
	// before any tenant's second. One tenant's burst (a bulk reassignment)
	// waits its turn instead of delaying everyone else's mail.
	//
	// Grouped rows about one item go out in the order they were queued: a row
	// waits while an earlier one for the same (tenant, user, item) is pending
	// or sending. After a relay outage the first e-mail has backed off further
	// than the digest that follows it; without this the digest ("3 changes,
	// current state") would arrive first and the stale first notice after it.
	//
	// The due condition is repeated on the locked row: FOR UPDATE re-checks it
	// against the row's latest version, so a row another replica claimed
	// between the ranking and the lock is skipped, never claimed twice.
	var rows []domain.NoticeEmailOutbox
	err := r.db.WithContext(ctx).Raw(`
		WITH ranked AS (
		     SELECT n.id, n.due_at, row_number() OVER (PARTITION BY n.tenant_id ORDER BY n.due_at, n.id) AS turn
		       FROM notice_email_outbox n
		      WHERE ((n.status = @pending AND n.due_at <= now())
		          OR (n.status = @sending AND n.locked_until < now()))
		        AND NOT (n.grouped AND EXISTS (
		             SELECT 1 FROM notice_email_outbox e
		              WHERE e.tenant_id = n.tenant_id AND e.user_id = n.user_id
		                AND e.resource_type = n.resource_type AND e.resource_id = n.resource_id
		                AND e.grouped AND e.status IN (@pending, @sending)
		                AND (e.created_at, e.id) < (n.created_at, n.id)))
		), picked AS (
		     SELECT o.id
		       FROM notice_email_outbox o
		       JOIN ranked r ON r.id = o.id
		      WHERE (o.status = @pending AND o.due_at <= now())
		         OR (o.status = @sending AND o.locked_until < now())
		      ORDER BY r.turn, r.due_at, r.id
		      LIMIT @limit
		        FOR UPDATE OF o SKIP LOCKED
		)
		UPDATE notice_email_outbox
		   SET status = @sending, locked_until = now() + make_interval(secs => @lease), attempts = attempts + 1
		 WHERE id IN (SELECT id FROM picked)
		RETURNING *`,
		sql.Named("pending", domain.NoticeEmailPending),
		sql.Named("sending", domain.NoticeEmailSending),
		sql.Named("limit", limit),
		sql.Named("lease", lease.Seconds()),
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !row.Grouped || row.ResourceID == nil {
			continue
		}
		if err := r.db.WithContext(ctx).Exec(`
			UPDATE notice_email_throttle SET sent_at = GREATEST(sent_at, now())
			 WHERE tenant_id = ? AND user_id = ? AND resource_type = ? AND resource_id = ?`,
			row.TenantID, row.UserID, row.ResourceType, *row.ResourceID).Error; err != nil {
			return rows, err
		}
	}
	return rows, nil
}

// MarkSent closes a row that went out.
func (r *GormNoticeEmailOutbox) MarkSent(ctx context.Context, row domain.NoticeEmailOutbox) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE notice_email_outbox
		   SET status = ?, sent_at = now(), locked_until = NULL, last_error = ''
		 WHERE id = ? AND tenant_id = ?`,
		domain.NoticeEmailSent, row.ID, row.TenantID).Error
}

// Retry puts a row back in the queue, due at at. reason never carries an
// address (the caller redacts it).
func (r *GormNoticeEmailOutbox) Retry(ctx context.Context, row domain.NoticeEmailOutbox, at time.Time, reason string) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE notice_email_outbox
		   SET status = ?, due_at = ?, locked_until = NULL, last_error = ?
		 WHERE id = ? AND tenant_id = ?`,
		domain.NoticeEmailPending, at, reason, row.ID, row.TenantID).Error
}

// Close ends a row without sending it: failed after its last attempt, or
// skipped because the recipient may no longer receive it.
func (r *GormNoticeEmailOutbox) Close(ctx context.Context, row domain.NoticeEmailOutbox, status, reason string) error {
	if status != domain.NoticeEmailFailed && status != domain.NoticeEmailSkipped {
		return domain.NewValidationError("close status must be failed or skipped")
	}
	return r.db.WithContext(ctx).Exec(`
		UPDATE notice_email_outbox
		   SET status = ?, locked_until = NULL, last_error = ?
		 WHERE id = ? AND tenant_id = ?`,
		status, reason, row.ID, row.TenantID).Error
}

// Prune deletes what no longer serves: sent and skipped rows after a week,
// failed rows after a month (they are what an operator looks at), and throttle
// stamps older than the longest window.
func (r *GormNoticeEmailOutbox) Prune(ctx context.Context) (int64, error) {
	var total int64
	for _, q := range []struct {
		sql  string
		args []interface{}
	}{
		{`DELETE FROM notice_email_outbox WHERE status IN (?, ?) AND created_at < now() - interval '7 days'`,
			[]interface{}{domain.NoticeEmailSent, domain.NoticeEmailSkipped}},
		{`DELETE FROM notice_email_outbox WHERE status = ? AND created_at < now() - interval '30 days'`,
			[]interface{}{domain.NoticeEmailFailed}},
		{`DELETE FROM notice_email_throttle WHERE sent_at < now() - interval '25 hours'`, nil},
	} {
		res := r.db.WithContext(ctx).Exec(q.sql, q.args...)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

// GormNoticeEmailPolicyRepository stores one grouping window per tenant.
// Every method filters on the tenant it is given.
type GormNoticeEmailPolicyRepository struct {
	db *gorm.DB
}

func NewGormNoticeEmailPolicyRepository(db *gorm.DB) *GormNoticeEmailPolicyRepository {
	return &GormNoticeEmailPolicyRepository{db: db}
}

// GetNoticeEmailPolicy returns the tenant's saved policy, or nil when it never
// saved one.
func (r *GormNoticeEmailPolicyRepository) GetNoticeEmailPolicy(ctx context.Context, tenantID uuid.UUID) (*domain.NoticeEmailPolicy, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("no organization in context")
	}
	var rows []domain.NoticeEmailPolicy
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// SaveNoticeEmailPolicy creates or updates the tenant's policy. The update is
// keyed on tenant_id, never on an id the caller supplied.
func (r *GormNoticeEmailPolicyRepository) SaveNoticeEmailPolicy(ctx context.Context, p *domain.NoticeEmailPolicy) error {
	if p == nil || p.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant is required")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	existing, err := r.GetNoticeEmailPolicy(ctx, p.TenantID)
	if err != nil {
		return err
	}
	if existing == nil {
		p.ID = uuid.New()
		return r.db.WithContext(ctx).Create(p).Error
	}
	// Struct-form update so the audit trail records the before and after.
	existing.WindowMinutes = p.WindowMinutes
	existing.UpdatedByID = p.UpdatedByID
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", p.TenantID).Save(existing).Error; err != nil {
		return err
	}
	*p = *existing
	return nil
}
