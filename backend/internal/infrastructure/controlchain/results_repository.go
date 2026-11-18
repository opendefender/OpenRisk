// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
)

// The result repository (#1416, D-118). It reads and writes control_test_results,
// which is append-only: there is no update and no delete here, and no method that
// could grow one. The table carries the test's tenant_id (A10.5), so every query
// names it, and the parent chain (test, implementation, control) is resolved live
// in the tenant before any result is read or written. Evidence on a result is one
// canonical_edges row, evidenced_by from the test to the evidence, qualifier the
// result id.

// ResultRepository stores the results of a test. The audit entry of a write is
// appended in the same transaction.
type ResultRepository struct {
	db    *gorm.DB
	audit AuditAppender
}

// NewResultRepository builds the repository.
func NewResultRepository(db *gorm.DB, audit AuditAppender) *ResultRepository {
	return &ResultRepository{db: db, audit: audit}
}

var _ ctlapp.ResultRepository = (*ResultRepository)(nil)

func resultNotFound() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlResultNotFound)
}

// listResultEvidenceCap bounds the evidence list of one result.
const listResultEvidenceCap = 500

// ResultToRow is the persistence shape of a result of a test of the tenant.
func ResultToRow(tenantID, testID uuid.UUID, r canonical.ControlTestResult) ControlTestResultRow {
	return ControlTestResultRow{
		ID: r.ResultID, TenantID: tenantID, TestID: testID, Outcome: string(r.Outcome),
		ObservedAt: r.ObservedAt.UTC(), RecordedAt: r.RecordedAt.UTC(), Source: string(r.Source), SourceID: r.SourceID,
		Confidence: r.Confidence, ExecutedByKind: string(r.ExecutedBy.Kind), ExecutedByID: r.ExecutedBy.ID, Note: r.Note,
	}
}

// RowToResult is the reverse of ResultToRow.
func RowToResult(row ControlTestResultRow) canonical.ControlTestResult {
	return canonical.ControlTestResult{
		ResultID: row.ID, Outcome: canonical.Outcome(row.Outcome), ObservedAt: row.ObservedAt.UTC(), RecordedAt: row.RecordedAt.UTC(),
		Source: canonical.Source(row.Source), SourceID: row.SourceID, Confidence: row.Confidence,
		ExecutedBy: canonical.Actor{Kind: canonical.ActorKind(row.ExecutedByKind), ID: row.ExecutedByID}, Note: row.Note,
	}
}

// resultOfTest loads one result WHERE tenant_id = ? AND test_id = ? AND id = ?.
// A result of another tenant or of another test is the same ErrNotFound as an
// absent one.
func resultOfTest(ctx context.Context, tx *gorm.DB, tenantID, testID, resultID uuid.UUID) (ControlTestResultRow, error) {
	var row ControlTestResultRow
	err := tx.WithContext(ctx).Where("tenant_id = ? AND test_id = ? AND id = ?", tenantID, testID, resultID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, resultNotFound()
	}
	return row, err
}

// RecordResult stores the result and its audit entry in one transaction, after
// the test (and its implementation and control) resolved live in the tenant. The
// test row is locked on Postgres so a concurrent delete of its implementation,
// which cascades to the test, cannot leave a result under a deleted test.
func (r *ResultRepository) RecordResult(ctx context.Context, tenantID, testID uuid.UUID, res canonical.ControlTestResult, audit *domain.AuditEvent) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	if audit == nil {
		return errors.New("controlchain.RecordResult: an audit entry is required")
	}
	row := ResultToRow(tenantID, testID, res)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := liveTestRow(ctx, tx, tenantID, testID, true); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		// A new latest result can change the control's status (#555): mark it in
		// this transaction, so the commit and the invalidation are one.
		if err := controlstatus.MarkTest(ctx, tx, tenantID, testID); err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	return wrapApp(err, "controlchain.RecordResult")
}

// resultOrder is "last tested": when the test ran against the world, not when it
// was typed in (D-118 point 2). No DISTINCT ON, so the SQLite suite runs it too.
const resultOrder = "observed_at DESC, recorded_at DESC, id DESC"

// ListResults returns one page of the results of a live test of the tenant.
func (r *ResultRepository) ListResults(ctx context.Context, tenantID, testID uuid.UUID, limit, offset int) ([]canonical.ControlTestResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if _, err := liveTestRow(ctx, r.db, tenantID, testID, false); err != nil {
		return nil, wrapApp(err, "controlchain.ListResults")
	}
	var rows []ControlTestResultRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND test_id = ?", tenantID, testID).
		Order(resultOrder).Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListResults: %w", err)
	}
	out := make([]canonical.ControlTestResult, len(rows))
	for i := range rows {
		out[i] = RowToResult(rows[i])
	}
	return out, nil
}

// LatestResult returns the first result of the order above. The test is resolved
// first, so "never run" (control_test.no_result) is told from "no such test".
func (r *ResultRepository) LatestResult(ctx context.Context, tenantID, testID uuid.UUID) (*canonical.ControlTestResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if _, err := liveTestRow(ctx, r.db, tenantID, testID, false); err != nil {
		return nil, wrapApp(err, "controlchain.LatestResult")
	}
	var row ControlTestResultRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND test_id = ?", tenantID, testID).Order(resultOrder).Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.InternalControlError(domain.ErrNotFound, domain.IntCtlNoResult)
	}
	if err != nil {
		return nil, fmt.Errorf("controlchain.LatestResult: %w", err)
	}
	res := RowToResult(row)
	return &res, nil
}

// resultEvidenceEdges is the where clause of the edges that back one result.
func resultEvidenceEdges(ctx context.Context, tx *gorm.DB, tenantID, testID, resultID uuid.UUID) *gorm.DB {
	return tx.WithContext(ctx).Model(&CanonicalEdgeRow{}).
		Where("tenant_id = ? AND from_type = ? AND from_id = ? AND verb = ? AND to_type = ? AND qualifier = ?",
			tenantID, string(canonical.TypeControlTest), testID, string(canonical.VerbEvidencedBy), string(canonical.TypeEvidence), resultID)
}

func edgeRowToResultEvidence(row CanonicalEdgeRow) (ctlapp.ResultEvidenceView, error) {
	src := canonical.Source(row.Source)
	prov, err := canonical.ParseProvenance([]byte(row.Provenance), src)
	if err != nil {
		return ctlapp.ResultEvidenceView{}, fmt.Errorf("controlchain: edge %s provenance: %w", row.ID, err)
	}
	return ctlapp.ResultEvidenceView{
		ID: row.ID, TestID: row.FromID, ResultID: row.Qualifier, EvidenceID: row.ToID,
		Source: src, SourceID: row.SourceID, Confidence: row.Confidence, Provenance: prov, CreatedAt: row.CreatedAt,
	}, nil
}

// AttachEvidence resolves the test chain, the result of that test in the tenant
// and the evidence (`id = ? AND tenant_id = ? AND deleted_at IS NULL`), then
// inserts the edge (OnConflict DoNothing on D5's key). RowsAffected decides
// whether the audit entry is written: an existing pair returns the stored edge
// and audits nothing.
func (r *ResultRepository) AttachEvidence(ctx context.Context, tenantID, testID, resultID uuid.UUID, e canonical.Edge, audit *domain.AuditEvent) (*ctlapp.ResultEvidenceView, bool, error) {
	if tenantID == uuid.Nil || e.TenantID != tenantID || e.To.TenantID != tenantID {
		return nil, false, domain.ErrForbidden
	}
	if e.Verb != canonical.VerbEvidencedBy || e.From.Type != canonical.TypeControlTest || e.From.ID != testID ||
		e.To.Type != canonical.TypeEvidence || e.Qualifier == nil || *e.Qualifier != resultID {
		return nil, false, domain.ErrForbidden
	}
	if audit == nil {
		return nil, false, errors.New("controlchain.AttachEvidence: an audit entry is required")
	}
	var out *ctlapp.ResultEvidenceView
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := liveTestRow(ctx, tx, tenantID, testID, true); err != nil {
			return err
		}
		if _, err := resultOfTest(ctx, tx, tenantID, testID, resultID); err != nil {
			return err
		}
		if err := Resolve(ctx, tx, tenantID, canonical.TypeEvidence, e.To.ID); err != nil {
			return err
		}
		row, err := EdgeToRow(e)
		if err != nil {
			return err
		}
		res := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 1 {
			created = true
			// Evidence on the latest result can make a user-entered pass supported
			// (#555): mark the control in this transaction.
			if err := controlstatus.MarkTest(ctx, tx, tenantID, testID); err != nil {
				return err
			}
			if out, err = viewPtr(edgeRowToResultEvidence(row)); err != nil {
				return err
			}
			return r.audit.AppendTx(ctx, tx, audit)
		}
		var stored CanonicalEdgeRow
		if err := resultEvidenceEdges(ctx, tx, tenantID, testID, resultID).Where("to_id = ?", e.To.ID).Take(&stored).Error; err != nil {
			return err
		}
		out, err = viewPtr(edgeRowToResultEvidence(stored))
		return err
	})
	if err != nil {
		return nil, false, wrapApp(err, "controlchain.AttachEvidence")
	}
	return out, created, nil
}

func viewPtr(v ctlapp.ResultEvidenceView, err error) (*ctlapp.ResultEvidenceView, error) {
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListResultEvidence returns the evidence attached to a result, an edge being
// live when its evidence is (A10.3): the evidence is joined on the tenant and on
// deleted_at IS NULL.
func (r *ResultRepository) ListResultEvidence(ctx context.Context, tenantID, testID, resultID uuid.UUID) ([]ctlapp.ResultEvidenceView, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if _, err := liveTestRow(ctx, r.db, tenantID, testID, false); err != nil {
		return nil, wrapApp(err, "controlchain.ListResultEvidence")
	}
	if _, err := resultOfTest(ctx, r.db, tenantID, testID, resultID); err != nil {
		return nil, wrapApp(err, "controlchain.ListResultEvidence")
	}
	var rows []CanonicalEdgeRow
	err := r.db.WithContext(ctx).Table(TableCanonicalEdges+" e").
		Select("e.*").
		Joins("JOIN evidences ev ON ev.id = e.to_id AND ev.tenant_id = e.tenant_id AND ev.deleted_at IS NULL").
		Where("e.tenant_id = ? AND e.from_type = ? AND e.from_id = ? AND e.verb = ? AND e.to_type = ? AND e.qualifier = ?",
			tenantID, string(canonical.TypeControlTest), testID, string(canonical.VerbEvidencedBy), string(canonical.TypeEvidence), resultID).
		Where("ev.tenant_id = ?", tenantID).
		Order("e.created_at ASC, e.id ASC").Limit(listResultEvidenceCap).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListResultEvidence: %w", err)
	}
	out := make([]ctlapp.ResultEvidenceView, len(rows))
	for i := range rows {
		if out[i], err = edgeRowToResultEvidence(rows[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
