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
)

// The reminder sweep's half of GormVendorAssessmentRepository (#672, ADR 0004 D6).

func vendorReminderColumn(offset int) (string, error) {
	switch offset {
	case 7:
		return "reminder_d7_sent_at", nil
	case 3:
		return "reminder_d3_sent_at", nil
	case 1:
		return "reminder_d1_sent_at", nil
	}
	return "", fmt.Errorf("vendor reminder: unknown offset %d", offset)
}

// ListOpenAssessmentsDueWithin returns open assessments across tenants whose due
// date is after now and within horizon, with a reminder still unsent. No items.
func (r *GormVendorAssessmentRepository) ListOpenAssessmentsDueWithin(ctx context.Context, now time.Time, horizon time.Duration) ([]domain.VendorAssessment, error) {
	out := []domain.VendorAssessment{}
	err := r.db.WithContext(ctx).
		// Cross-tenant BY NECESSITY: a scheduled sweep holds no session; every row carries its own tenant_id, which addresses all that is done with it.
		Where("status IN ? AND due_at > ? AND due_at <= ?", openAssessmentStatuses, now, now.Add(horizon)).
		Where("(reminder_d7_sent_at IS NULL OR reminder_d3_sent_at IS NULL OR reminder_d1_sent_at IS NULL)").
		Order("due_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list assessments due for a reminder: %w", err)
	}
	return out, nil
}

// RecordReminder stamps offset and every larger offset still unstamped and, when
// tok is not nil, supersedes the active token and inserts tok — one transaction,
// conditional on the assessment being open AND this reminder being unsent. The
// condition is what makes two overlapping sweeps send one reminder, not two.
func (r *GormVendorAssessmentRepository) RecordReminder(ctx context.Context, a *domain.VendorAssessment, offset int, tok *domain.VendorAssessmentToken, at time.Time) (bool, error) {
	if a == nil || a.TenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	column, err := vendorReminderColumn(offset)
	if err != nil {
		return false, err
	}
	if tok != nil && (tok.TenantID != a.TenantID || tok.AssessmentID != a.ID) {
		return false, fmt.Errorf("vendor reminder: the token does not belong to this assessment")
	}

	updates := map[string]interface{}{"updated_at": at}
	for _, o := range domain.VendorReminderOffsets {
		if o < offset {
			continue
		}
		c, _ := vendorReminderColumn(o)
		// COALESCE keeps an earlier stamp: it records when that reminder really went out.
		updates[c] = gorm.Expr("COALESCE("+c+", ?)", at)
	}

	recorded := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.VendorAssessment{}).
			Where("id = ? AND tenant_id = ? AND status IN ? AND "+column+" IS NULL", a.ID, a.TenantID, openAssessmentStatuses).
			Updates(updates)
		if res.Error != nil {
			return fmt.Errorf("stamp reminder: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if tok != nil {
			if err := tx.Model(&domain.VendorAssessmentToken{}).
				Where("assessment_id = ? AND tenant_id = ? AND superseded_at IS NULL", a.ID, a.TenantID).
				Update("superseded_at", at).Error; err != nil {
				return fmt.Errorf("supersede token for reminder: %w", err)
			}
			if err := tx.Create(tok).Error; err != nil {
				return fmt.Errorf("issue reminder token: %w", err)
			}
		}
		recorded = true
		return nil
	})
	return recorded, err
}
