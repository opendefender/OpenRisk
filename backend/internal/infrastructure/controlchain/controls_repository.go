// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// The control library repository (#1414). It reads and writes the controls table
// and the addressed_by edges of canonical_edges. Every query names tenant_id (the
// tenant guard enforces it in tests). Reads re-resolve both ends of an edge, so a
// control or a requirement deleted after the link drops out of the answer: an edge
// has no deleted_at and is live when both its ends are (A10.3, D-112).

// listCap bounds the two link reads. A control addresses a handful of
// requirements; the bound only stops a runaway.
const listCap = 500

// AuditAppender appends an audit entry on the caller's transaction. Satisfied by
// repository.GormAuditChainRepository.
type AuditAppender interface {
	AppendTx(ctx context.Context, tx *gorm.DB, e *domain.AuditEvent) error
}

// ControlRepository stores the controls of the library and their requirement
// links. The audit entry of a write is appended in the same transaction: if the
// append fails the write is not stored.
type ControlRepository struct {
	db    *gorm.DB
	audit AuditAppender
}

// NewControlRepository builds the repository.
func NewControlRepository(db *gorm.DB, audit AuditAppender) *ControlRepository {
	return &ControlRepository{db: db, audit: audit}
}

func (r *ControlRepository) supportsRowLock() bool { return r.db.Dialector.Name() == "postgres" }

func controlNotFound() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlNotFound)
}

// RowToControl is the reverse of ControlToRow. The provenance read from the
// column is validated again, so a corrupt row is an error and not a half-read
// control.
func RowToControl(row ControlRow) (canonical.Control, error) {
	src := canonical.Source(row.Source)
	prov, err := canonical.ParseProvenance([]byte(row.Provenance), src)
	if err != nil {
		return canonical.Control{}, fmt.Errorf("controlchain: control %s provenance: %w", row.ID, err)
	}
	var tags canonical.List[string]
	if row.Tags != "" {
		if err := json.Unmarshal([]byte(row.Tags), &tags); err != nil {
			return canonical.Control{}, fmt.Errorf("controlchain: control %s tags: %w", row.ID, err)
		}
	}
	c := canonical.Control{
		Envelope: canonical.Envelope{
			ID: row.ID, TenantID: row.TenantID, Source: src, SourceID: row.SourceID, SourceVersion: row.SourceVersion,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ObservedAt: row.ObservedAt,
			Confidence: row.Confidence, Provenance: prov,
		},
		Name: row.Name, Description: row.Description, Objective: row.Objective,
		Ownership:                  canonical.Ownership{OwnerID: row.OwnerID, AssigneeID: row.AssigneeID, ReviewerID: row.ReviewerID},
		Applicability:              canonical.Applicability(row.Applicability),
		ApplicabilityJustification: row.ApplicabilityJustification,
		DeclaredStatus:             canonical.DeclaredStatus(row.DeclaredStatus),
		Tags:                       tags,
	}
	return canonical.WithApplicabilityReviewPending(c, row.ApplicabilityReviewPending), nil
}

func rowsToControls(rows []ControlRow) ([]canonical.Control, error) {
	out := make([]canonical.Control, len(rows))
	for i := range rows {
		c, err := RowToControl(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// CreateControl stores the control and its audit entry in one transaction.
func (r *ControlRepository) CreateControl(ctx context.Context, c canonical.Control, audit *domain.AuditEvent) error {
	if c.TenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	if audit == nil {
		return errors.New("controlchain.CreateControl: an audit entry is required")
	}
	row, err := ControlToRow(c, nil)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	if err != nil {
		return fmt.Errorf("controlchain.CreateControl: %w", err)
	}
	return nil
}

// GetControl reads one live control of the tenant.
func (r *ControlRepository) GetControl(ctx context.Context, tenantID, id uuid.UUID) (*canonical.Control, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var row ControlRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, controlNotFound()
		}
		return nil, fmt.Errorf("controlchain.GetControl: %w", err)
	}
	c, err := RowToControl(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListControls returns one page of the tenant's live controls, by name then id.
func (r *ControlRepository) ListControls(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]canonical.Control, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var rows []ControlRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("name ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListControls: %w", err)
	}
	return rowsToControls(rows)
}

// lockLive loads the live control under row lock where the engine has one.
func (r *ControlRepository) lockLive(tx *gorm.DB, tenantID, id uuid.UUID) (ControlRow, error) {
	q := tx.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id)
	if r.supportsRowLock() {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row ControlRow
	if err := q.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, controlNotFound()
		}
		return row, err
	}
	return row, nil
}

// UpdateControl loads the live control under lock (tenant_id and id), lets apply
// decide, then stores the result and the audit entry in one transaction. The
// UPDATE is a map, so a cleared field (false, nil) is written. A nil audit entry
// from apply means no change: nothing is written.
func (r *ControlRepository) UpdateControl(ctx context.Context, tenantID, id uuid.UUID,
	apply func(cur canonical.Control, evidenced func(asOf time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error),
) (*canonical.Control, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var out canonical.Control
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := r.lockLive(tx, tenantID, id)
		if err != nil {
			return err
		}
		cur, err := RowToControl(row)
		if err != nil {
			return err
		}
		next, ev, err := apply(cur, func(asOf time.Time) (bool, error) {
			return hasLiveEvidence(ctx, tx, tenantID, id, asOf)
		})
		if err != nil {
			return err
		}
		out = next
		if ev == nil {
			return nil
		}
		tags, err := json.Marshal(next.Tags)
		if err != nil {
			return err
		}
		res := tx.Model(&ControlRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{
				"name": next.Name, "description": next.Description, "objective": next.Objective,
				"owner_id": next.OwnerID, "assignee_id": next.AssigneeID, "reviewer_id": next.ReviewerID,
				"applicability": string(next.Applicability), "applicability_justification": next.ApplicabilityJustification,
				"applicability_review_pending": next.ApplicabilityReviewPending(),
				"declared_status":              string(next.DeclaredStatus), "tags": string(tags),
				"updated_at": next.UpdatedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return controlNotFound()
		}
		return r.audit.AppendTx(ctx, tx, ev)
	})
	if err != nil {
		return nil, wrapApp(err, "controlchain.UpdateControl")
	}
	return &out, nil
}

// DeleteControl soft-deletes the live control (UPDATE deleted_at, tenant_id in
// the WHERE) after apply accepted it, with the audit entry apply returned, in one
// transaction. Edges are kept: readers filter on the liveness of both ends.
func (r *ControlRepository) DeleteControl(ctx context.Context, tenantID, id uuid.UUID,
	apply func(cur canonical.Control) (*domain.AuditEvent, error),
) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := r.lockLive(tx, tenantID, id)
		if err != nil {
			return err
		}
		cur, err := RowToControl(row)
		if err != nil {
			return err
		}
		ev, err := apply(cur)
		if err != nil {
			return err
		}
		res := tx.Model(&ControlRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{"deleted_at": ev.CreatedAt, "updated_at": ev.CreatedAt})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return controlNotFound()
		}
		return r.audit.AppendTx(ctx, tx, ev)
	})
	return wrapApp(err, "controlchain.DeleteControl")
}

// LinkRequirement writes the addressed_by edge of the requirement to the control.
// Both ends go through the shared resolver with the tenant, in the transaction,
// before the insert: an end that is absent, deleted or another tenant's is
// ErrNotFound. The insert is idempotent on the edge key.
func (r *ControlRepository) LinkRequirement(ctx context.Context, tenantID uuid.UUID, edge canonical.Edge, audit *domain.AuditEvent) (bool, error) {
	if tenantID == uuid.Nil || edge.TenantID != tenantID || edge.To.TenantID != tenantID {
		return false, domain.ErrForbidden
	}
	if edge.Verb != canonical.VerbAddressedBy || edge.From.Type != canonical.TypeRequirement || edge.To.Type != canonical.TypeControl {
		return false, domain.NewValidationError("only an addressed_by edge from a requirement to a control is written here")
	}
	row, err := EdgeToRow(edge)
	if err != nil {
		return false, err
	}
	created := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := Resolve(ctx, tx, tenantID, canonical.TypeRequirement, edge.From.ID); err != nil {
			return err
		}
		if err := Resolve(ctx, tx, tenantID, canonical.TypeControl, edge.To.ID); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		created = true
		return r.audit.AppendTx(ctx, tx, audit)
	})
	if err != nil {
		return false, wrapApp(err, "controlchain.LinkRequirement")
	}
	return created, nil
}

// UnlinkRequirement hard-deletes the addressed_by edge (tenant_id in the WHERE).
// The control must be a live control of the tenant; no edge to delete is
// ErrNotFound.
func (r *ControlRepository) UnlinkRequirement(ctx context.Context, tenantID, controlID, requirementID uuid.UUID, audit *domain.AuditEvent) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := Resolve(ctx, tx, tenantID, canonical.TypeControl, controlID); err != nil {
			return err
		}
		res := tx.Where("tenant_id = ? AND from_type = ? AND from_id = ? AND verb = ? AND to_type = ? AND to_id = ? AND qualifier = ?",
			tenantID, string(canonical.TypeRequirement), requirementID, string(canonical.VerbAddressedBy),
			string(canonical.TypeControl), controlID, uuid.Nil).Delete(&CanonicalEdgeRow{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlLinkNotFound)
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	return wrapApp(err, "controlchain.UnlinkRequirement")
}

// ListRequirementsOfControl returns the live requirements the control addresses.
func (r *ControlRepository) ListRequirementsOfControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.InternalControlRequirement, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if err := Resolve(ctx, r.db, tenantID, canonical.TypeControl, controlID); err != nil {
		return nil, err
	}
	var out []domain.InternalControlRequirement
	err := r.db.WithContext(ctx).Table("canonical_edges e").
		Select("rc.id AS requirement_id, rc.framework_id AS framework_id, rc.reference_code AS reference_code, rc.name AS name, e.created_at AS linked_at").
		Joins("JOIN compliance_controls rc ON rc.id = e.from_id AND rc.tenant_id = e.tenant_id AND rc.deleted_at IS NULL").
		Where("e.tenant_id = ? AND e.verb = ? AND e.from_type = ? AND e.to_type = ? AND e.to_id = ?",
			tenantID, string(canonical.VerbAddressedBy), string(canonical.TypeRequirement), string(canonical.TypeControl), controlID).
		Order("rc.reference_code ASC, rc.id ASC").Limit(listCap).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListRequirementsOfControl: %w", err)
	}
	return out, nil
}

// ListControlsOfRequirement returns the live controls that address the
// requirement, the split control and the user-created ones alike.
func (r *ControlRepository) ListControlsOfRequirement(ctx context.Context, tenantID, requirementID uuid.UUID) ([]canonical.Control, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if err := Resolve(ctx, r.db, tenantID, canonical.TypeRequirement, requirementID); err != nil {
		return nil, err
	}
	var rows []ControlRow
	err := r.db.WithContext(ctx).Table(TableControls).Select("controls.*").
		Joins("JOIN canonical_edges e ON e.to_id = controls.id AND e.tenant_id = controls.tenant_id").
		Where("controls.tenant_id = ? AND controls.deleted_at IS NULL", tenantID).
		Where("e.tenant_id = ? AND e.verb = ? AND e.from_type = ? AND e.from_id = ? AND e.to_type = ?",
			tenantID, string(canonical.VerbAddressedBy), string(canonical.TypeRequirement), requirementID, string(canonical.TypeControl)).
		Order("controls.name ASC, controls.id ASC").Limit(listCap).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListControlsOfRequirement: %w", err)
	}
	return rowsToControls(rows)
}

// hasLiveEvidence reports whether the control reaches at least one evidence that
// is live (not deleted), accepted and not expired at asOf, through an
// evidenced_by edge from the control itself or from one of its live tests (a test
// of a live implementation of the control). Every table is read with the tenant.
func hasLiveEvidence(ctx context.Context, tx *gorm.DB, tenantID, controlID uuid.UUID, asOf time.Time) (bool, error) {
	var n int64
	err := tx.WithContext(ctx).Table("canonical_edges e").
		Joins("JOIN evidences ev ON ev.id = e.to_id AND ev.tenant_id = e.tenant_id AND ev.deleted_at IS NULL").
		Where("e.tenant_id = ? AND e.verb = ? AND e.to_type = ?", tenantID, string(canonical.VerbEvidencedBy), string(canonical.TypeEvidence)).
		Where("ev.tenant_id = ? AND ev.review = ? AND (ev.valid_until IS NULL OR ev.valid_until > ?)", tenantID, domain.EvidenceReviewAccepted, asOf).
		Where("((e.from_type = ? AND e.from_id = ?) OR (e.from_type = ? AND e.from_id IN ("+
			"SELECT t.id FROM control_tests t JOIN control_implementations i ON i.id = t.implementation_id AND i.tenant_id = t.tenant_id AND i.deleted_at IS NULL "+
			"WHERE t.tenant_id = ? AND t.deleted_at IS NULL AND i.control_id = ?)))",
			string(canonical.TypeControl), controlID, string(canonical.TypeControlTest), tenantID, controlID).
		Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("controlchain: control evidence: %w", err)
	}
	return n > 0, nil
}

// wrapApp returns typed application errors unchanged (so the central mapper
// answers them) and wraps anything else with the operation.
func wrapApp(err error, op string) error {
	if err == nil {
		return nil
	}
	var app *domain.AppError
	if errors.As(err, &app) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}
