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
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormEvidenceRepository implements domain.EvidenceRepository.
//
// ABSOLUTE RULE #2: every query filters by tenant_id. The link table carries its
// own tenant_id as well as the two foreign keys, so a join that reaches through
// it cannot bridge two registers even if a caller passes the wrong id.
type GormEvidenceRepository struct {
	db *gorm.DB
}

func NewGormEvidenceRepository(db *gorm.DB) *GormEvidenceRepository {
	return &GormEvidenceRepository{db: db}
}

func (r *GormEvidenceRepository) Create(ctx context.Context, e *domain.Evidence) error {
	if e.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *GormEvidenceRepository) Update(ctx context.Context, e *domain.Evidence) error {
	if e.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	// Scoped by id AND tenant: an update is as much a cross-tenant risk as a read.
	res := r.db.WithContext(ctx).
		Model(&domain.Evidence{}).
		Where("id = ? AND tenant_id = ?", e.ID, e.TenantID).
		Select("title", "type", "description", "file_ref", "filename", "external_url",
			"collected_at", "valid_until", "review", "review_note", "reviewed_by",
			"reviewed_at", "source", "source_detail", "owner_id", "assignee_id",
			"reviewer_id", "updated_at").
		Updates(e)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("evidence", e.ID)
	}
	return nil
}

func (r *GormEvidenceRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error) {
	var ev domain.Evidence
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&ev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ev, nil
}

// List returns a filtered page of the library plus the total matching count (so
// the UI can paginate without a second round trip).
func (r *GormEvidenceRepository) List(ctx context.Context, tenantID uuid.UUID, f domain.EvidenceFilter) ([]domain.Evidence, int64, error) {
	q := r.db.WithContext(ctx).Model(&domain.Evidence{}).Where("evidences.tenant_id = ?", tenantID)

	if f.Type != "" {
		q = q.Where("evidences.type = ?", f.Type)
	}
	if f.Review != "" {
		q = q.Where("evidences.review = ?", f.Review)
	}
	if f.ExpiringBefore != nil {
		q = q.Where("evidences.valid_until IS NOT NULL AND evidences.valid_until < ?", *f.ExpiringBefore)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("evidences.title ILIKE ? OR evidences.description ILIKE ? OR evidences.filename ILIKE ?", like, like, like)
	}
	// Narrowing by control or framework goes through the link table. A subquery
	// rather than a join keeps the row set free of duplicates when one artifact
	// answers several controls of the same framework — which is the normal case,
	// and would otherwise make the library appear to hold the same file twice.
	if f.ControlID != nil {
		q = q.Where("EXISTS (SELECT 1 FROM evidence_control_links l WHERE l.evidence_id = evidences.id AND l.tenant_id = evidences.tenant_id AND l.control_id = ?)", *f.ControlID)
	}
	if f.FrameworkID != nil {
		q = q.Where(`EXISTS (
			SELECT 1 FROM evidence_control_links l
			JOIN compliance_controls c ON c.id = l.control_id AND c.tenant_id = l.tenant_id
			WHERE l.evidence_id = evidences.id AND l.tenant_id = evidences.tenant_id
			  AND c.framework_id = ? AND c.deleted_at IS NULL)`, *f.FrameworkID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Newest collection first: an evidence library is read to answer "what do we
	// have now", not "what did we have first".
	q = q.Order("evidences.collected_at DESC, evidences.created_at DESC")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}

	var out []domain.Evidence
	if err := q.Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Delete soft-deletes the artifact and hard-deletes its links.
//
// The links go for real because a soft-deleted link is a link that still has to
// be filtered out of every coverage query forever; the artifact is what carries
// history, and it is still there.
func (r *GormEvidenceRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&domain.Evidence{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.NewNotFoundError("evidence", id)
		}
		return tx.Where("evidence_id = ? AND tenant_id = ?", id, tenantID).
			Delete(&domain.EvidenceControlLink{}).Error
	})
}

// Link attaches an artifact to a control, idempotently.
//
// ON CONFLICT DO NOTHING against the (evidence_id, control_id) unique index: the
// UI's "attach to control" is a button people press twice, and the second press
// must mean the same thing as the first, not create a phantom second proof that
// inflates every coverage count.
func (r *GormEvidenceRepository) Link(ctx context.Context, link *domain.EvidenceControlLink) error {
	if link.TenantID == uuid.Nil {
		return domain.NewValidationError("tenant_id is required")
	}
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "evidence_id"}, {Name: "control_id"}}, DoNothing: true}).
		Create(link).Error
}

func (r *GormEvidenceRepository) Unlink(ctx context.Context, tenantID, evidenceID, controlID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND evidence_id = ? AND control_id = ?", tenantID, evidenceID, controlID).
		Delete(&domain.EvidenceControlLink{}).Error
}

func (r *GormEvidenceRepository) ListLinks(ctx context.Context, tenantID uuid.UUID, evidenceIDs []uuid.UUID) ([]domain.EvidenceControlLink, error) {
	if len(evidenceIDs) == 0 {
		return nil, nil
	}
	var out []domain.EvidenceControlLink
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND evidence_id IN ?", tenantID, evidenceIDs).
		Find(&out).Error
	return out, err
}

func (r *GormEvidenceRepository) ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.Evidence, error) {
	var out []domain.Evidence
	err := r.db.WithContext(ctx).
		Joins("JOIN evidence_control_links l ON l.evidence_id = evidences.id AND l.tenant_id = evidences.tenant_id").
		Where("evidences.tenant_id = ? AND l.control_id = ?", tenantID, controlID).
		Order("evidences.collected_at DESC").
		Find(&out).Error
	return out, err
}

// coveringPredicate is the SQL twin of domain.Evidence.Covers.
//
// It exists because counting coverage row-by-row in Go would mean loading every
// artifact of every framework; it is kept next to its Go counterpart in spirit by
// this comment and pinned by a repository test that runs the same fixtures
// through both. If the two ever disagree, the product reports coverage it cannot
// substantiate.
const coveringPredicate = `e.review = 'accepted' AND (e.valid_until IS NULL OR e.valid_until > ?)`

func (r *GormEvidenceRepository) CountCoveringByFramework(ctx context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]int, error) {
	type row struct {
		ControlID uuid.UUID
		N         int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("evidence_control_links l").
		Select("l.control_id AS control_id, COUNT(*) AS n").
		Joins("JOIN evidences e ON e.id = l.evidence_id AND e.tenant_id = l.tenant_id AND e.deleted_at IS NULL").
		Joins("JOIN compliance_controls c ON c.id = l.control_id AND c.tenant_id = l.tenant_id AND c.deleted_at IS NULL").
		Where("l.tenant_id = ? AND c.framework_id = ?", tenantID, frameworkID).
		Where(coveringPredicate, now).
		Group("l.control_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.ControlID] = r.N
	}
	return out, nil
}

func (r *GormEvidenceRepository) ControlsWithCoverage(ctx context.Context, tenantID, frameworkID uuid.UUID, now time.Time) (map[uuid.UUID]bool, error) {
	counts, err := r.CountCoveringByFramework(ctx, tenantID, frameworkID, now)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]bool, len(counts))
	for id, n := range counts {
		if n > 0 {
			out[id] = true
		}
	}
	return out, nil
}

// ListExpiring returns artifacts falling due inside the window that have not been
// reminded since they last changed.
//
// Cross-tenant by design — it feeds a background worker that sweeps every tenant,
// the same shape as the mitigation due worker. The tenant scoping happens where
// the reminder is addressed, not here.
func (r *GormEvidenceRepository) ListExpiring(ctx context.Context, now time.Time, window time.Duration, limit int) ([]domain.Evidence, error) {
	if limit <= 0 {
		limit = 500
	}
	var out []domain.Evidence
	err := r.db.WithContext(ctx).
		Where("valid_until IS NOT NULL AND valid_until <= ?", now.Add(window)).
		Where("review = ?", domain.EvidenceReviewAccepted).
		// "Not reminded since the artifact last changed" rather than "never
		// reminded": re-dating an expiry (the whole point of renewing a proof)
		// bumps updated_at and legitimately re-arms the reminder.
		Where("reminder_sent_at IS NULL OR reminder_sent_at < updated_at").
		Order("valid_until ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

func (r *GormEvidenceRepository) MarkReminded(ctx context.Context, id uuid.UUID, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.Evidence{}).
		Where("id = ?", id).
		UpdateColumn("reminder_sent_at", at).Error
}
