// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/pkg/monitoring"
)

// The mirror (#1413, D-108 follow-up R1-R7). The legacy repositories call these
// functions inside their own transaction, after the legacy write, so the chain
// and the legacy table commit or roll back together. The mirror is one way:
// nothing here reads the chain back into a legacy response, and nothing changes
// a legacy table.
//
// Every function takes the transaction handle `tx` of its caller and uses only
// it (a second connection would not see the caller's uncommitted rows). Every
// query carries the tenant_id of the write. A target that does not resolve in
// that tenant is domain.ErrNotFound, so a foreign id answers as an absent one.
//
// Idempotency: every insert is OnConflict DoNothing on the unique keys
// (ux_controls_identity, ux_canonical_edges_key) and builds the same
// from/to/verb/qualifier as Reconcile, so a boot reconcile and a live mirror
// converge on one key set.

// mirrorMethod is the provenance method of everything the mirror writes (R5).
const mirrorMethod = canonical.MethodManualEntry

// Skip reasons of the openrisk_control_chain_mirror_skipped_total counter.
const (
	reasonRequirementUnsplittable = "requirement_unsplittable"
	reasonEdgeInvalid             = "edge_invalid"
)

// entityTable is where a canonical entity type lives and whether it has a
// soft-delete column.
type entityTable struct {
	table      string
	softDelete bool
}

// entityTables is the shared edge resolver's registry (R1): one entry per
// entity type a mirrored edge can name. Table names are constants of this
// file, never input.
var entityTables = map[canonical.EntityType]entityTable{
	canonical.TypeRequirement: {"compliance_controls", true},
	canonical.TypeControl:     {TableControls, true},
	canonical.TypeRisk:        {"risks", true},
	canonical.TypeEvidence:    {"evidences", true},
}

// Resolve is the one gate every mirrored edge passes both of its ends through
// (R1): the entity of that type with that id must exist in the tenant and, where
// the table has a deleted_at column, be live (R7). Absent, foreign or deleted is
// domain.ErrNotFound, never a different error: a foreign id is not confirmed to
// exist.
func Resolve(ctx context.Context, tx *gorm.DB, tenant uuid.UUID, t canonical.EntityType, id uuid.UUID) error {
	if tenant == uuid.Nil {
		return domain.NewForbiddenError("tenant_id is required")
	}
	et, ok := entityTables[t]
	if !ok {
		log.Error().Str("entity_type", string(t)).Msg("controlchain: no table is registered for this entity type")
		return domain.NewValidationError("the entity type cannot be resolved")
	}
	q := tx.WithContext(ctx).Table(et.table).Where("id = ? AND tenant_id = ?", id, tenant)
	if et.softDelete {
		q = q.Where("deleted_at IS NULL")
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return fmt.Errorf("controlchain.Resolve: %s: %w", et.table, err)
	}
	if n == 0 {
		return domain.NewNotFoundError(string(t), id)
	}
	return nil
}

func skipped(reason string, tenant uuid.UUID, ids ...uuid.UUID) {
	monitoring.ControlChainMirrorSkipped.WithLabelValues(reason).Inc()
	// Ids only: a note or a name can hold anything, and is never logged.
	log.Warn().Str("reason", reason).Str("tenant_id", tenant.String()).Strs("ids", uuidStrings(ids)).
		Msg("controlchain: legacy write succeeded, its control chain edge was not written; the boot reconciler refuses the same input")
}

// insertSplit writes the split control of one requirement and resolves it back
// with the tenant (R2): the control id is global, so a control of that id owned
// by another tenant, or already deleted, makes the insert a no-op and the
// resolve answers ErrNotFound. It reports false, with nothing written, when the
// canonical Control refuses the requirement (R3).
func insertSplit(ctx context.Context, tx *gorm.DB, req RequirementRow) (bool, error) {
	control, _ := SplitControl(req, mirrorMethod)
	if control.Validate() != nil {
		return false, nil
	}
	row, err := ControlToRow(control, nil)
	if err != nil {
		return false, err
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return false, fmt.Errorf("controlchain.mirror: insert control: %w", err)
	}
	if err := Resolve(ctx, tx, req.TenantID, canonical.TypeControl, control.ID); err != nil {
		return false, err
	}
	return true, nil
}

// ensureControl returns true when the split control of the requirement is in
// the chain for the tenant, creating it from the tenant's own requirement row
// when it is not there yet (R3: a legacy row the reconciler could not split, or
// a write racing the boot reconcile). The requirement is read with the tenant,
// so a requirement of another tenant is ErrNotFound. It returns false, nothing
// written, when the requirement cannot be split.
func ensureControl(ctx context.Context, tx *gorm.DB, tenant, requirementID uuid.UUID) (bool, error) {
	err := Resolve(ctx, tx, tenant, canonical.TypeControl, canonical.ControlIDFor(requirementID))
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return false, err
	}
	var rows []reqSrc
	if err := tx.WithContext(ctx).Table("compliance_controls").
		Select("id, tenant_id, name, COALESCE(description, '') AS description, status, created_at, updated_at, deleted_at").
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", requirementID, tenant).
		Limit(1).Find(&rows).Error; err != nil {
		return false, fmt.Errorf("controlchain.mirror: read requirement: %w", err)
	}
	if len(rows) == 0 {
		return false, domain.NewNotFoundError(string(canonical.TypeRequirement), requirementID)
	}
	r := rows[0]
	ok, err := insertSplit(ctx, tx, RequirementRow{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Description: r.Description, Status: r.Status,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
	if err != nil {
		return false, err
	}
	if !ok {
		skipped(reasonRequirementUnsplittable, tenant, requirementID)
	}
	return ok, nil
}

func noteAttrs(note string) canonical.EdgeAttributes {
	attrs := canonical.EdgeAttributes{}
	if note != "" {
		attrs.Note = &note
	}
	return attrs
}

// putEdge inserts one carried edge, idempotently. An edge the chain refuses
// (a credential URL pasted in a note, say) is not written and does not fail the
// legacy write (R4): the reconciler lists the same input as unplaceable, so the
// two converge on the same key set.
func putEdge(ctx context.Context, tx *gorm.DB, e canonical.Edge, legacyID uuid.UUID) error {
	if e.Validate() != nil {
		skipped(reasonEdgeInvalid, e.TenantID, legacyID)
		return nil
	}
	row, err := EdgeToRow(e)
	if err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return fmt.Errorf("controlchain.mirror: insert edge: %w", err)
	}
	return nil
}

// deleteEdges removes the edges of one verb from one end to the given ends. The
// tenant is in the statement, so another tenant's edge of the same ids is never
// touched.
func deleteEdges(ctx context.Context, tx *gorm.DB, tenant uuid.UUID, fromType canonical.EntityType, fromIDs []uuid.UUID,
	verb canonical.Verb, toType canonical.EntityType, toIDs []uuid.UUID,
) error {
	if len(fromIDs) == 0 || len(toIDs) == 0 {
		return nil
	}
	err := tx.WithContext(ctx).
		Where("tenant_id = ? AND from_type = ? AND from_id IN ? AND verb = ? AND to_type = ? AND to_id IN ? AND qualifier = ?",
			tenant, string(fromType), fromIDs, string(verb), string(toType), toIDs, uuid.Nil).
		Delete(&CanonicalEdgeRow{}).Error
	if err != nil {
		return fmt.Errorf("controlchain.mirror: delete edges: %w", err)
	}
	return nil
}

func needTenant(tenant uuid.UUID) error {
	if tenant == uuid.Nil {
		return domain.NewForbiddenError("tenant_id is required")
	}
	return nil
}

// MirrorRequirementCreated writes the split control of a requirement just
// created and its addressed_by edge, coverage full (AC1). Unlike a link or a
// mapping, a requirement the chain refuses fails the call, so the caller's
// transaction rolls back and neither the requirement nor the control persists.
func MirrorRequirementCreated(ctx context.Context, tx *gorm.DB, req RequirementRow) error {
	if err := needTenant(req.TenantID); err != nil {
		return err
	}
	ok, err := insertSplit(ctx, tx, req)
	if err != nil {
		return err
	}
	if !ok {
		return domain.NewValidationError("the requirement cannot be carried onto the control chain")
	}
	if err := Resolve(ctx, tx, req.TenantID, canonical.TypeRequirement, req.ID); err != nil {
		return err
	}
	full := canonical.CoverageFull
	e := Edge(req.TenantID, canonical.TypeRequirement, req.ID, canonical.VerbAddressedBy,
		canonical.TypeControl, canonical.ControlIDFor(req.ID), canonical.EdgeAttributes{Coverage: &full},
		req.ID.String(), req.CreatedAt, mirrorMethod)
	if err := e.Validate(); err != nil {
		log.Error().Str("requirement_id", req.ID.String()).Err(err).Msg("controlchain: addressed_by edge refused")
		return domain.NewValidationError("the requirement cannot be carried onto the control chain")
	}
	return putEdge(ctx, tx, e, req.ID)
}

// MirrorEvidenceLinked writes the control-level evidenced_by edge of an
// evidence link (AC2). Both ends resolve in the tenant first (AC3). linkID and
// note are those of the stored link row: when the link already existed, the
// caller passes the stored ones, so the edge carries what the reconciler would.
func MirrorEvidenceLinked(ctx context.Context, tx *gorm.DB, tenant, linkID, evidenceID, requirementID uuid.UUID, note string, at time.Time) error {
	if err := needTenant(tenant); err != nil {
		return err
	}
	if err := Resolve(ctx, tx, tenant, canonical.TypeEvidence, evidenceID); err != nil {
		return err
	}
	ok, err := ensureControl(ctx, tx, tenant, requirementID)
	if err != nil || !ok {
		return err
	}
	return putEdge(ctx, tx, Edge(tenant, canonical.TypeControl, canonical.ControlIDFor(requirementID),
		canonical.VerbEvidencedBy, canonical.TypeEvidence, evidenceID, noteAttrs(note), linkID.String(), at, mirrorMethod), linkID)
}

// MirrorEvidenceUnlinked removes the evidenced_by edge of one (evidence,
// requirement) pair. One link per pair, so there is no other live row to keep
// the edge for.
func MirrorEvidenceUnlinked(ctx context.Context, tx *gorm.DB, tenant, evidenceID, requirementID uuid.UUID) error {
	if err := needTenant(tenant); err != nil {
		return err
	}
	return deleteEdges(ctx, tx, tenant, canonical.TypeControl, []uuid.UUID{canonical.ControlIDFor(requirementID)},
		canonical.VerbEvidencedBy, canonical.TypeEvidence, []uuid.UUID{evidenceID})
}

// MirrorEvidenceDeleted removes the evidenced_by edges of an evidence whose
// links the caller hard-deleted. requirementIDs are the links' control_ids,
// read by the caller before the delete.
func MirrorEvidenceDeleted(ctx context.Context, tx *gorm.DB, tenant, evidenceID uuid.UUID, requirementIDs []uuid.UUID) error {
	if err := needTenant(tenant); err != nil {
		return err
	}
	controlIDs := make([]uuid.UUID, len(requirementIDs))
	for i, id := range requirementIDs {
		controlIDs[i] = canonical.ControlIDFor(id)
	}
	return deleteEdges(ctx, tx, tenant, canonical.TypeControl, controlIDs,
		canonical.VerbEvidencedBy, canonical.TypeEvidence, []uuid.UUID{evidenceID})
}

// MirrorMappingCreated writes the mitigated_by edge of a risk-to-control
// mapping (AC2). A framework-only mapping (requirementID nil) has no edge (ADR
// 0008 D7), and no target to resolve. Several live mappings of one (risk,
// control) pair are one edge, as in Reconcile: the first writes it, the others
// find it present.
func MirrorMappingCreated(ctx context.Context, tx *gorm.DB, tenant, mappingID, riskID uuid.UUID, requirementID *uuid.UUID, note string, at time.Time) error {
	if err := needTenant(tenant); err != nil {
		return err
	}
	if requirementID == nil {
		return nil
	}
	if err := Resolve(ctx, tx, tenant, canonical.TypeRisk, riskID); err != nil {
		return err
	}
	ok, err := ensureControl(ctx, tx, tenant, *requirementID)
	if err != nil || !ok {
		return err
	}
	return putEdge(ctx, tx, Edge(tenant, canonical.TypeRisk, riskID, canonical.VerbMitigatedBy,
		canonical.TypeControl, canonical.ControlIDFor(*requirementID), noteAttrs(note), mappingID.String(), at, mirrorMethod), mappingID)
}

// MirrorMappingDeleted removes the mitigated_by edge of a mapping the caller has
// just soft-deleted (AC2), unless another live mapping of the same (risk,
// control) pair remains: the pair is one edge, and that mapping still asserts
// it. The count excludes the deleted row by id as well as by deleted_at. When the
// edge outlives the mapping whose id it carries as source_id, it keeps that id
// (R6): the reconciler is insert-only as well.
func MirrorMappingDeleted(ctx context.Context, tx *gorm.DB, tenant, mappingID, riskID uuid.UUID, requirementID *uuid.UUID) error {
	if err := needTenant(tenant); err != nil {
		return err
	}
	if requirementID == nil {
		return nil
	}
	var others int64
	if err := tx.WithContext(ctx).Table("risk_control_mappings").
		Where("tenant_id = ? AND risk_id = ? AND control_id = ? AND id <> ? AND deleted_at IS NULL",
			tenant, riskID, *requirementID, mappingID).
		Count(&others).Error; err != nil {
		return fmt.Errorf("controlchain.mirror: count other mappings: %w", err)
	}
	if others > 0 {
		return nil
	}
	return deleteEdges(ctx, tx, tenant, canonical.TypeRisk, []uuid.UUID{riskID},
		canonical.VerbMitigatedBy, canonical.TypeControl, []uuid.UUID{canonical.ControlIDFor(*requirementID)})
}
