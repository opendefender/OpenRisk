// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormVendorAssessmentRepository persists questionnaire templates, vendor
// assessments, their items and their public-link tokens (#670, ADR 0004 D3/D4).
//
// ABSOLUTE RULE: every query carries tenant_id — every TPRM table has its own
// column. The single exception is FindTokenByHash, which runs before any tenant
// is known BY NECESSITY: the tenant is what the token row resolves to. It is
// looked up by hash only, and every read after it uses the row's own tenant.
//
// Every state change a vendor or a sender can race is a CONDITIONAL update
// ("... AND status IN (open states)") inside a transaction, so two concurrent
// submissions, or a submission racing a revocation, produce exactly one winner
// rather than two writes that both passed a read.
type GormVendorAssessmentRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewGormVendorAssessmentRepository(db *gorm.DB) *GormVendorAssessmentRepository {
	return &GormVendorAssessmentRepository{db: db, now: time.Now}
}

// WithClock overrides the clock used to derive an expired status (tests).
func (r *GormVendorAssessmentRepository) WithClock(now func() time.Time) *GormVendorAssessmentRepository {
	r.now = now
	return r
}

var errVendorAssessmentRepoNoTenant = errors.New("vendor assessment repository: tenant_id is required")

var openAssessmentStatuses = []domain.VendorAssessmentStatus{
	domain.VendorAssessmentSent,
	domain.VendorAssessmentInProgress,
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (r *GormVendorAssessmentRepository) CreateTemplate(ctx context.Context, t *domain.VendorQuestionnaireTemplate) error {
	if t.TenantID == uuid.Nil {
		return errVendorAssessmentRepoNoTenant
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		return insertQuestions(tx, t)
	})
}

func insertQuestions(tx *gorm.DB, t *domain.VendorQuestionnaireTemplate) error {
	if len(t.Questions) == 0 {
		return nil
	}
	for i := range t.Questions {
		t.Questions[i].TenantID = t.TenantID
		t.Questions[i].TemplateID = t.ID
	}
	if err := tx.Create(&t.Questions).Error; err != nil {
		return fmt.Errorf("create template questions: %w", err)
	}
	return nil
}

func (r *GormVendorAssessmentRepository) GetTemplate(ctx context.Context, id, tenantID uuid.UUID) (*domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorAssessmentRepoNoTenant
	}
	var t domain.VendorQuestionnaireTemplate
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Where("template_id = ? AND tenant_id = ?", id, tenantID).
		Order("position ASC").
		Find(&t.Questions).Error; err != nil {
		return nil, fmt.Errorf("get template questions: %w", err)
	}
	return &t, nil
}

func (r *GormVendorAssessmentRepository) ListTemplates(ctx context.Context, tenantID uuid.UUID, includeArchived bool) ([]domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorAssessmentRepoNoTenant
	}
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if !includeArchived {
		// Chained onto q, which already filters by tenant_id on the line above.
		q = q.Where("archived_at IS NULL")
	}
	templates := []domain.VendorQuestionnaireTemplate{}
	if err := q.Order("name ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return templates, nil
}

// ReplaceTemplate saves the template's fields and replaces its questions with
// t.Questions, as one version. It answers false when the template is absent,
// foreign or archived: an archived template is frozen.
func (r *GormVendorAssessmentRepository) ReplaceTemplate(ctx context.Context, t *domain.VendorQuestionnaireTemplate) (bool, error) {
	if t.TenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.VendorQuestionnaireTemplate{}).
			Where("id = ? AND tenant_id = ? AND archived_at IS NULL", t.ID, t.TenantID).
			Updates(map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"language":    t.Language,
				"version":     t.Version,
				"updated_at":  t.UpdatedAt,
			})
		if res.Error != nil {
			return fmt.Errorf("update template: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Where("template_id = ? AND tenant_id = ?", t.ID, t.TenantID).
			Delete(&domain.VendorQuestionnaireQuestion{}).Error; err != nil {
			return fmt.Errorf("replace template questions: %w", err)
		}
		if err := insertQuestions(tx, t); err != nil {
			return err
		}
		updated = true
		return nil
	})
	return updated, err
}

func (r *GormVendorAssessmentRepository) ArchiveTemplate(ctx context.Context, id, tenantID uuid.UUID, at time.Time) (bool, error) {
	if tenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	res := r.db.WithContext(ctx).Model(&domain.VendorQuestionnaireTemplate{}).
		Where("id = ? AND tenant_id = ? AND archived_at IS NULL", id, tenantID).
		Updates(map[string]interface{}{"archived_at": at, "updated_at": at})
	if res.Error != nil {
		return false, fmt.Errorf("archive template: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// ---------------------------------------------------------------------------
// Assessments
// ---------------------------------------------------------------------------

// CreateAssessment writes the assessment, its snapshotted items and its first
// token in one transaction: a sent questionnaire with no items, or no working
// link, never exists.
func (r *GormVendorAssessmentRepository) CreateAssessment(ctx context.Context, a *domain.VendorAssessment, tok *domain.VendorAssessmentToken) error {
	if a.TenantID == uuid.Nil || tok == nil || tok.TenantID != a.TenantID {
		return errVendorAssessmentRepoNoTenant
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(a).Error; err != nil {
			return fmt.Errorf("create assessment: %w", err)
		}
		if len(a.Items) > 0 {
			if err := tx.Create(&a.Items).Error; err != nil {
				return fmt.Errorf("create assessment items: %w", err)
			}
		}
		if err := tx.Create(tok).Error; err != nil {
			return fmt.Errorf("create assessment token: %w", err)
		}
		return nil
	})
}

func (r *GormVendorAssessmentRepository) GetAssessment(ctx context.Context, id, tenantID uuid.UUID) (*domain.VendorAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorAssessmentRepoNoTenant
	}
	var a domain.VendorAssessment
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get assessment: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Where("assessment_id = ? AND tenant_id = ?", id, tenantID).
		Order("position ASC").
		Find(&a.Items).Error; err != nil {
		return nil, fmt.Errorf("get assessment items: %w", err)
	}
	return &a, nil
}

func (r *GormVendorAssessmentRepository) ListAssessmentsByVendor(ctx context.Context, tenantID, vendorID uuid.UUID) ([]domain.VendorAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorAssessmentRepoNoTenant
	}
	out := []domain.VendorAssessment{}
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_asset_id = ?", tenantID, vendorID).
		Order("sent_at DESC").
		Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	return out, nil
}

// RevokeAssessment revokes an OPEN assessment. False when it is absent, foreign,
// or already submitted or revoked.
func (r *GormVendorAssessmentRepository) RevokeAssessment(ctx context.Context, id, tenantID, by uuid.UUID, at time.Time) (bool, error) {
	if tenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	res := r.db.WithContext(ctx).Model(&domain.VendorAssessment{}).
		Where("id = ? AND tenant_id = ? AND status IN ?", id, tenantID, openAssessmentStatuses).
		Updates(map[string]interface{}{
			"status":     domain.VendorAssessmentRevoked,
			"revoked_at": at,
			"revoked_by": by,
			"updated_at": at,
		})
	if res.Error != nil {
		return false, fmt.Errorf("revoke assessment: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// IssueToken supersedes the assessment's active token and inserts tok, in one
// transaction, only while the assessment is still open. False when it is not.
// The partial unique index of migration 0062 is the backstop under concurrency.
func (r *GormVendorAssessmentRepository) IssueToken(ctx context.Context, tok *domain.VendorAssessmentToken) (bool, error) {
	if tok.TenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	issued := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.VendorAssessment{}).
			Where("id = ? AND tenant_id = ? AND status IN ?", tok.AssessmentID, tok.TenantID, openAssessmentStatuses).
			Update("updated_at", tok.IssuedAt)
		if res.Error != nil {
			return fmt.Errorf("check assessment for token: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&domain.VendorAssessmentToken{}).
			Where("assessment_id = ? AND tenant_id = ? AND superseded_at IS NULL", tok.AssessmentID, tok.TenantID).
			Update("superseded_at", tok.IssuedAt).Error; err != nil {
			return fmt.Errorf("supersede token: %w", err)
		}
		if err := tx.Create(tok).Error; err != nil {
			return fmt.Errorf("issue token: %w", err)
		}
		issued = true
		return nil
	})
	return issued, err
}

// FindTokenByHash resolves a presented token. Deliberately NOT tenant-scoped:
// the caller holds no session, and the tenant is what this row resolves to. The
// plaintext never reaches a query — only its hash does.
func (r *GormVendorAssessmentRepository) FindTokenByHash(ctx context.Context, hash string) (*domain.VendorAssessmentToken, error) {
	if len(hash) != 64 {
		return nil, nil
	}
	var tok domain.VendorAssessmentToken
	// Cross-tenant BY NECESSITY: no session exists yet, and the tenant is what this row resolves to (ADR 0004 D4).
	err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find token: %w", err)
	}
	return &tok, nil
}

func saveItemAnswers(tx *gorm.DB, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem) error {
	for _, it := range items {
		if err := tx.Model(&domain.VendorAssessmentItem{}).
			Where("id = ? AND assessment_id = ? AND tenant_id = ?", it.ID, assessmentID, tenantID).
			Updates(map[string]interface{}{
				"answer_value":   it.AnswerValue,
				"answer_na":      it.AnswerNA,
				"answer_comment": it.AnswerComment,
				"answered_at":    it.AnsweredAt,
			}).Error; err != nil {
			return fmt.Errorf("save answer: %w", err)
		}
	}
	return nil
}

// SaveAnswers writes the items' answers and moves the assessment to in_progress,
// only while it is open. False (and nothing written) when it is not.
func (r *GormVendorAssessmentRepository) SaveAnswers(ctx context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, at time.Time) (bool, error) {
	if tenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	saved := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.VendorAssessment{}).
			Where("id = ? AND tenant_id = ? AND status IN ?", assessmentID, tenantID, openAssessmentStatuses).
			Updates(map[string]interface{}{"status": domain.VendorAssessmentInProgress, "updated_at": at})
		if res.Error != nil {
			return fmt.Errorf("mark assessment in progress: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := saveItemAnswers(tx, tenantID, assessmentID, items); err != nil {
			return err
		}
		saved = true
		return nil
	})
	return saved, err
}

// SubmitAssessment writes the final answers and locks the assessment, only while
// it is open. The conditional update is what makes a second submission, or a
// submission racing a revocation, a clean false rather than a double write.
//
// The score (#671) is written by the same conditional update, so a submitted
// assessment always carries the score of exactly the answers it locked.
func (r *GormVendorAssessmentRepository) SubmitAssessment(ctx context.Context, tenantID, assessmentID uuid.UUID, items []domain.VendorAssessmentItem, provenance domain.JSONMap, scoring domain.VendorAssessmentScoring, at time.Time) (bool, error) {
	if tenantID == uuid.Nil {
		return false, errVendorAssessmentRepoNoTenant
	}
	submitted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.VendorAssessment{}).
			Where("id = ? AND tenant_id = ? AND status IN ?", assessmentID, tenantID, openAssessmentStatuses).
			Updates(map[string]interface{}{
				"status":          domain.VendorAssessmentSubmitted,
				"submitted_at":    at,
				"observed_at":     at,
				"provenance":      provenance,
				"score":           scoring.Score,
				"tier":            scoring.Tier,
				"score_breakdown": scoring.Breakdown,
				"scoring_version": scoring.Version,
				"updated_at":      at,
			})
		if res.Error != nil {
			return fmt.Errorf("submit assessment: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := saveItemAnswers(tx, tenantID, assessmentID, items); err != nil {
			return err
		}
		submitted = true
		return nil
	})
	return submitted, err
}

// LatestByVendor implements domain.VendorLatestAssessmentReader: the most
// recently sent assessment of each vendor, with its EFFECTIVE status (an open
// assessment past its grace period reads as expired).
func (r *GormVendorAssessmentRepository) LatestByVendor(ctx context.Context, tenantID uuid.UUID, vendorIDs []uuid.UUID) (map[uuid.UUID]domain.VendorLatestAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, errVendorAssessmentRepoNoTenant
	}
	out := make(map[uuid.UUID]domain.VendorLatestAssessment, len(vendorIDs))
	if len(vendorIDs) == 0 {
		return out, nil
	}
	var rows []domain.VendorAssessment
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_asset_id IN ?", tenantID, vendorIDs).
		Order("sent_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("latest assessments: %w", err)
	}
	now := r.now()
	for _, a := range rows {
		if _, seen := out[a.VendorAssetID]; seen {
			continue
		}
		a := a
		out[a.VendorAssetID] = domain.VendorLatestAssessment{
			ID:     a.ID,
			Status: string(a.State(now)),
			Score:  a.Score,
			Tier:   a.Tier,
			DueAt:  a.DueAt,
		}
	}
	return out, nil
}
