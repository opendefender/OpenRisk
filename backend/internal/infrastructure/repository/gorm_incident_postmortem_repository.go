// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormIncidentPostMortemRepository stores incident reviews. Tenant-scoped on
// every query (RULE #2): incident ids are sequential integers, so a review keyed
// only by incident id would be readable by anyone who can count.
type GormIncidentPostMortemRepository struct{ db *gorm.DB }

// NewGormIncidentPostMortemRepository builds the store.
func NewGormIncidentPostMortemRepository(db *gorm.DB) *GormIncidentPostMortemRepository {
	return &GormIncidentPostMortemRepository{db: db}
}

var _ domain.IncidentPostMortemRepository = (*GormIncidentPostMortemRepository)(nil)

// Get returns the review for an incident, or nil when none was written yet.
func (r *GormIncidentPostMortemRepository) Get(ctx context.Context, tenantID uuid.UUID, incidentID uint) (*domain.IncidentPostMortem, error) {
	var pm domain.IncidentPostMortem
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND incident_id = ?", tenantID, incidentID).
		Take(&pm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get post-mortem: %w", err)
	}
	return &pm, nil
}

// Upsert creates or updates the review (one per incident).
func (r *GormIncidentPostMortemRepository) Upsert(ctx context.Context, p *domain.IncidentPostMortem) error {
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	existing, err := r.Get(ctx, p.TenantID, p.IncidentID)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.db.WithContext(ctx).Create(p).Error
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	res := r.db.WithContext(ctx).
		Model(&domain.IncidentPostMortem{}).
		Where("id = ? AND tenant_id = ?", existing.ID, p.TenantID).
		Updates(map[string]interface{}{
			"summary":              p.Summary,
			"root_cause":           p.RootCause,
			"contributing_factors": p.ContributingFactors,
			"impact":               p.Impact,
			"detection":            p.Detection,
			"what_went_well":       p.WhatWentWell,
			"lessons_learned":      p.LessonsLearned,
			"timeline":             p.Timeline,
			"corrective_actions":   p.CorrectiveActions,
			"status":               p.Status,
			"published_at":         p.PublishedAt,
			"published_by":         p.PublishedBy,
		})
	if res.Error != nil {
		return fmt.Errorf("failed to update post-mortem: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("post-mortem", existing.ID)
	}
	return nil
}

// ListByTenant returns the tenant's reviews, newest first.
func (r *GormIncidentPostMortemRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, status string, limit int) ([]domain.IncidentPostMortem, error) {
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []domain.IncidentPostMortem
	if err := q.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("failed to list post-mortems: %w", err)
	}
	return out, nil
}
