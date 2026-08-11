// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormMitigationDueRepository is a focused, cross-tenant view of mitigation
// plans used by the D-7 / D-1 deadline sweep.
//
// Cross-tenant on purpose and by necessity: a scheduled job has no session. It
// is deliberately NOT part of repository.MitigationRepository (which is
// tenant-scoped) so the reminder cadence cannot become a way to read another
// tenant's plans through a normal request path — every row it returns carries
// its own tenant_id, and the notification is addressed with that.
type GormMitigationDueRepository struct{ db *gorm.DB }

func NewGormMitigationDueRepository(db *gorm.DB) *GormMitigationDueRepository {
	return &GormMitigationDueRepository{db: db}
}

// ListDueForReminder returns unfinished plans whose deadline has entered the
// widest reminder window (D-7) and that still have a reminder left to send.
//
// The SQL narrows; domain.Mitigation.DueReminderDue decides. Keeping the
// decision in the domain means the rule is tested without a database, and the
// query only has to avoid dragging the whole table into memory.
func (r *GormMitigationDueRepository) ListDueForReminder(ctx context.Context, now time.Time) ([]domain.Mitigation, error) {
	widest := domain.DueReminderOffsets[0] // 7 days
	horizon := now.AddDate(0, 0, widest)

	var rows []domain.Mitigation
	err := r.db.WithContext(ctx).
		Where("due_date IS NOT NULL AND due_date <= ?", horizon).
		Where("status NOT IN ?", []domain.MitigationStatus{domain.MitigationDone, domain.MitigationCancelled}).
		Where("deleted_at IS NULL").
		// At least one nudge still unsent. Without this the sweep would re-read
		// every overdue plan for ever.
		Where("reminder_d7_sent_at IS NULL OR reminder_d1_sent_at IS NULL").
		Order("due_date ASC").
		Limit(500).
		Find(&rows).Error
	return rows, err
}

// MarkReminderSent stamps the reminder that was just sent. Reaching D-1 also
// closes D-7 (see domain.Mitigation.MarkReminderSent): sending the earlier,
// less urgent nudge afterwards would be noise.
func (r *GormMitigationDueRepository) MarkReminderSent(ctx context.Context, id uuid.UUID, offset int, at time.Time) error {
	updates := map[string]interface{}{}
	switch offset {
	case 1:
		updates["reminder_d1_sent_at"] = at
		updates["reminder_d7_sent_at"] = gorm.Expr("COALESCE(reminder_d7_sent_at, ?)", at)
	case 7:
		updates["reminder_d7_sent_at"] = at
	default:
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&domain.Mitigation{}).
		Where("id = ?", id).
		Updates(updates).Error
}
