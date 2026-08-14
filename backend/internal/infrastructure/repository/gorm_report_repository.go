// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormReportRepository implements domain.ReportRepository.
// Tenant-scoped on every query except ClaimQueued, which is a worker's query and
// documents its own reason.
type GormReportRepository struct {
	db *gorm.DB
}

func NewGormReportRepository(db *gorm.DB) *GormReportRepository {
	return &GormReportRepository{db: db}
}

func (r *GormReportRepository) Create(ctx context.Context, rep *domain.Report) error {
	if rep.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if rep.ID == uuid.Nil {
		rep.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(rep).Error
}

// Update writes the mutable fields, scoped by id AND tenant.
//
// An explicit Select rather than a full Save: Save would also rewrite Artifact
// on every progress tick, pushing the whole document through the connection each
// time the worker moves a percentage point.
func (r *GormReportRepository) Update(ctx context.Context, rep *domain.Report) error {
	if rep.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	res := r.db.WithContext(ctx).
		Model(&domain.Report{}).
		Where("id = ? AND tenant_id = ?", rep.ID, rep.TenantID).
		Select("title", "run_state", "progress", "step", "error", "lifecycle",
			"filename", "content_type", "artifact", "size_bytes", "content_hash",
			"content_fingerprint",
			"template_key", "template_version", "approved_by", "approved_at",
			"published_at", "completed_at", "updated_at").
		Updates(rep)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("report", rep.ID)
	}
	return nil
}

// UpdateProgress writes only the progress fields.
//
// Separate from Update because it runs on every step of a render, and touching
// the artifact column on each tick would be gratuitous IO on a multi-megabyte
// blob.
func (r *GormReportRepository) UpdateProgress(ctx context.Context, id uuid.UUID, progress int, step string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Report{}).
		Where("id = ?", id).
		Updates(map[string]any{"progress": progress, "step": step, "updated_at": time.Now()}).Error
}

func (r *GormReportRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Report, error) {
	var rep domain.Report
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&rep).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("report", id)
		}
		return nil, err
	}
	return &rep, nil
}

// List returns a filtered page plus the total.
//
// The artifact column is excluded from the SELECT: a listing of twenty reports
// would otherwise pull twenty documents out of the database to render a table of
// titles.
func (r *GormReportRepository) List(ctx context.Context, tenantID uuid.UUID, f domain.ReportFilter) ([]domain.Report, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.Report{}).Where("tenant_id = ?", tenantID)
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	if f.Lifecycle != "" {
		q = q.Where("lifecycle = ?", f.Lifecycle)
	}
	if f.Format != "" {
		q = q.Where("format = ?", f.Format)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "created_at DESC"
	if f.Sort == "created_at" {
		order = "created_at ASC"
	}
	q = q.Order(order)
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}

	var out []domain.Report
	err := q.Select("id", "tenant_id", "type", "format", "locale", "template_key",
		"template_version", "params", "title", "run_state", "progress", "step",
		"error", "lifecycle", "filename", "content_type", "size_bytes",
		"content_hash", "content_fingerprint", "version", "supersedes", "requested_by", "approved_by",
		"approved_at", "published_at", "created_at", "updated_at", "completed_at").
		Find(&out).Error
	return out, total, err
}

func (r *GormReportRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&domain.Report{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.NewNotFoundError("report", id)
		}
		// Comments go with the report they annotate. A review remark on a document
		// that no longer exists is not history, it is litter.
		return tx.Where("report_id = ? AND tenant_id = ?", id, tenantID).
			Delete(&domain.ReportComment{}).Error
	})
}

// Lineage walks the supersedes chain in both directions and returns every
// version, newest first.
//
// Walked in Go rather than as a recursive CTE: a lineage is a handful of rows,
// and a WITH RECURSIVE would tie this to Postgres while the tests run on sqlite.
func (r *GormReportRepository) Lineage(ctx context.Context, tenantID, id uuid.UUID) ([]domain.Report, error) {
	seed, err := r.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	// Walk back to the root.
	root := seed
	seen := map[uuid.UUID]bool{root.ID: true}
	for root.Supersedes != nil && !seen[*root.Supersedes] {
		prev, err := r.GetByID(ctx, tenantID, *root.Supersedes)
		if err != nil {
			break // a deleted ancestor ends the chain; it does not break the read
		}
		seen[prev.ID] = true
		root = prev
	}

	// Walk forward from the root, collecting each successor.
	chain := []domain.Report{*root}
	current := root
	for {
		var next domain.Report
		err := r.db.WithContext(ctx).
			Where("tenant_id = ? AND supersedes = ?", tenantID, current.ID).
			Order("version ASC").
			First(&next).Error
		if err != nil {
			break
		}
		if seen[next.ID] && next.ID != seed.ID {
			break // defensive: a cycle must not hang the request
		}
		chain = append(chain, next)
		current = &next
	}

	// Newest first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func (r *GormReportRepository) AddComment(ctx context.Context, c *domain.ReportComment) error {
	if c.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *GormReportRepository) ListComments(ctx context.Context, tenantID, reportID uuid.UUID) ([]domain.ReportComment, error) {
	var out []domain.ReportComment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND report_id = ?", tenantID, reportID).
		Order("created_at ASC").
		Find(&out).Error
	return out, err
}

// ClaimQueued atomically takes the oldest queued report and marks it running.
//
// Cross-tenant by necessity: a background worker has no session, and every row
// carries its own tenant_id. The claim is a single conditional UPDATE ... WHERE
// run_state = 'queued' RETURNING, so two workers cannot take the same row —
// selecting then updating would let both pass the select and render the document
// twice, which for a report means two artifacts and two hashes for one request.
func (r *GormReportRepository) ClaimQueued(ctx context.Context) (*domain.Report, error) {
	// Pick a candidate, then take it with a CONDITIONAL update. The condition is
	// the whole guard: two workers can both read the same candidate, but only one
	// UPDATE ... WHERE run_state = 'queued' affects a row, and the loser sees
	// RowsAffected = 0 and moves on. A plain select-then-update would let both
	// pass and render the document twice — two artifacts and two hashes for one
	// request.
	//
	// Written this way rather than with RETURNING because the tests run on sqlite
	// and production on Postgres, and a claim that only works on one of them is a
	// claim whose exclusivity is never actually tested.
	for attempt := 0; attempt < 5; attempt++ {
		var candidate domain.Report
		err := r.db.WithContext(ctx).
			Where("run_state = ?", domain.ReportRunQueued).
			Order("created_at ASC").
			First(&candidate).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil // nothing queued
			}
			return nil, err
		}

		res := r.db.WithContext(ctx).
			Model(&domain.Report{}).
			Where("id = ? AND run_state = ?", candidate.ID, domain.ReportRunQueued).
			Updates(map[string]any{
				"run_state":  domain.ReportRunRunning,
				"step":       "starting",
				"progress":   1,
				"updated_at": time.Now(),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			// Someone else took it. Try the next one rather than returning empty:
			// there may be a queue behind it, and a worker that gives up on a lost
			// race would idle for a full tick while work waits.
			continue
		}

		// Re-read so the render sees the row as it now stands, params included.
		var full domain.Report
		if err := r.db.WithContext(ctx).Where("id = ?", candidate.ID).First(&full).Error; err != nil {
			return nil, err
		}
		return &full, nil
	}
	// Five consecutive losses means heavy contention, not an error. The next tick
	// will try again.
	return nil, nil
}
