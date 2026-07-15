// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRiskReviewRepository is a focused, cross-tenant view of risks used by the
// RiskReviewWorker. It is intentionally NOT part of domain.RiskRepository (which
// is tenant-scoped and paginated) so the review cadence stays isolated and does
// not churn that interface or its mocks.
type GormRiskReviewRepository struct{ db *gorm.DB }

func NewGormRiskReviewRepository(db *gorm.DB) *GormRiskReviewRepository {
	return &GormRiskReviewRepository{db: db}
}

// ListDueForReview returns risks (across all tenants) whose review is due.
func (r *GormRiskReviewRepository) ListDueForReview(ctx context.Context, now time.Time) ([]domain.Risk, error) {
	var risks []domain.Risk
	err := r.db.WithContext(ctx).
		Where("review_interval_days > 0 AND next_review_at IS NOT NULL AND next_review_at <= ?", now).
		Limit(500).
		Find(&risks).Error
	return risks, err
}

// BumpNextReview pushes a risk's next review out by one cadence so the owner is
// nudged once per interval, not once per worker tick.
func (r *GormRiskReviewRepository) BumpNextReview(ctx context.Context, id uuid.UUID, next time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("id = ?", id).
		Update("next_review_at", next).Error
}
