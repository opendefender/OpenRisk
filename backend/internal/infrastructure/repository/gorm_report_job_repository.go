// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormReportJobRepository persists report jobs, tenant-scoped on every query
// (RULE #2).
type GormReportJobRepository struct {
	db *gorm.DB
}

// NewGormReportJobRepository builds the repository.
func NewGormReportJobRepository(db *gorm.DB) *GormReportJobRepository {
	return &GormReportJobRepository{db: db}
}

func (r *GormReportJobRepository) Create(ctx context.Context, job *domain.ReportJob) error {
	if job.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(job).Error
}

// GetByID scopes by tenant so a job id guessed (or leaked) from another tenant
// reads as absent rather than as forbidden — the two are indistinguishable to a
// caller, and saying "not found" leaks nothing about what exists elsewhere.
func (r *GormReportJobRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.ReportJob, error) {
	var job domain.ReportJob
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NewNotFoundError("report job", id.String())
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// List returns the tenant's most recent jobs. Artifact is excluded: the list
// feeds a status table, and selecting every stored PDF to render it would move
// megabytes to describe kilobytes.
func (r *GormReportJobRepository) List(ctx context.Context, tenantID uuid.UUID, limit int) ([]domain.ReportJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	var jobs []domain.ReportJob
	err := r.db.WithContext(ctx).
		Select("id", "tenant_id", "kind", "status", "params", "title", "filename",
			"content_type", "size_bytes", "error", "requested_by",
			"created_at", "updated_at", "completed_at").
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

// Update writes the job back, scoped to its tenant so a mis-set TenantID cannot
// overwrite another tenant's row.
func (r *GormReportJobRepository) Update(ctx context.Context, job *domain.ReportJob) error {
	if job.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	res := r.db.WithContext(ctx).
		Model(&domain.ReportJob{}).
		Where("id = ? AND tenant_id = ?", job.ID, job.TenantID).
		Updates(map[string]interface{}{
			"status":       job.Status,
			"title":        job.Title,
			"filename":     job.Filename,
			"content_type": job.ContentType,
			"artifact":     job.Artifact,
			"size_bytes":   job.SizeBytes,
			"error":        job.Error,
			"completed_at": job.CompletedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("report job", job.ID.String())
	}
	return nil
}
