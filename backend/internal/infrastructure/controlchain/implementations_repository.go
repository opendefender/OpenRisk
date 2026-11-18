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
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
)

// The implementation repository (#1415). It reads and writes control_implementations
// and the applies_to edges of canonical_edges. Every query names tenant_id. The
// parent chain is resolved live in the tenant on every read and write (an
// implementation of a deleted control reads as absent), and an applies_to target
// is resolved in the tenant before its edge is inserted (Resolve, the S2 gate), so
// another tenant's asset id is ErrNotFound and nothing is written.

// ImplementationRepository stores the implementations of a control. The audit
// entry of a write is appended in the same transaction.
type ImplementationRepository struct {
	db    *gorm.DB
	audit AuditAppender
}

// NewImplementationRepository builds the repository.
func NewImplementationRepository(db *gorm.DB, audit AuditAppender) *ImplementationRepository {
	return &ImplementationRepository{db: db, audit: audit}
}

var _ ctlapp.ImplementationRepository = (*ImplementationRepository)(nil)

func implementationNotFound() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlImplementationNotFound)
}

const dateLayout = "2006-01-02"

// ImplementationToRow is the persistence shape of a canonical ControlImplementation.
func ImplementationToRow(ci canonical.ControlImplementation, deletedAt *time.Time) (ControlImplementationRow, error) {
	prov, err := json.Marshal(ci.Provenance)
	if err != nil {
		return ControlImplementationRow{}, fmt.Errorf("controlchain: provenance: %w", err)
	}
	return ControlImplementationRow{
		ID: ci.ID, TenantID: ci.TenantID, Source: string(ci.Source), SourceID: ci.SourceID,
		SourceVersion: ci.SourceVersion, Confidence: ci.Confidence, Provenance: string(prov),
		ObservedAt: ci.ObservedAt, CreatedAt: ci.CreatedAt, UpdatedAt: ci.UpdatedAt, DeletedAt: deletedAt,
		ControlID: ci.ControlID, Description: ci.Description, Status: string(ci.Status),
		InPlaceSince: dateToTime(ci.InPlaceSince),
		OwnerID:      ci.OwnerID, AssigneeID: ci.AssigneeID, ReviewerID: ci.ReviewerID,
	}, nil
}

func dateToTime(d *canonical.Date) *time.Time {
	if d == nil {
		return nil
	}
	t, err := time.Parse(dateLayout, string(*d))
	if err != nil {
		return nil // Validate refused it before the repository was reached
	}
	return &t
}

func timeToDate(t *time.Time) *canonical.Date {
	if t == nil {
		return nil
	}
	d := canonical.Date(t.UTC().Format(dateLayout))
	return &d
}

// RowToImplementation is the reverse of ImplementationToRow. The provenance read
// from the column is validated again, so a corrupt row is an error.
func RowToImplementation(row ControlImplementationRow) (canonical.ControlImplementation, error) {
	src := canonical.Source(row.Source)
	prov, err := canonical.ParseProvenance([]byte(row.Provenance), src)
	if err != nil {
		return canonical.ControlImplementation{}, fmt.Errorf("controlchain: implementation %s provenance: %w", row.ID, err)
	}
	return canonical.ControlImplementation{
		Envelope: canonical.Envelope{
			ID: row.ID, TenantID: row.TenantID, Source: src, SourceID: row.SourceID, SourceVersion: row.SourceVersion,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ObservedAt: row.ObservedAt,
			Confidence: row.Confidence, Provenance: prov,
		},
		ControlID: row.ControlID, Description: row.Description, Status: canonical.ImplementationStatus(row.Status),
		InPlaceSince: timeToDate(row.InPlaceSince),
		Ownership:    canonical.Ownership{OwnerID: row.OwnerID, AssigneeID: row.AssigneeID, ReviewerID: row.ReviewerID},
	}, nil
}

// liveImplementationRow loads the live implementation of the tenant (under row
// lock when lock is set and the engine has one) and resolves its control live in
// the same tenant. An absent, deleted or foreign implementation, and one whose
// control is deleted, is the same ErrNotFound.
func liveImplementationRow(ctx context.Context, tx *gorm.DB, tenantID, id uuid.UUID, lock bool) (ControlImplementationRow, error) {
	q := tx.WithContext(ctx).Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id)
	if lock && tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row ControlImplementationRow
	if err := q.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, implementationNotFound()
		}
		return row, err
	}
	if err := Resolve(ctx, tx, tenantID, canonical.TypeControl, row.ControlID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return row, implementationNotFound()
		}
		return row, err
	}
	return row, nil
}

// appliesToAssets returns, per implementation, the live assets of the tenant it
// applies to. An edge is live when both its ends are (A10.3), so an asset deleted
// after the link drops out. The edges and the assets are read with the tenant.
func appliesToAssets(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, implIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := make(map[uuid.UUID][]uuid.UUID, len(implIDs))
	if len(implIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ImplementationID uuid.UUID
		AssetID          uuid.UUID
	}
	err := tx.WithContext(ctx).Table("canonical_edges e").
		Select("e.from_id AS implementation_id, a.id AS asset_id").
		Joins("JOIN assets a ON a.id = e.to_id AND a.tenant_id = e.tenant_id AND a.deleted_at IS NULL").
		Where("e.tenant_id = ? AND e.verb = ? AND e.from_type = ? AND e.to_type = ? AND e.from_id IN ?",
			tenantID, string(canonical.VerbAppliesTo), string(canonical.TypeControlImplementation), string(canonical.TypeAsset), implIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain: implementation targets: %w", err)
	}
	for _, r := range rows {
		out[r.ImplementationID] = append(out[r.ImplementationID], r.AssetID)
	}
	for k := range out {
		ids := out[k]
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	}
	return out, nil
}

// rawTargets returns the asset ids of every applies_to edge of the implementation,
// live asset or not, read straight from canonical_edges with the tenant.
func rawTargets(ctx context.Context, tx *gorm.DB, tenantID, implID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.WithContext(ctx).Model(&CanonicalEdgeRow{}).
		Where("tenant_id = ? AND from_type = ? AND from_id = ? AND verb = ? AND to_type = ?",
			tenantID, string(canonical.TypeControlImplementation), implID, string(canonical.VerbAppliesTo), string(canonical.TypeAsset)).
		Pluck("to_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain: implementation edges: %w", err)
	}
	return ids, nil
}

func (r *ImplementationRepository) record(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, row ControlImplementationRow) (ctlapp.ImplementationRecord, error) {
	ci, err := RowToImplementation(row)
	if err != nil {
		return ctlapp.ImplementationRecord{}, err
	}
	targets, err := appliesToAssets(ctx, tx, tenantID, []uuid.UUID{row.ID})
	if err != nil {
		return ctlapp.ImplementationRecord{}, err
	}
	return ctlapp.ImplementationRecord{Implementation: ci, AppliesTo: targets[row.ID]}, nil
}

// insertAppliesTo resolves every asset in the tenant, then inserts the edges
// (idempotent on the edge key). The edges' tenant must be the caller's.
func insertAppliesTo(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, edges []canonical.Edge) error {
	for _, e := range edges {
		if e.TenantID != tenantID || e.To.TenantID != tenantID ||
			e.Verb != canonical.VerbAppliesTo || e.From.Type != canonical.TypeControlImplementation || e.To.Type != canonical.TypeAsset {
			return domain.ErrForbidden
		}
		if err := Resolve(ctx, tx, tenantID, canonical.TypeAsset, e.To.ID); err != nil {
			return err
		}
		row, err := EdgeToRow(e)
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// CreateImplementation stores the implementation, its applies_to edges and the
// audit entry in one transaction, after the control and every asset resolved in
// the tenant.
func (r *ImplementationRepository) CreateImplementation(ctx context.Context, tenantID uuid.UUID, impl canonical.ControlImplementation, edges []canonical.Edge, audit *domain.AuditEvent) error {
	if tenantID == uuid.Nil || impl.TenantID != tenantID {
		return domain.ErrForbidden
	}
	if audit == nil {
		return errors.New("controlchain.CreateImplementation: an audit entry is required")
	}
	row, err := ImplementationToRow(impl, nil)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the control (Postgres) so a concurrent delete of it cannot leave this
		// implementation orphaned.
		q := tx.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, impl.ControlID)
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var ctl ControlRow
		if err := q.Take(&ctl).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return controlNotFound()
			}
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := insertAppliesTo(ctx, tx, tenantID, edges); err != nil {
			return err
		}
		// A new implementation changes which tests are in scope (#555).
		if err := controlstatus.MarkControls(ctx, tx, tenantID, impl.ControlID); err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	return wrapApp(err, "controlchain.CreateImplementation")
}

// GetImplementation reads one live implementation of the tenant and its live targets.
func (r *ImplementationRepository) GetImplementation(ctx context.Context, tenantID, id uuid.UUID) (*ctlapp.ImplementationRecord, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	row, err := liveImplementationRow(ctx, r.db, tenantID, id, false)
	if err != nil {
		return nil, wrapApp(err, "controlchain.GetImplementation")
	}
	rec, err := r.record(ctx, r.db, tenantID, row)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListImplementations returns one page of the live implementations of a live
// control of the tenant, with their targets read in one query.
func (r *ImplementationRepository) ListImplementations(ctx context.Context, tenantID, controlID uuid.UUID, limit, offset int) ([]ctlapp.ImplementationRecord, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if err := Resolve(ctx, r.db, tenantID, canonical.TypeControl, controlID); err != nil {
		return nil, wrapApp(err, "controlchain.ListImplementations")
	}
	var rows []ControlImplementationRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND control_id = ? AND deleted_at IS NULL", tenantID, controlID).
		Order("created_at ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListImplementations: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	targets, err := appliesToAssets(ctx, r.db, tenantID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ctlapp.ImplementationRecord, len(rows))
	for i := range rows {
		ci, err := RowToImplementation(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = ctlapp.ImplementationRecord{Implementation: ci, AppliesTo: targets[rows[i].ID]}
	}
	return out, nil
}

// UpdateImplementation loads the live implementation under lock (tenant_id and
// id), lets apply decide, then stores the row, deletes the removed applies_to
// edges (tenant_id in the WHERE), inserts the added ones after resolving their
// assets in the tenant, and appends the audit entry, in one transaction. The
// UPDATE is a map, so a cleared field (nil) is written. A nil entry from apply
// means no change: nothing is written.
func (r *ImplementationRepository) UpdateImplementation(ctx context.Context, tenantID, id uuid.UUID,
	apply func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error),
) (*ctlapp.ImplementationRecord, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var out ctlapp.ImplementationRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := liveImplementationRow(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		cur, err := r.record(ctx, tx, tenantID, row)
		if err != nil {
			return err
		}
		raw, err := rawTargets(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		live := make(map[uuid.UUID]bool, len(cur.AppliesTo))
		for _, a := range cur.AppliesTo {
			live[a] = true
		}
		for _, a := range raw {
			if !live[a] {
				cur.StaleTargets = append(cur.StaleTargets, a)
			}
		}
		next, add, remove, ev, err := apply(cur)
		if err != nil {
			return err
		}
		if ev == nil {
			out = cur
			return nil
		}
		res := tx.Model(&ControlImplementationRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{
				"description": next.Description, "status": string(next.Status), "in_place_since": dateToTime(next.InPlaceSince),
				"owner_id": next.OwnerID, "assignee_id": next.AssigneeID, "reviewer_id": next.ReviewerID,
				"updated_at": next.UpdatedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return implementationNotFound()
		}
		// A retired implementation leaves the scope of its control (#555).
		if err := controlstatus.MarkImplementation(ctx, tx, tenantID, id); err != nil {
			return err
		}
		if len(remove) > 0 {
			if err := tx.Where("tenant_id = ? AND from_type = ? AND from_id = ? AND verb = ? AND to_type = ? AND to_id IN ? AND qualifier = ?",
				tenantID, string(canonical.TypeControlImplementation), id, string(canonical.VerbAppliesTo),
				string(canonical.TypeAsset), remove, uuid.Nil).Delete(&CanonicalEdgeRow{}).Error; err != nil {
				return err
			}
		}
		if err := insertAppliesTo(ctx, tx, tenantID, add); err != nil {
			return err
		}
		if err := r.audit.AppendTx(ctx, tx, ev); err != nil {
			return err
		}
		targets, err := appliesToAssets(ctx, tx, tenantID, []uuid.UUID{id})
		if err != nil {
			return err
		}
		out = ctlapp.ImplementationRecord{Implementation: next, AppliesTo: targets[id]}
		return nil
	})
	if err != nil {
		return nil, wrapApp(err, "controlchain.UpdateImplementation")
	}
	return &out, nil
}

// DeleteImplementation soft-deletes the live implementation and its live tests
// (UPDATE deleted_at, tenant_id in every WHERE, never a hard delete) after apply
// accepted it, with the audit entries apply returned, in one transaction. The
// applies_to edges are kept: readers filter on the liveness of both ends.
func (r *ImplementationRepository) DeleteImplementation(ctx context.Context, tenantID, id uuid.UUID,
	apply func(cur ctlapp.ImplementationRecord, tests []canonical.ControlTest) ([]*domain.AuditEvent, error),
) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := liveImplementationRow(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		cur, err := r.record(ctx, tx, tenantID, row)
		if err != nil {
			return err
		}
		var testRows []ControlTestRow
		if err := tx.Where("tenant_id = ? AND implementation_id = ? AND deleted_at IS NULL", tenantID, id).
			Order("id ASC").Find(&testRows).Error; err != nil {
			return err
		}
		tests := make([]canonical.ControlTest, len(testRows))
		for i := range testRows {
			if tests[i], err = RowToTest(testRows[i]); err != nil {
				return err
			}
		}
		events, err := apply(cur, tests)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return errors.New("controlchain.DeleteImplementation: an audit entry is required")
		}
		at := events[0].CreatedAt
		// Marked before the delete, while the implementation still resolves to its control (#555).
		if err := controlstatus.MarkImplementation(ctx, tx, tenantID, id); err != nil {
			return err
		}
		res := tx.Model(&ControlImplementationRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{"deleted_at": at, "updated_at": at})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return implementationNotFound()
		}
		if err := tx.Model(&ControlTestRow{}).
			Where("tenant_id = ? AND implementation_id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{"deleted_at": at, "updated_at": at}).Error; err != nil {
			return err
		}
		for _, ev := range events {
			if err := r.audit.AppendTx(ctx, tx, ev); err != nil {
				return err
			}
		}
		return nil
	})
	return wrapApp(err, "controlchain.DeleteImplementation")
}
