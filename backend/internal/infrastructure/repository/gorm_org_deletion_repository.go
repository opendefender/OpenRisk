// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormOrgDeletionRepository persists danger-zone org-deletion requests.
type GormOrgDeletionRepository struct {
	db *gorm.DB
}

func NewGormOrgDeletionRepository(db *gorm.DB) *GormOrgDeletionRepository {
	return &GormOrgDeletionRepository{db: db}
}

// GetActive returns the pending request for a tenant, or (nil, nil).
func (r *GormOrgDeletionRepository) GetActive(ctx context.Context, tenant uuid.UUID) (*domain.OrgDeletionRequest, error) {
	var req domain.OrgDeletionRequest
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND status = ?", tenant, domain.DeletionPending).
		First(&req).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *GormOrgDeletionRepository) Create(ctx context.Context, req *domain.OrgDeletionRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

// Cancel marks the tenant's pending request canceled.
func (r *GormOrgDeletionRepository) Cancel(ctx context.Context, tenant, by uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.OrgDeletionRequest{}).
		Where("organization_id = ? AND status = ?", tenant, domain.DeletionPending).
		Updates(map[string]interface{}{
			"status":      domain.DeletionCanceled,
			"canceled_by": by,
			"canceled_at": at,
		}).Error
}

// ListDue returns pending requests whose grace window has elapsed. Cross-tenant by
// design — the purge worker sweeps every tenant.
func (r *GormOrgDeletionRepository) ListDue(ctx context.Context, now time.Time) ([]domain.OrgDeletionRequest, error) {
	var out []domain.OrgDeletionRequest
	err := r.db.WithContext(ctx).
		Where("status = ? AND scheduled_purge_at <= ?", domain.DeletionPending, now).
		Find(&out).Error
	return out, err
}

func (r *GormOrgDeletionRepository) MarkCompleted(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).Model(&domain.OrgDeletionRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"status": domain.DeletionCompleted, "completed_at": at}).Error
}

// Name returns the organization's exact name (satisfies orgdeletion.OrgReader).
func (r *GormOrgDeletionRepository) Name(ctx context.Context, tenant uuid.UUID) (string, error) {
	var org domain.Organization
	if err := r.db.WithContext(ctx).Select("name").Where("id = ?", tenant).First(&org).Error; err != nil {
		return "", err
	}
	return org.Name, nil
}
