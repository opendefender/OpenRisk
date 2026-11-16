// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
	"github.com/opendefender/openrisk/pkg/monitoring"
)

// #1413: the legacy repositories mirror their writes onto the control chain in
// the same transaction. The scenarios below run twice, on sqlite and on a
// throwaway PostgreSQL schema (control_chain_mirror_pg_test.go), because the
// transaction, the ON CONFLICT clauses and the unique keys are the point and
// only PostgreSQL proves them.

type mirrorEnv struct {
	db   *gorm.DB
	comp *GormComplianceRepository
	ev   *GormEvidenceRepository
	maps *GormRiskControlMappingRepository
}

// mirrorLegacyDDL is the legacy side of the schema, as the columns the
// repositories and the mirror read and write, in the types of the dialect.
func mirrorLegacyDDL(id, ts string) []string {
	return []string{
		`CREATE TABLE compliance_controls (id ` + id + ` PRIMARY KEY, tenant_id ` + id + ` NOT NULL, framework_id ` + id + ` NOT NULL,
			reference_code TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, description TEXT, source_reference TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'not_implemented', created_at ` + ts + `, updated_at ` + ts + `, deleted_at ` + ts + `)`,
		`CREATE UNIQUE INDEX idx_mirror_compliance_controls_ref ON compliance_controls (tenant_id, framework_id, reference_code)
			WHERE deleted_at IS NULL AND reference_code <> ''`,
		`CREATE TABLE risks (id ` + id + ` PRIMARY KEY, tenant_id ` + id + ` NOT NULL, title TEXT, score DOUBLE PRECISION DEFAULT 0,
			smart_score DOUBLE PRECISION DEFAULT 0, residual_risk DOUBLE PRECISION, created_at ` + ts + `, updated_at ` + ts + `, deleted_at ` + ts + `)`,
		`CREATE TABLE risk_histories (id ` + id + ` PRIMARY KEY, risk_id ` + id + ` NOT NULL, score DOUBLE PRECISION, created_at ` + ts + `)`,
		`CREATE TABLE risk_control_mappings (id ` + id + ` PRIMARY KEY, tenant_id ` + id + ` NOT NULL, risk_id ` + id + ` NOT NULL,
			framework_id ` + id + ` NOT NULL, control_id ` + id + `, note TEXT, created_by ` + id + `, source TEXT DEFAULT 'manual',
			created_at ` + ts + `, updated_at ` + ts + `, deleted_at ` + ts + `)`,
	}
}

func newMirrorEnv(t *testing.T, db *gorm.DB) *mirrorEnv {
	t.Helper()
	for _, ddl := range mirrorLegacyDDL("UUID", "TIMESTAMPTZ") {
		if db.Dialector.Name() == "sqlite" {
			ddl = strings.NewReplacer("UUID", "TEXT", "TIMESTAMPTZ", "DATETIME").Replace(ddl)
		}
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, db.AutoMigrate(&domain.Evidence{}, &domain.EvidenceControlLink{}))
	require.NoError(t, db.AutoMigrate(controlchain.Models()...))
	return &mirrorEnv{
		db: db, comp: NewGormComplianceRepository(db), ev: NewGormEvidenceRepository(db), maps: NewGormRiskControlMappingRepository(db),
	}
}

func newMirrorEnvSQLite(t *testing.T) *mirrorEnv {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true, DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // one in-memory database for every statement, the transaction's included
	return newMirrorEnv(t, db)
}

// ---------------------------------------------------------------------------
// helpers

func (e *mirrorEnv) requirement(t *testing.T, tenant uuid.UUID, name string) uuid.UUID {
	t.Helper()
	c := &domain.ComplianceControl{ID: uuid.New(), TenantID: tenant, FrameworkID: uuid.New(), ReferenceCode: uuid.NewString()[:8],
		Name: name, Status: domain.ControlStatus("in_progress")}
	require.NoError(t, e.comp.CreateControl(context.Background(), c))
	return c.ID
}

// legacyOnlyRequirement inserts a requirement the way a pre-migration database
// holds it: no split control, no edge.
func (e *mirrorEnv) legacyOnlyRequirement(t *testing.T, tenant uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now().UTC()
	require.NoError(t, e.db.Exec(`INSERT INTO compliance_controls (id, tenant_id, framework_id, name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'not_implemented', ?, ?)`, id, tenant, uuid.New(), name, now, now).Error)
	return id
}

func (e *mirrorEnv) risk(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now().UTC()
	require.NoError(t, e.db.Exec(`INSERT INTO risks (id, tenant_id, title, score, smart_score, residual_risk, created_at, updated_at)
		VALUES (?, ?, 'r', 7.5, 63.25, 2.125, ?, ?)`, id, tenant, now, now).Error)
	require.NoError(t, e.db.Exec(`INSERT INTO risk_histories (id, risk_id, score, created_at) VALUES (?, ?, 7.5, ?)`, uuid.New(), id, now).Error)
	return id
}

func (e *mirrorEnv) evidence(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	ev := &domain.Evidence{TenantID: tenant, Title: "proof", CollectedAt: time.Now(), Review: domain.EvidenceReviewAccepted}
	require.NoError(t, e.ev.Create(context.Background(), ev))
	return ev.ID
}

func (e *mirrorEnv) link(t *testing.T, tenant, evidence, requirement uuid.UUID, note string) error {
	t.Helper()
	return e.ev.Link(context.Background(), &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: evidence, ControlID: requirement, Note: note})
}

func (e *mirrorEnv) mapping(t *testing.T, tenant, risk uuid.UUID, requirement *uuid.UUID, note string) (*domain.RiskControlMapping, error) {
	t.Helper()
	m := &domain.RiskControlMapping{TenantID: tenant, RiskID: risk, FrameworkID: uuid.New(), ControlID: requirement, Note: note}
	return m, e.maps.Create(context.Background(), m)
}

func (e *mirrorEnv) edges(t *testing.T, tenant uuid.UUID, verb canonical.Verb) []controlchain.CanonicalEdgeRow {
	t.Helper()
	var rows []controlchain.CanonicalEdgeRow
	require.NoError(t, e.db.Where("tenant_id = ? AND verb = ?", tenant, string(verb)).Find(&rows).Error)
	return rows
}

func (e *mirrorEnv) edgeCount(t *testing.T, tenant uuid.UUID, verb canonical.Verb) int {
	return len(e.edges(t, tenant, verb))
}

func (e *mirrorEnv) controlCount(t *testing.T, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Model(&controlchain.ControlRow{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	return n
}

func (e *mirrorEnv) requirementCount(t *testing.T, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Table("compliance_controls").Where("tenant_id = ?", tenant).Count(&n).Error)
	return n
}

// compactJSON: PostgreSQL's jsonb prints `"k": v`, sqlite keeps what was written.
func compactJSON(t *testing.T, raw string) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, json.Compact(&b, []byte(raw)))
	return b.String()
}

func skippedTotal(reason string) float64 {
	return testutil.ToFloat64(monitoring.ControlChainMirrorSkipped.WithLabelValues(reason))
}

// converged: Reconcile, run after the mirror wrote, finds nothing left to
// insert: the mirror and the reconciler agree on the key set.
func (e *mirrorEnv) converged(t *testing.T) {
	t.Helper()
	res, err := controlchain.Reconcile(context.Background(), e.db)
	require.NoError(t, err)
	assert.Zero(t, res.Created(), "the boot reconciler must find the mirror's rows already there: %+v", res)
}

// ---------------------------------------------------------------------------
// requirement create

func (s mirrorSuite) TestCreateControl_Success(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	id := e.requirement(t, tenant, "ICT Risk Management")

	var c controlchain.ControlRow
	require.NoError(t, e.db.Where("tenant_id = ? AND id = ?", tenant, canonical.ControlIDFor(id)).First(&c).Error)
	assert.Equal(t, "ICT Risk Management", c.Name)
	assert.Equal(t, "in_progress", c.DeclaredStatus)
	assert.Contains(t, compactJSON(t, c.Provenance), `"method":"manual_entry"`, "R5: a mirrored write is manual_entry, not migration")

	edges := e.edges(t, tenant, canonical.VerbAddressedBy)
	require.Len(t, edges, 1)
	assert.Equal(t, id, edges[0].FromID)
	assert.Equal(t, canonical.ControlIDFor(id), edges[0].ToID)
	assert.Equal(t, id.String(), edges[0].SourceID)
	assert.Contains(t, compactJSON(t, edges[0].Attributes), `"coverage":"full"`)
	assert.Contains(t, compactJSON(t, edges[0].Provenance), `"method":"manual_entry"`)
	e.converged(t)
}

// R2: the control id is global. A control of that id owned by another tenant
// makes the requirement unreachable, and nothing is written.
func (s mirrorSuite) TestCreateControl_NotFound_WhenTheControlIdBelongsToAnotherTenant(t *testing.T) {
	e := s.env(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	reqID := uuid.New()
	foreign := e.splitOf(t, tenantB, reqID)
	require.NoError(t, e.db.Create(&foreign).Error)

	err := e.comp.CreateControl(context.Background(), &domain.ComplianceControl{
		ID: reqID, TenantID: tenantA, FrameworkID: uuid.New(), Name: "n", Status: domain.ControlStatus("not_implemented"),
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Zero(t, e.requirementCount(t, tenantA), "the legacy row rolled back with the mirror")
	assert.Zero(t, e.controlCount(t, tenantA))
	assert.Zero(t, e.edgeCount(t, tenantA, canonical.VerbAddressedBy))
}

func (s mirrorSuite) TestCreateControl_Unauthorized_NilTenantWritesNothing(t *testing.T) {
	e := s.env(t)
	err := e.comp.CreateControl(context.Background(), &domain.ComplianceControl{ID: uuid.New(), FrameworkID: uuid.New(), Name: "n"})
	require.Error(t, err)
	assert.Zero(t, e.requirementCount(t, uuid.Nil))
	assert.Zero(t, e.controlCount(t, uuid.Nil))
}

// AC1: the second insert fails, neither persists.
func (s mirrorSuite) TestCreateControl_RollsBackWhenTheEdgeInsertFails(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	require.NoError(t, e.db.Exec(`ALTER TABLE canonical_edges RENAME TO canonical_edges_gone`).Error)

	id := uuid.New()
	err := e.comp.CreateControl(context.Background(), &domain.ComplianceControl{
		ID: id, TenantID: tenant, FrameworkID: uuid.New(), Name: "n", Status: domain.ControlStatus("not_implemented"),
	})
	require.Error(t, err)
	assert.Zero(t, e.requirementCount(t, tenant), "the requirement must not persist")
	assert.Zero(t, e.controlCount(t, tenant), "the split control, inserted first, must not persist either")
}

// A requirement the chain refuses (blank name) fails the create: AC1 over R3.
func (s mirrorSuite) TestCreateControl_RefusedByTheChainRollsBack(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	err := e.comp.CreateControl(context.Background(), &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenant, FrameworkID: uuid.New(), Name: "   ", Status: domain.ControlStatus("not_implemented"),
	})
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Zero(t, e.requirementCount(t, tenant))
	assert.Zero(t, e.controlCount(t, tenant))
}

// The duplicate reference code stays a conflict, as before.
func (s mirrorSuite) TestCreateControl_DuplicateReferenceStillConflicts(t *testing.T) {
	e := s.env(t)
	tenant, fw := uuid.New(), uuid.New()
	mk := func() error {
		return e.comp.CreateControl(context.Background(), &domain.ComplianceControl{
			ID: uuid.New(), TenantID: tenant, FrameworkID: fw, ReferenceCode: "A.5.1", Name: "n", Status: domain.ControlStatus("not_implemented"),
		})
	}
	require.NoError(t, mk())
	require.ErrorIs(t, mk(), domain.ErrConflict)
	assert.EqualValues(t, 1, e.controlCount(t, tenant), "the refused duplicate left no split control")
}

// ---------------------------------------------------------------------------
// evidence link / unlink / delete

func (s mirrorSuite) TestLink_Success(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)

	require.NoError(t, e.link(t, tenant, ev, req, "pentest 2026"))
	require.NoError(t, e.link(t, tenant, ev, req, "pressed twice"), "re-linking stays idempotent")

	edges := e.edges(t, tenant, canonical.VerbEvidencedBy)
	require.Len(t, edges, 1)
	assert.Equal(t, canonical.TypeControl, canonical.EntityType(edges[0].FromType))
	assert.Equal(t, canonical.ControlIDFor(req), edges[0].FromID)
	assert.Equal(t, ev, edges[0].ToID)
	assert.Contains(t, edges[0].Attributes, "pentest 2026", "the edge carries the stored link's note")
	var stored domain.EvidenceControlLink
	require.NoError(t, e.db.Where("tenant_id = ? AND evidence_id = ?", tenant, ev).First(&stored).Error)
	assert.Equal(t, stored.ID.String(), edges[0].SourceID, "source_id is the stored link row, as Reconcile would write it")
	e.converged(t)
}

func (s mirrorSuite) TestLink_NotFound_UnknownEvidenceOrRequirement(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)

	require.ErrorIs(t, e.link(t, tenant, uuid.New(), req, ""), domain.ErrNotFound)
	require.ErrorIs(t, e.link(t, tenant, ev, uuid.New(), ""), domain.ErrNotFound)
	var n int64
	require.NoError(t, e.db.Model(&domain.EvidenceControlLink{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	assert.Zero(t, n, "a refused edge takes its legacy link with it")
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
}

func (s mirrorSuite) TestLink_Unauthorized_NilTenantWritesNothing(t *testing.T) {
	e := s.env(t)
	err := e.link(t, uuid.Nil, uuid.New(), uuid.New(), "")
	require.Error(t, err)
	assert.Zero(t, e.edgeCount(t, uuid.Nil, canonical.VerbEvidencedBy))
}

// R3: a legacy requirement with no split control (a database not reconciled yet).
func (s mirrorSuite) TestLink_CreatesTheMissingSplitControl(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.legacyOnlyRequirement(t, tenant, "Legacy requirement")
	ev := e.evidence(t, tenant)
	require.Zero(t, e.controlCount(t, tenant))

	require.NoError(t, e.link(t, tenant, ev, req, ""))
	assert.EqualValues(t, 1, e.controlCount(t, tenant), "the split is created lazily from the tenant's requirement row")
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
	var c controlchain.ControlRow
	require.NoError(t, e.db.Where("tenant_id = ? AND id = ?", tenant, canonical.ControlIDFor(req)).First(&c).Error)
	assert.Contains(t, compactJSON(t, c.Provenance), `"method":"manual_entry"`)
	// The split has no addressed_by edge yet: Reconcile adds it, and nothing else.
	res, err := controlchain.Reconcile(context.Background(), e.db)
	require.NoError(t, err)
	assert.Equal(t, 1, res.AddressedByCreated)
	assert.Zero(t, res.LinksCreated)
	assert.Zero(t, res.ControlsCreated)
}

// R3 / AC5: a requirement the chain cannot split does not fail the legacy link.
func (s mirrorSuite) TestLink_UnsplittableRequirementKeepsTheLegacyWrite(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.legacyOnlyRequirement(t, tenant, "   ")
	ev := e.evidence(t, tenant)
	before := skippedTotal("requirement_unsplittable")

	require.NoError(t, e.link(t, tenant, ev, req, ""))
	var n int64
	require.NoError(t, e.db.Model(&domain.EvidenceControlLink{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	assert.EqualValues(t, 1, n, "the legacy link persists")
	assert.Zero(t, e.controlCount(t, tenant))
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
	assert.Equal(t, before+1, skippedTotal("requirement_unsplittable"))
	e.converged(t)
}

// R4: an edge the chain refuses (a credential URL in the note) is skipped as the
// reconciler skips it, and the legacy link persists.
func (s mirrorSuite) TestLink_CredentialNoteKeepsTheLegacyWriteAndWritesNoEdge(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)
	before := skippedTotal("edge_invalid")

	require.NoError(t, e.link(t, tenant, ev, req, "see https://h/o?X-Amz-Signature=abc"))
	var n int64
	require.NoError(t, e.db.Model(&domain.EvidenceControlLink{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	assert.EqualValues(t, 1, n)
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
	assert.Equal(t, before+1, skippedTotal("edge_invalid"))
	res, err := controlchain.Reconcile(context.Background(), e.db)
	require.NoError(t, err)
	assert.Zero(t, res.LinksCreated, "the reconciler refuses the same input: both converge on no edge")
	assert.Len(t, res.LinksUnplaceable, 1)
}

func (s mirrorSuite) TestUnlink_Success(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req, req2 := e.requirement(t, tenant, "A.5.1"), e.requirement(t, tenant, "A.5.2")
	ev := e.evidence(t, tenant)
	require.NoError(t, e.link(t, tenant, ev, req, ""))
	require.NoError(t, e.link(t, tenant, ev, req2, ""))

	require.NoError(t, e.ev.Unlink(context.Background(), tenant, ev, req))
	edges := e.edges(t, tenant, canonical.VerbEvidencedBy)
	require.Len(t, edges, 1, "only the unlinked pair's edge goes")
	assert.Equal(t, canonical.ControlIDFor(req2), edges[0].FromID)
	e.converged(t)
}

// Unlink of a pair that is not linked is a no-op, as it always was; another
// tenant's unlink never reaches this tenant's edge.
func (s mirrorSuite) TestUnlink_NotFound_IsANoOpAndCrossTenantKeepsTheEdge(t *testing.T) {
	e := s.env(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	req := e.requirement(t, tenantA, "A.5.1")
	ev := e.evidence(t, tenantA)
	require.NoError(t, e.link(t, tenantA, ev, req, ""))

	require.NoError(t, e.ev.Unlink(context.Background(), tenantA, ev, uuid.New()))
	require.NoError(t, e.ev.Unlink(context.Background(), tenantB, ev, req))
	assert.Equal(t, 1, e.edgeCount(t, tenantA, canonical.VerbEvidencedBy))
}

func (s mirrorSuite) TestUnlink_Unauthorized_NilTenantKeepsTheEdge(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)
	require.NoError(t, e.link(t, tenant, ev, req, ""))

	require.ErrorIs(t, e.ev.Unlink(context.Background(), uuid.Nil, ev, req), domain.ErrForbidden)
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
}

func (s mirrorSuite) TestEvidenceDelete_Success_RemovesEveryEdgeOfThatEvidenceOnly(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req, req2 := e.requirement(t, tenant, "A.5.1"), e.requirement(t, tenant, "A.5.2")
	gone, kept := e.evidence(t, tenant), e.evidence(t, tenant)
	for _, ev := range []uuid.UUID{gone, kept} {
		require.NoError(t, e.link(t, tenant, ev, req, ""))
	}
	require.NoError(t, e.link(t, tenant, gone, req2, ""))
	require.Equal(t, 3, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))

	require.NoError(t, e.ev.Delete(context.Background(), tenant, gone))
	edges := e.edges(t, tenant, canonical.VerbEvidencedBy)
	require.Len(t, edges, 1)
	assert.Equal(t, kept, edges[0].ToID)
	e.converged(t)
}

func (s mirrorSuite) TestEvidenceDelete_NotFound_AndCrossTenantKeepEdges(t *testing.T) {
	e := s.env(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	req := e.requirement(t, tenantA, "A.5.1")
	ev := e.evidence(t, tenantA)
	require.NoError(t, e.link(t, tenantA, ev, req, ""))

	require.ErrorIs(t, e.ev.Delete(context.Background(), tenantA, uuid.New()), domain.ErrNotFound)
	require.ErrorIs(t, e.ev.Delete(context.Background(), tenantB, ev), domain.ErrNotFound)
	assert.Equal(t, 1, e.edgeCount(t, tenantA, canonical.VerbEvidencedBy))
}

func (s mirrorSuite) TestEvidenceDelete_Unauthorized_NilTenantKeepsEdges(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)
	require.NoError(t, e.link(t, tenant, ev, req, ""))

	require.Error(t, e.ev.Delete(context.Background(), uuid.Nil, ev))
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
}

// ---------------------------------------------------------------------------
// risk mappings

func (s mirrorSuite) TestMappingCreate_Success(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	risk := e.risk(t, tenant)

	m, err := e.mapping(t, tenant, risk, &req, "mitigates")
	require.NoError(t, err)
	edges := e.edges(t, tenant, canonical.VerbMitigatedBy)
	require.Len(t, edges, 1)
	assert.Equal(t, risk, edges[0].FromID)
	assert.Equal(t, canonical.ControlIDFor(req), edges[0].ToID)
	assert.Equal(t, m.ID.String(), edges[0].SourceID)
	assert.Contains(t, edges[0].Attributes, "mitigates")

	// A second live mapping of the same pair is the same edge, as in Reconcile.
	_, err = e.mapping(t, tenant, risk, &req, "again")
	require.NoError(t, err)
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
	res, rerr := controlchain.Reconcile(context.Background(), e.db)
	require.NoError(t, rerr)
	assert.Zero(t, res.Created())
}

func (s mirrorSuite) TestMappingCreate_FrameworkOnlyWritesNoEdge(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	_, err := e.mapping(t, tenant, uuid.New(), nil, "")
	require.NoError(t, err, "no control, no target to resolve, as before")
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
}

func (s mirrorSuite) TestMappingCreate_NotFound_UnknownRiskOrRequirement(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	risk := e.risk(t, tenant)
	unknown := uuid.New()

	_, err := e.mapping(t, tenant, uuid.New(), &req, "")
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.mapping(t, tenant, risk, &unknown, "")
	require.ErrorIs(t, err, domain.ErrNotFound)
	var n int64
	require.NoError(t, e.db.Table("risk_control_mappings").Where("tenant_id = ?", tenant).Count(&n).Error)
	assert.Zero(t, n)
}

func (s mirrorSuite) TestMappingCreate_Unauthorized_NilTenantWritesNothing(t *testing.T) {
	e := s.env(t)
	req := uuid.New()
	_, err := e.mapping(t, uuid.Nil, uuid.New(), &req, "")
	require.Error(t, err)
	assert.Zero(t, e.edgeCount(t, uuid.Nil, canonical.VerbMitigatedBy))
}

func (s mirrorSuite) TestMappingDelete_Success_RemovesTheEdge(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	risk := e.risk(t, tenant)
	m, err := e.mapping(t, tenant, risk, &req, "")
	require.NoError(t, err)
	fw, err := e.mapping(t, tenant, risk, nil, "")
	require.NoError(t, err)

	require.NoError(t, e.maps.Delete(context.Background(), fw.ID, tenant), "deleting a framework-only mapping touches no edge")
	require.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
	require.NoError(t, e.maps.Delete(context.Background(), m.ID, tenant))
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
	e.converged(t)
}

// AC2: another live mapping of the same (risk, control) pair keeps the edge; the
// last one takes it. R6: the edge keeps the first mapping's id as source_id.
func (s mirrorSuite) TestMappingDelete_KeepsTheEdgeWhileAnotherLiveMappingOfThePairRemains(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	risk := e.risk(t, tenant)
	first, err := e.mapping(t, tenant, risk, &req, "")
	require.NoError(t, err)
	second, err := e.mapping(t, tenant, risk, &req, "")
	require.NoError(t, err)

	require.NoError(t, e.maps.Delete(context.Background(), first.ID, tenant))
	edges := e.edges(t, tenant, canonical.VerbMitigatedBy)
	require.Len(t, edges, 1, "the second mapping still asserts the pair")
	assert.Equal(t, first.ID.String(), edges[0].SourceID, "R6: a stale source_id is left as is")

	require.NoError(t, e.maps.Delete(context.Background(), second.ID, tenant))
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
}

func (s mirrorSuite) TestMappingDelete_NotFound_AndCrossTenantKeepTheEdge(t *testing.T) {
	e := s.env(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	req := e.requirement(t, tenantA, "A.5.1")
	risk := e.risk(t, tenantA)
	m, err := e.mapping(t, tenantA, risk, &req, "")
	require.NoError(t, err)

	require.ErrorIs(t, e.maps.Delete(context.Background(), uuid.New(), tenantA), domain.ErrNotFound)
	require.ErrorIs(t, e.maps.Delete(context.Background(), m.ID, tenantB), domain.ErrNotFound)
	assert.Equal(t, 1, e.edgeCount(t, tenantA, canonical.VerbMitigatedBy))
}

func (s mirrorSuite) TestMappingDelete_Unauthorized_NilTenantKeepsTheEdge(t *testing.T) {
	e := s.env(t)
	tenant := uuid.New()
	req := e.requirement(t, tenant, "A.5.1")
	risk := e.risk(t, tenant)
	m, err := e.mapping(t, tenant, risk, &req, "")
	require.NoError(t, err)

	require.Error(t, e.maps.Delete(context.Background(), m.ID, uuid.Nil))
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbMitigatedBy))
}

// ---------------------------------------------------------------------------
// AC3

// TestEdgeWrite_TargetInOtherTenant_404: an edge never names an end of another
// tenant. The write answers ErrNotFound, the legacy row rolls back, and no edge
// or control is left in either tenant.
func (s mirrorSuite) TestEdgeWrite_TargetInOtherTenant_404(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		write func(e *mirrorEnv, a, b uuid.UUID) error
	}{
		{"addressed_by: the split control id is owned by another tenant", func(e *mirrorEnv, a, b uuid.UUID) error {
			reqID := uuid.New()
			foreign := e.splitOf(t, b, reqID)
			require.NoError(t, e.db.Create(&foreign).Error)
			return e.comp.CreateControl(ctx, &domain.ComplianceControl{ID: reqID, TenantID: a, FrameworkID: uuid.New(), Name: "n", Status: domain.ControlStatus("not_implemented")})
		}},
		{"mitigated_by: the risk is another tenant's", func(e *mirrorEnv, a, b uuid.UUID) error {
			req := e.requirement(t, a, "A.5.1")
			_, err := e.mapping(t, a, e.risk(t, b), &req, "")
			return err
		}},
		{"mitigated_by: the control is another tenant's", func(e *mirrorEnv, a, b uuid.UUID) error {
			req := e.requirement(t, b, "A.5.1")
			_, err := e.mapping(t, a, e.risk(t, a), &req, "")
			return err
		}},
		{"addressed_by: the requirement is another tenant's (link of a control of tenant A)", func(e *mirrorEnv, a, b uuid.UUID) error {
			l := e.lib(t)
			ctl := l.newControl(t, a, "A's control")
			_, err := l.link(t, a, ctl.ID, e.requirement(t, b, "B's requirement"))
			return err
		}},
		{"addressed_by: the control is another tenant's (link of a requirement of tenant A)", func(e *mirrorEnv, a, b uuid.UUID) error {
			l := e.lib(t)
			ctl := l.newControl(t, b, "B's control")
			_, err := l.link(t, a, ctl.ID, e.requirement(t, a, "A's requirement"))
			return err
		}},
		{"addressed_by: both ends are another tenant's (tenant A names tenant B's control and requirement)", func(e *mirrorEnv, a, b uuid.UUID) error {
			l := e.lib(t)
			ctl := l.newControl(t, b, "B's control")
			_, err := l.link(t, a, ctl.ID, e.requirement(t, b, "B's requirement"))
			return err
		}},
		{"evidenced_by (control level): the evidence is another tenant's", func(e *mirrorEnv, a, b uuid.UUID) error {
			return e.link(t, a, e.evidence(t, b), e.requirement(t, a, "A.5.1"), "")
		}},
		{"evidenced_by (control level): the control is another tenant's", func(e *mirrorEnv, a, b uuid.UUID) error {
			return e.link(t, a, e.evidence(t, a), e.requirement(t, b, "A.5.1"), "")
		}},
		{"applies_to: the asset is another tenant's (create of an implementation of tenant A's control)", func(e *mirrorEnv, a, b uuid.UUID) error {
			c := e.chain(t)
			ctl := c.newControl(t, a, "A's control")
			impl := userImplementation(a, ctl.ID)
			return c.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, c.asset(t, b))},
				implAudit(a, impl.ID, domain.AuditActionCreate))
		}},
		{"applies_to: one asset is tenant A's, the other tenant B's", func(e *mirrorEnv, a, b uuid.UUID) error {
			c := e.chain(t)
			ctl := c.newControl(t, a, "A's control")
			impl := userImplementation(a, ctl.ID)
			return c.impls.CreateImplementation(ctx, a, impl,
				[]canonical.Edge{appliesEdge(a, impl.ID, c.asset(t, a)), appliesEdge(a, impl.ID, c.asset(t, b))},
				implAudit(a, impl.ID, domain.AuditActionCreate))
		}},
		{"applies_to: the control is another tenant's (implementation of tenant B's control)", func(e *mirrorEnv, a, b uuid.UUID) error {
			c := e.chain(t)
			ctl := c.newControl(t, b, "B's control")
			impl := userImplementation(a, ctl.ID)
			return c.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, c.asset(t, a))},
				implAudit(a, impl.ID, domain.AuditActionCreate))
		}},
		{"applies_to: the asset is another tenant's (replacement of the targets of an implementation)", func(e *mirrorEnv, a, b uuid.UUID) error {
			c := e.chain(t)
			impl := c.newImpl(t, a, c.newControl(t, a, "A's control").ID, c.asset(t, a))
			foreign := c.asset(t, b)
			_, err := c.impls.UpdateImplementation(ctx, a, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
				return cur.Implementation, []canonical.Edge{appliesEdge(a, impl.ID, foreign)}, cur.AppliesTo, implAudit(a, impl.ID, domain.AuditActionUpdate), nil
			})
			// The failed edit left tenant A's own edge in place.
			assert.Equal(t, 1, c.edgeCount(t, a, canonical.VerbAppliesTo))
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := s.env(t)
			a, b := uuid.New(), uuid.New()
			err := tc.write(e, a, b)
			require.ErrorIs(t, err, domain.ErrNotFound)

			var mappings, links int64
			require.NoError(t, e.db.Table("risk_control_mappings").Where("tenant_id = ?", a).Count(&mappings).Error)
			require.NoError(t, e.db.Model(&domain.EvidenceControlLink{}).Where("tenant_id = ?", a).Count(&links).Error)
			assert.Zero(t, mappings+links, "no legacy row of tenant A persisted")
			for _, v := range []canonical.Verb{canonical.VerbMitigatedBy, canonical.VerbEvidencedBy} {
				assert.Zero(t, e.edgeCount(t, a, v), "no %s edge in tenant A", v)
			}
			assert.Zero(t, e.edgeCount(t, b, canonical.VerbMitigatedBy)+e.edgeCount(t, b, canonical.VerbEvidencedBy),
				"nothing was written into tenant B's chain by tenant A's request")
			assert.Zero(t, e.edgeCount(t, b, canonical.VerbAppliesTo), "no applies_to edge in tenant B")
			var impls int64
			require.NoError(t, e.db.Model(&controlchain.ControlImplementationRow{}).Where("tenant_id = ? AND source = 'openrisk_user' AND description = 'Quarterly review' AND status = 'in_place'", a).
				Count(&impls).Error)
			assert.Equal(t, int64(e.edgeCount(t, a, canonical.VerbAppliesTo)), impls, "an implementation of A exists only with its own edge (the replacement case's)")
			// addressed_by: a requirement made by the legacy write carries its split edge and
			// nothing else may exist, in either tenant (AC3, the S2 resolver).
			for _, tenant := range []uuid.UUID{a, b} {
				assert.EqualValues(t, e.requirementCount(t, tenant), e.edgeCount(t, tenant, canonical.VerbAddressedBy),
					"no addressed_by edge beyond the split ones")
			}
		})
	}
}

// splitOf is a control row of the chain owned by a tenant, with the id the split
// of requirement reqID would have.
func (e *mirrorEnv) splitOf(t *testing.T, tenant, reqID uuid.UUID) controlchain.ControlRow {
	t.Helper()
	c, _ := controlchain.SplitControl(controlchain.RequirementRow{
		ID: reqID, TenantID: tenant, Name: "foreign", Status: "implemented", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}, canonical.MethodManualEntry)
	row, err := controlchain.ControlToRow(c, nil)
	require.NoError(t, err)
	return row
}

// ---------------------------------------------------------------------------
// AC4

type riskSnap struct {
	ID                uuid.UUID
	Score, SmartScore float64
	ResidualRisk      *float64
	UpdatedAt         time.Time
}

func (e *mirrorEnv) snapshot(t *testing.T, tenants ...uuid.UUID) (risks []riskSnap, histories int64) {
	t.Helper()
	require.NoError(t, e.db.Table("risks").Select("id, score, smart_score, residual_risk, updated_at").
		Where("tenant_id IN ?", tenants).Order("id").Scan(&risks).Error)
	require.NoError(t, e.db.Table("risk_histories").
		Where("risk_id IN (SELECT id FROM risks WHERE tenant_id IN ?)", tenants).Count(&histories).Error)
	return risks, histories
}

// AC4: no mirrored write changes a risk's Score, SmartScore or ResidualRisk, its
// updated_at, or its history. The mirror reads the risk row and writes only the chain.
func (s mirrorSuite) TestMirror_NeverChangesARiskScore(t *testing.T) {
	e := s.env(t)
	ctx := context.Background()
	tenant := uuid.New()
	risk1, risk2 := e.risk(t, tenant), e.risk(t, tenant)
	beforeRisks, beforeHist := e.snapshot(t, tenant)
	require.Len(t, beforeRisks, 2)
	require.EqualValues(t, 2, beforeHist)

	req := e.requirement(t, tenant, "A.5.1")
	ev := e.evidence(t, tenant)
	require.NoError(t, e.link(t, tenant, ev, req, ""))
	m1, err := e.mapping(t, tenant, risk1, &req, "")
	require.NoError(t, err)
	_, err = e.mapping(t, tenant, risk2, &req, "")
	require.NoError(t, err)
	require.NoError(t, e.maps.Delete(ctx, m1.ID, tenant))
	require.NoError(t, e.ev.Unlink(ctx, tenant, ev, req))
	require.NoError(t, e.ev.Delete(ctx, tenant, ev))

	afterRisks, afterHist := e.snapshot(t, tenant)
	assert.Equal(t, beforeRisks, afterRisks)
	assert.Equal(t, beforeHist, afterHist)
	assert.Equal(t, 7.5, afterRisks[0].Score)
	assert.Equal(t, 63.25, afterRisks[0].SmartScore)
	require.NotNil(t, afterRisks[0].ResidualRisk)
	assert.Equal(t, 2.125, *afterRisks[0].ResidualRisk)
}

// ---------------------------------------------------------------------------
// suite plumbing

type mirrorSuite struct {
	env func(t *testing.T) *mirrorEnv
}

func TestControlChainMirror_SQLite(t *testing.T) {
	runMirrorSuite(t, mirrorSuite{env: newMirrorEnvSQLite})
}

func runMirrorSuite(t *testing.T, s mirrorSuite) {
	for name, fn := range map[string]func(*testing.T){
		"TestCreateControl_Success":                                    s.TestCreateControl_Success,
		"TestCreateControl_NotFound":                                   s.TestCreateControl_NotFound_WhenTheControlIdBelongsToAnotherTenant,
		"TestCreateControl_Unauthorized":                               s.TestCreateControl_Unauthorized_NilTenantWritesNothing,
		"TestCreateControl_RollsBackWhenTheEdgeInsertFails":            s.TestCreateControl_RollsBackWhenTheEdgeInsertFails,
		"TestCreateControl_RefusedByTheChainRollsBack":                 s.TestCreateControl_RefusedByTheChainRollsBack,
		"TestCreateControl_DuplicateReferenceStillConflicts":           s.TestCreateControl_DuplicateReferenceStillConflicts,
		"TestLink_Success":                                             s.TestLink_Success,
		"TestLink_NotFound":                                            s.TestLink_NotFound_UnknownEvidenceOrRequirement,
		"TestLink_Unauthorized":                                        s.TestLink_Unauthorized_NilTenantWritesNothing,
		"TestLink_CreatesTheMissingSplitControl":                       s.TestLink_CreatesTheMissingSplitControl,
		"TestLink_UnsplittableRequirementKeepsTheLegacyWrite":          s.TestLink_UnsplittableRequirementKeepsTheLegacyWrite,
		"TestLink_CredentialNoteKeepsTheLegacyWriteAndWritesNoEdge":    s.TestLink_CredentialNoteKeepsTheLegacyWriteAndWritesNoEdge,
		"TestUnlink_Success":                                           s.TestUnlink_Success,
		"TestUnlink_NotFound":                                          s.TestUnlink_NotFound_IsANoOpAndCrossTenantKeepsTheEdge,
		"TestUnlink_Unauthorized":                                      s.TestUnlink_Unauthorized_NilTenantKeepsTheEdge,
		"TestEvidenceDelete_Success":                                   s.TestEvidenceDelete_Success_RemovesEveryEdgeOfThatEvidenceOnly,
		"TestEvidenceDelete_NotFound":                                  s.TestEvidenceDelete_NotFound_AndCrossTenantKeepEdges,
		"TestEvidenceDelete_Unauthorized":                              s.TestEvidenceDelete_Unauthorized_NilTenantKeepsEdges,
		"TestMappingCreate_Success":                                    s.TestMappingCreate_Success,
		"TestMappingCreate_FrameworkOnlyWritesNoEdge":                  s.TestMappingCreate_FrameworkOnlyWritesNoEdge,
		"TestMappingCreate_NotFound":                                   s.TestMappingCreate_NotFound_UnknownRiskOrRequirement,
		"TestMappingCreate_Unauthorized":                               s.TestMappingCreate_Unauthorized_NilTenantWritesNothing,
		"TestMappingDelete_Success":                                    s.TestMappingDelete_Success_RemovesTheEdge,
		"TestMappingDelete_KeepsTheEdgeWhileAnotherLiveMappingRemains": s.TestMappingDelete_KeepsTheEdgeWhileAnotherLiveMappingOfThePairRemains,
		"TestMappingDelete_NotFound":                                   s.TestMappingDelete_NotFound_AndCrossTenantKeepTheEdge,
		"TestMappingDelete_Unauthorized":                               s.TestMappingDelete_Unauthorized_NilTenantKeepsTheEdge,
		"TestEdgeWrite_TargetInOtherTenant_404":                        s.TestEdgeWrite_TargetInOtherTenant_404,
		"TestMirror_NeverChangesARiskScore":                            s.TestMirror_NeverChangesARiskScore,
		"TestControlLibrary_CreateReadEditDelete_Success":              s.TestControlLibrary_CreateReadEditDelete_Success,
		"TestControlLibrary_PendingFlagClearIsWritten":                 s.TestControlLibrary_PendingFlagClearIsWritten,
		"TestControlLibrary_Unauthorized":                              s.TestControlLibrary_Unauthorized_NilTenantTouchesNothing,
		"TestControlLibrary_CrossTenantIsInvisible":                    s.TestControlLibrary_CrossTenantIsInvisible,
		"TestControlLibrary_AuditFailureRollsTheWriteBack":             s.TestControlLibrary_AuditFailureRollsTheWriteBack,
		"TestControlLibrary_OneControlTwoRequirementsReadBackFromBoth": s.TestControlLibrary_OneControlTwoRequirementsReadBackFromBoth,
		"TestControlLibrary_ReadsFilterOnTheLivenessOfBothEnds":        s.TestControlLibrary_ReadsFilterOnTheLivenessOfBothEnds,
		"TestControlLibrary_UnlinkDeletesOnlyThatEdge":                 s.TestControlLibrary_UnlinkDeletesOnlyThatEdge,
		"TestControlLibrary_EvidenceProbe":                             s.TestControlLibrary_EvidenceProbe,
		"TestControlImplementation_CreateReadEditDelete_Success":       s.TestControlImplementation_CreateReadEditDelete_Success,
		"TestControlImplementation_NotFound":                           s.TestControlImplementation_NotFound,
		"TestControlImplementation_Unauthorized":                       s.TestControlImplementation_Unauthorized_NilTenantTouchesNothing,
		"TestControlImplementation_CrossTenantIsInvisible":             s.TestControlImplementation_CrossTenantIsInvisible,
		"TestControlImplementation_AuditFailureRollsTheWriteBack":      s.TestControlImplementation_AuditFailureRollsTheWriteBack,
		"TestControlImplementation_ParentChainLiveness":                s.TestControlImplementation_ReadsFilterOnTheLivenessOfTheParentChain,
		"TestControlTest_CreateReadEditDelete_Success":                 s.TestControlTest_CreateReadEditDelete_Success,
		"TestControlTest_NotFound":                                     s.TestControlTest_NotFound,
		"TestControlTest_Unauthorized":                                 s.TestControlTest_Unauthorized_NilTenantTouchesNothing,
		"TestControlTest_CrossTenantIsInvisible":                       s.TestControlTest_CrossTenantIsInvisible,
		"TestControlTest_AuditFailureRollsTheWriteBack":                s.TestControlTest_AuditFailureRollsTheWriteBack,
	} {
		t.Run(name, fn)
	}
}
