// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// advisoryLockKey serialises Reconcile across replicas booting together. The
// value is arbitrary and only has to be stable ("553" and the migration number).
const advisoryLockKey int64 = 553_0085

// insertBatch keeps every INSERT far below PostgreSQL's 65535 bind parameters.
const insertBatch = 500

// Result is what one Reconcile found and wrote. Every Unplaceable and Collapsed
// slice lists row ids, so an operator can find what was not carried.
type Result struct {
	// compliance_controls: every row, soft-deleted included, has exactly one split.
	Requirements         int64
	ControlsCreated      int
	ControlsAlreadyThere int
	ApplicabilityFlagged int         // not_applicable rows, split with the pending flag
	StatusDefaulted      []uuid.UUID // rows whose status the model does not know
	AddressedByCreated   int

	// evidence_control_links: carried, or listed as unplaceable.
	Links            int64
	LinksCarried     int
	LinksCreated     int
	LinksUnplaceable []uuid.UUID

	// risk_control_mappings: every row is in exactly one of these classes.
	Mappings              int64
	MappingsCarried       int
	MappingsCreated       int
	MappingsSoftDeleted   int
	MappingsFrameworkOnly int // control_id NULL: left without an edge (ADR 0008 D7)
	MappingsUnplaceable   []uuid.UUID
	MappingsCollapsed     []uuid.UUID // live duplicates of a (risk, control) pair already carried
}

// Created is the number of rows Reconcile inserted: zero on a converged database.
func (r Result) Created() int {
	return r.ControlsCreated + r.AddressedByCreated + r.LinksCreated + r.MappingsCreated
}

type reqSrc struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type linkSrc struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	EvidenceID uuid.UUID
	ControlID  uuid.UUID
	Note       string
	CreatedAt  time.Time
}

type mappingSrc struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	RiskID    uuid.UUID
	ControlID *uuid.UUID
	Note      string
	CreatedAt time.Time
	DeletedAt *time.Time
}

type idTenant struct {
	ID       uuid.UUID
	TenantID uuid.UUID
}

type edgeKey struct {
	Tenant    uuid.UUID
	FromType  string
	FromID    uuid.UUID
	Verb      string
	ToType    string
	ToID      uuid.UUID
	Qualifier uuid.UUID
}

func keyOf(r CanonicalEdgeRow) edgeKey {
	return edgeKey{r.TenantID, r.FromType, r.FromID, r.Verb, r.ToType, r.ToID, r.Qualifier}
}

// Reconcile makes the control chain agree with the legacy compliance tables
// (#553, D-108). It is insert-only and convergent: for every compliance_controls
// row without one it creates the split Control and its addressed_by edge, and it
// carries each evidence_control_links row as a control-level evidenced_by edge
// and each risk_control_mappings row that names a control as a mitigated_by edge.
// It never updates or deletes a row, so a late replay changes nothing that
// exists, and a second run inserts nothing.
//
// It runs in one transaction. On PostgreSQL it first takes a transaction-level
// advisory lock, so two replicas booting together do not both insert: the second
// waits, then finds everything present. It returns an error, and the transaction
// rolls back, when the counts do not reconcile with count(*) on the source tables.
//
// Tenant isolation: this is a boot-time system task, so it reads every tenant's
// legacy rows in one pass. Nothing crosses tenants: every row it writes carries
// the tenant_id of the legacy row it comes from, and a link or mapping whose
// ends do not both resolve in that same tenant is not carried, it is listed.
func Reconcile(ctx context.Context, db *gorm.DB) (Result, error) {
	var res Result
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			// tenant: a process-wide mutex between replicas, it reads no row.
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", advisoryLockKey).Error; err != nil {
				return fmt.Errorf("controlchain.Reconcile: advisory lock: %w", err)
			}
		}
		var err error
		res, err = reconcileTx(tx)
		return err
	})
	if err != nil {
		return res, err
	}
	logResult(res)
	return res, nil
}

func logResult(r Result) {
	log.Info().
		Int64("requirements", r.Requirements).
		Int("controls_created", r.ControlsCreated).
		Int("controls_already_there", r.ControlsAlreadyThere).
		Int("applicability_flagged", r.ApplicabilityFlagged).
		Int("addressed_by_created", r.AddressedByCreated).
		Int64("evidence_links", r.Links).
		Int("evidence_links_carried", r.LinksCarried).
		Int("evidence_links_created", r.LinksCreated).
		Int("evidence_links_unplaceable", len(r.LinksUnplaceable)).
		Int64("risk_mappings", r.Mappings).
		Int("risk_mappings_carried", r.MappingsCarried).
		Int("risk_mappings_created", r.MappingsCreated).
		Int("risk_mappings_soft_deleted", r.MappingsSoftDeleted).
		Int("risk_mappings_framework_only", r.MappingsFrameworkOnly).
		Int("risk_mappings_unplaceable", len(r.MappingsUnplaceable)).
		Int("risk_mappings_collapsed", len(r.MappingsCollapsed)).
		Msg("controlchain: reconciled")
	if len(r.LinksUnplaceable) > 0 {
		log.Warn().Strs("ids", uuidStrings(r.LinksUnplaceable)).
			Msg("controlchain: evidence links not carried, an end does not resolve in the link's tenant")
	}
	if len(r.MappingsUnplaceable) > 0 {
		log.Warn().Strs("ids", uuidStrings(r.MappingsUnplaceable)).
			Msg("controlchain: risk mappings not carried, an end does not resolve in the mapping's tenant")
	}
	if len(r.MappingsCollapsed) > 0 {
		log.Info().Strs("ids", uuidStrings(r.MappingsCollapsed)).
			Msg("controlchain: duplicate risk mappings merged into one mitigated_by edge")
	}
	if len(r.StatusDefaulted) > 0 {
		log.Warn().Strs("ids", uuidStrings(r.StatusDefaulted)).
			Msg("controlchain: requirements with an unknown status split as not_implemented")
	}
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func reconcileTx(tx *gorm.DB) (Result, error) {
	var res Result

	// Source tables, soft-deleted rows included (D-108 Q8).
	var reqs []reqSrc
	if err := tx.Table("compliance_controls").
		Select("id, tenant_id, name, COALESCE(description, '') AS description, status, created_at, updated_at, deleted_at").
		Order("created_at, id").Find(&reqs).Error; err != nil {
		return res, fmt.Errorf("controlchain.Reconcile: read compliance_controls: %w", err)
	}
	var links []linkSrc
	if err := tx.Table("evidence_control_links").
		Select("id, tenant_id, evidence_id, control_id, COALESCE(note, '') AS note, created_at").
		Order("created_at, id").Find(&links).Error; err != nil {
		return res, fmt.Errorf("controlchain.Reconcile: read evidence_control_links: %w", err)
	}
	var mappings []mappingSrc
	if err := tx.Table("risk_control_mappings").
		Select("id, tenant_id, risk_id, control_id, COALESCE(note, '') AS note, created_at, deleted_at").
		Order("created_at, id").Find(&mappings).Error; err != nil {
		return res, fmt.Errorf("controlchain.Reconcile: read risk_control_mappings: %w", err)
	}
	for table, dst := range map[string]*int64{
		"compliance_controls": &res.Requirements, "evidence_control_links": &res.Links, "risk_control_mappings": &res.Mappings,
	} {
		// tenant: count(*) of the whole source table is the reconciliation proof, taken across all tenants by design.
		if err := tx.Table(table).Count(dst).Error; err != nil {
			return res, fmt.Errorf("controlchain.Reconcile: count %s: %w", table, err)
		}
	}
	// Both ends of a link or mapping must live in the row's own tenant.
	reqTenant := make(map[uuid.UUID]uuid.UUID, len(reqs))
	for _, r := range reqs {
		reqTenant[r.ID] = r.TenantID
	}
	evidenceTenant, err := tenantsOf(tx, "evidences")
	if err != nil {
		return res, err
	}
	riskTenant, err := tenantsOf(tx, "risks")
	if err != nil {
		return res, err
	}

	haveControls, err := loadControlIDs(tx)
	if err != nil {
		return res, err
	}
	haveEdges, err := loadEdgeKeys(tx)
	if err != nil {
		return res, err
	}

	var newControls []ControlRow
	var newEdges []CanonicalEdgeRow
	planned := map[edgeKey]bool{} // every edge the legacy rows imply, present or not
	plan := func(e canonical.Edge) (isNew bool, err error) {
		row, err := EdgeToRow(e)
		if err != nil {
			return false, err
		}
		k := keyOf(row)
		if planned[k] {
			return false, nil
		}
		planned[k] = true
		if haveEdges[k] {
			return false, nil
		}
		newEdges = append(newEdges, row)
		return true, nil
	}

	plannedControls := make([]idTenant, 0, len(reqs))
	for _, r := range reqs {
		control, defaulted := SplitControl(RequirementRow{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Description: r.Description, Status: r.Status,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt,
		}, canonical.MethodMigration)
		if defaulted {
			res.StatusDefaulted = append(res.StatusDefaulted, r.ID)
		}
		if control.ApplicabilityReviewPending() {
			res.ApplicabilityFlagged++
		}
		plannedControls = append(plannedControls, idTenant{control.ID, r.TenantID})
		if haveControls[idTenant{control.ID, r.TenantID}] {
			res.ControlsAlreadyThere++
		} else {
			row, err := ControlToRow(control, r.DeletedAt)
			if err != nil {
				return res, err
			}
			newControls = append(newControls, row)
			res.ControlsCreated++
		}
		full := canonical.CoverageFull
		isNew, err := plan(Edge(r.TenantID, canonical.TypeRequirement, r.ID, canonical.VerbAddressedBy,
			canonical.TypeControl, control.ID, canonical.EdgeAttributes{Coverage: &full}, r.ID.String(), r.CreatedAt))
		if err != nil {
			return res, err
		}
		if isNew {
			res.AddressedByCreated++
		}
	}

	for _, l := range links {
		if evidenceTenant[l.EvidenceID] != l.TenantID || reqTenant[l.ControlID] != l.TenantID || l.TenantID == uuid.Nil {
			res.LinksUnplaceable = append(res.LinksUnplaceable, l.ID)
			continue
		}
		attrs := canonical.EdgeAttributes{}
		if l.Note != "" {
			note := l.Note
			attrs.Note = &note
		}
		isNew, err := plan(Edge(l.TenantID, canonical.TypeControl, canonical.ControlIDFor(l.ControlID), canonical.VerbEvidencedBy,
			canonical.TypeEvidence, l.EvidenceID, attrs, l.ID.String(), l.CreatedAt))
		if err != nil {
			return res, err
		}
		res.LinksCarried++
		if isNew {
			res.LinksCreated++
		}
	}

	for _, m := range mappings {
		switch {
		case m.DeletedAt != nil:
			res.MappingsSoftDeleted++
			continue
		case m.ControlID == nil:
			res.MappingsFrameworkOnly++
			continue
		case riskTenant[m.RiskID] != m.TenantID || reqTenant[*m.ControlID] != m.TenantID || m.TenantID == uuid.Nil:
			res.MappingsUnplaceable = append(res.MappingsUnplaceable, m.ID)
			continue
		}
		attrs := canonical.EdgeAttributes{}
		if m.Note != "" {
			note := m.Note
			attrs.Note = &note
		}
		e := Edge(m.TenantID, canonical.TypeRisk, m.RiskID, canonical.VerbMitigatedBy,
			canonical.TypeControl, canonical.ControlIDFor(*m.ControlID), attrs, m.ID.String(), m.CreatedAt)
		row, err := EdgeToRow(e)
		if err != nil {
			return res, err
		}
		if planned[keyOf(row)] {
			res.MappingsCollapsed = append(res.MappingsCollapsed, m.ID)
			continue
		}
		isNew, err := plan(e)
		if err != nil {
			return res, err
		}
		res.MappingsCarried++
		if isNew {
			res.MappingsCreated++
		}
	}

	if err := insertAll(tx, newControls); err != nil {
		return res, fmt.Errorf("controlchain.Reconcile: insert controls: %w", err)
	}
	if err := insertAll(tx, newEdges); err != nil {
		return res, fmt.Errorf("controlchain.Reconcile: insert edges: %w", err)
	}

	// The proof: what was planned is in the database, and the classes add up to
	// count(*) of each source table, or the transaction rolls back and the boot stops.
	afterControls, err := loadControlIDs(tx)
	if err != nil {
		return res, err
	}
	for _, c := range plannedControls {
		if !afterControls[c] {
			return res, fmt.Errorf("controlchain.Reconcile: split control %s of tenant %s is not in controls after the insert; nothing was written", c.ID, c.TenantID)
		}
	}
	afterEdges, err := loadEdgeKeys(tx)
	if err != nil {
		return res, err
	}
	for k := range planned {
		if !afterEdges[k] {
			return res, fmt.Errorf("controlchain.Reconcile: edge %s %s %s of tenant %s is not in canonical_edges after the insert; nothing was written", k.FromID, k.Verb, k.ToID, k.Tenant)
		}
	}
	if got := int64(res.ControlsCreated + res.ControlsAlreadyThere); got != res.Requirements {
		return res, fmt.Errorf("controlchain.Reconcile: %d controls accounted for, compliance_controls has %d rows; nothing was written", got, res.Requirements)
	}
	if got := int64(res.LinksCarried + len(res.LinksUnplaceable)); got != res.Links {
		return res, fmt.Errorf("controlchain.Reconcile: %d evidence links accounted for, evidence_control_links has %d rows; nothing was written", got, res.Links)
	}
	if got := int64(res.MappingsCarried + len(res.MappingsCollapsed) + len(res.MappingsUnplaceable) +
		res.MappingsSoftDeleted + res.MappingsFrameworkOnly); got != res.Mappings {
		return res, fmt.Errorf("controlchain.Reconcile: %d risk mappings accounted for, risk_control_mappings has %d rows; nothing was written", got, res.Mappings)
	}
	return res, nil
}

func insertAll[T any](tx *gorm.DB, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	// DoNothing is a belt: under the advisory lock nothing else inserts, and the
	// caller verifies afterwards that every planned row exists.
	return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, insertBatch).Error
}

func tenantsOf(tx *gorm.DB, table string) (map[uuid.UUID]uuid.UUID, error) {
	var rows []idTenant
	if err := tx.Table(table).Select("id, tenant_id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("controlchain.Reconcile: read %s: %w", table, err)
	}
	out := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, r := range rows {
		out[r.ID] = r.TenantID
	}
	return out, nil
}

func loadControlIDs(tx *gorm.DB) (map[idTenant]bool, error) {
	var rows []idTenant
	if err := tx.Table(TableControls).Select("id, tenant_id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("controlchain.Reconcile: read controls: %w", err)
	}
	out := make(map[idTenant]bool, len(rows))
	for _, r := range rows {
		out[r] = true
	}
	return out, nil
}

// loadEdgeKeys reads the keys of the verbs Reconcile writes.
func loadEdgeKeys(tx *gorm.DB) (map[edgeKey]bool, error) {
	var rows []CanonicalEdgeRow
	if err := tx.Select("tenant_id, from_type, from_id, verb, to_type, to_id, qualifier").
		Where("verb IN ?", []string{string(canonical.VerbAddressedBy), string(canonical.VerbEvidencedBy), string(canonical.VerbMitigatedBy)}).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("controlchain.Reconcile: read canonical_edges: %w", err)
	}
	out := make(map[edgeKey]bool, len(rows))
	for _, r := range rows {
		out[keyOf(r)] = true
	}
	return out, nil
}
