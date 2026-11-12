// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// legacyDB is an in-memory database with the legacy tables Reconcile reads,
// as the columns it reads, and the six tables built from the row types.
func legacyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), TranslateError: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // one in-memory database for every statement
	for _, ddl := range []string{
		`CREATE TABLE compliance_controls (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT,
			status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE evidence_control_links (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, evidence_id TEXT NOT NULL,
			control_id TEXT NOT NULL, note TEXT, created_at DATETIME)`,
		`CREATE TABLE risk_control_mappings (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, risk_id TEXT NOT NULL,
			control_id TEXT, note TEXT, created_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE evidences (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL)`,
		`CREATE TABLE risks (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, db.AutoMigrate(Models()...))
	return db
}

type fixture struct {
	db *gorm.DB

	reqA, reqNA, reqDeleted, reqB uuid.UUID
	evA, evB                      uuid.UUID
	riskA, riskB                  uuid.UUID

	linkOK, linkCrossTenant, linkOrphan                           uuid.UUID
	mapOK, mapDup, mapFrameworkOnly, mapDeleted, mapOrphan, mapXT uuid.UUID
}

func seed(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{db: legacyDB(t)}
	id := uuid.New
	f.reqA, f.reqNA, f.reqDeleted, f.reqB = id(), id(), id(), id()
	f.evA, f.evB, f.riskA, f.riskB = id(), id(), id(), id()
	f.linkOK, f.linkCrossTenant, f.linkOrphan = id(), id(), id()
	f.mapOK, f.mapDup, f.mapFrameworkOnly, f.mapDeleted, f.mapOrphan, f.mapXT = id(), id(), id(), id(), id(), id()

	exec := func(q string, args ...any) { t.Helper(); require.NoError(t, f.db.Exec(q, args...).Error) }
	ctl := func(rid, tenant uuid.UUID, name, status string, deleted any) {
		exec(`INSERT INTO compliance_controls (id, tenant_id, name, description, status, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, 'd', ?, ?, ?, ?)`, rid, tenant, name, status, t0, t0, deleted)
	}
	ctl(f.reqA, tenantA, "A.5.15 Access control", "implemented", nil)
	ctl(f.reqNA, tenantA, "Physical media", "not_applicable", nil)
	ctl(f.reqDeleted, tenantA, "Retired", "in_progress", t0)
	ctl(f.reqB, tenantB, "CC6.1", "not_implemented", nil)

	exec(`INSERT INTO evidences (id, tenant_id) VALUES (?, ?), (?, ?)`, f.evA, tenantA, f.evB, tenantB)
	exec(`INSERT INTO risks (id, tenant_id) VALUES (?, ?), (?, ?)`, f.riskA, tenantA, f.riskB, tenantB)

	link := func(lid, tenant, ev, ctrl uuid.UUID) {
		exec(`INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, note, created_at) VALUES (?, ?, ?, ?, 'because', ?)`,
			lid, tenant, ev, ctrl, t0)
	}
	link(f.linkOK, tenantA, f.evA, f.reqA)
	link(f.linkCrossTenant, tenantA, f.evB, f.reqA) // tenant A's link to tenant B's evidence
	link(f.linkOrphan, tenantA, f.evA, uuid.New())  // control row does not exist

	mapping := func(mid, tenant, risk uuid.UUID, ctrl any, deleted any) {
		exec(`INSERT INTO risk_control_mappings (id, tenant_id, risk_id, control_id, note, created_at, deleted_at) VALUES (?, ?, ?, ?, '', ?, ?)`,
			mid, tenant, risk, ctrl, t0, deleted)
	}
	mapping(f.mapOK, tenantA, f.riskA, f.reqA, nil)
	mapping(f.mapDup, tenantA, f.riskA, f.reqA, nil) // same (risk, control) pair
	mapping(f.mapFrameworkOnly, tenantA, f.riskA, nil, nil)
	mapping(f.mapDeleted, tenantA, f.riskA, f.reqNA, t0)
	mapping(f.mapOrphan, tenantA, uuid.New(), f.reqA, nil) // risk does not exist
	mapping(f.mapXT, tenantA, f.riskA, f.reqB, nil)        // control belongs to tenant B
	return f
}

func (f *fixture) edges(t *testing.T, verb canonical.Verb) []CanonicalEdgeRow {
	t.Helper()
	var rows []CanonicalEdgeRow
	require.NoError(t, f.db.Where("verb = ?", string(verb)).Find(&rows).Error)
	return rows
}

func (f *fixture) count(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Table(table).Count(&n).Error)
	return n
}

// TestReconcile_Success: every row of the three legacy tables is accounted for,
// by the rules of D-108 Q7 and Q8.
func TestReconcile_Success(t *testing.T) {
	f := seed(t)
	res, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)

	assert.Equal(t, int64(4), res.Requirements)
	assert.Equal(t, 4, res.ControlsCreated)
	assert.Equal(t, 1, res.ApplicabilityFlagged)
	assert.Equal(t, int64(4), f.count(t, TableControls), "one control per compliance_controls row, deleted included")

	var na ControlRow
	require.NoError(t, f.db.Where("id = ? AND tenant_id = ?", canonical.ControlIDFor(f.reqNA), tenantA).First(&na).Error)
	assert.Equal(t, "not_applicable", na.Applicability)
	assert.True(t, na.ApplicabilityReviewPending)
	assert.Nil(t, na.ApplicabilityJustification)
	assert.Equal(t, "not_implemented", na.DeclaredStatus)

	var gone ControlRow
	require.NoError(t, f.db.Where("id = ? AND tenant_id = ?", canonical.ControlIDFor(f.reqDeleted), tenantA).First(&gone).Error)
	require.NotNil(t, gone.DeletedAt, "a soft-deleted requirement has a soft-deleted control")

	var ok ControlRow
	require.NoError(t, f.db.Where("id = ? AND tenant_id = ?", canonical.ControlIDFor(f.reqA), tenantA).First(&ok).Error)
	assert.Equal(t, "implemented", ok.DeclaredStatus)
	assert.Equal(t, canonical.ControlIDFor(f.reqA).String(), ok.SourceID)

	addressed := f.edges(t, canonical.VerbAddressedBy)
	assert.Len(t, addressed, 4)
	assert.Equal(t, 4, res.AddressedByCreated)
	for _, e := range addressed {
		assert.Equal(t, string(canonical.TypeRequirement), e.FromType)
		assert.Contains(t, e.Attributes, `"full"`)
	}

	assert.Equal(t, int64(3), res.Links)
	assert.Equal(t, 1, res.LinksCarried)
	assert.ElementsMatch(t, []uuid.UUID{f.linkCrossTenant, f.linkOrphan}, res.LinksUnplaceable)
	ev := f.edges(t, canonical.VerbEvidencedBy)
	require.Len(t, ev, 1)
	assert.Equal(t, canonical.ControlIDFor(f.reqA), ev[0].FromID)
	assert.Equal(t, f.evA, ev[0].ToID)
	assert.Equal(t, tenantA, ev[0].TenantID)
	assert.Contains(t, ev[0].Attributes, "because")

	assert.Equal(t, int64(6), res.Mappings)
	assert.Equal(t, 1, res.MappingsCarried)
	// Both rows share created_at, so which of the pair is the one carried is the
	// id order: the other is listed as merged, and together they are the pair.
	require.Len(t, res.MappingsCollapsed, 1)
	assert.Equal(t, 1, res.MappingsFrameworkOnly)
	assert.Equal(t, 1, res.MappingsSoftDeleted)
	assert.ElementsMatch(t, []uuid.UUID{f.mapOrphan, f.mapXT}, res.MappingsUnplaceable)
	mit := f.edges(t, canonical.VerbMitigatedBy)
	require.Len(t, mit, 1)
	assert.Equal(t, f.riskA, mit[0].FromID)
	assert.Equal(t, canonical.ControlIDFor(f.reqA), mit[0].ToID)
	assert.ElementsMatch(t, []string{f.mapOK.String(), f.mapDup.String()}, []string{mit[0].SourceID, res.MappingsCollapsed[0].String()})

	// The legacy rows are not touched: counts are what they were.
	assert.Equal(t, int64(4), f.count(t, "compliance_controls"))
	assert.Equal(t, int64(3), f.count(t, "evidence_control_links"))
	assert.Equal(t, int64(6), f.count(t, "risk_control_mappings"))
}

// TestReconcile_NotFound: an end that does not exist is listed, never carried,
// and never creates a row.
func TestReconcile_NotFound(t *testing.T) {
	f := seed(t)
	res, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	assert.Contains(t, res.LinksUnplaceable, f.linkOrphan)
	assert.Contains(t, res.MappingsUnplaceable, f.mapOrphan)
	for _, verb := range []canonical.Verb{canonical.VerbEvidencedBy, canonical.VerbMitigatedBy} {
		for _, e := range f.edges(t, verb) {
			assert.NotEqual(t, f.mapOrphan.String(), e.SourceID)
			assert.NotEqual(t, f.linkOrphan.String(), e.SourceID)
		}
	}
}

// TestReconcile_Unauthorized: a link or mapping whose other end belongs to
// another tenant is not carried: no edge ever crosses tenants.
func TestReconcile_Unauthorized(t *testing.T) {
	f := seed(t)
	res, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	assert.Contains(t, res.LinksUnplaceable, f.linkCrossTenant)
	assert.Contains(t, res.MappingsUnplaceable, f.mapXT)

	var edges []CanonicalEdgeRow
	require.NoError(t, f.db.Find(&edges).Error)
	require.NotEmpty(t, edges)
	owner := map[uuid.UUID]uuid.UUID{}
	for _, tn := range []struct{ id, tenant uuid.UUID }{
		{f.reqA, tenantA}, {f.reqNA, tenantA}, {f.reqDeleted, tenantA}, {f.reqB, tenantB},
		{canonical.ControlIDFor(f.reqA), tenantA}, {canonical.ControlIDFor(f.reqNA), tenantA},
		{canonical.ControlIDFor(f.reqDeleted), tenantA}, {canonical.ControlIDFor(f.reqB), tenantB},
		{f.evA, tenantA}, {f.evB, tenantB}, {f.riskA, tenantA}, {f.riskB, tenantB},
	} {
		owner[tn.id] = tn.tenant
	}
	for _, e := range edges {
		assert.Equal(t, e.TenantID, owner[e.FromID], "from end of %s is in the edge's tenant", e.Verb)
		assert.Equal(t, e.TenantID, owner[e.ToID], "to end of %s is in the edge's tenant", e.Verb)
	}
}

// TestReconcile_ZeroTenantRowIsNotCarried: a legacy row with the nil tenant is
// never given a control edge (fail closed, never uuid.Nil).
func TestReconcile_ZeroTenantRowIsNotCarried(t *testing.T) {
	f := seed(t)
	require.NoError(t, f.db.Exec(`INSERT INTO evidences (id, tenant_id) VALUES (?, ?)`, uuid.New(), uuid.Nil).Error)
	orphan := uuid.New()
	require.NoError(t, f.db.Exec(`INSERT INTO evidence_control_links (id, tenant_id, evidence_id, control_id, note, created_at)
		VALUES (?, ?, ?, ?, '', ?)`, orphan, uuid.Nil, f.evA, f.reqA, t0).Error)
	res, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	assert.Contains(t, res.LinksUnplaceable, orphan)
}

// TestReconcile_Idempotent: a second run inserts nothing, and one that follows
// a new legacy row inserts only that row's chain.
func TestReconcile_Idempotent(t *testing.T) {
	f := seed(t)
	first, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	require.Positive(t, first.Created())
	controls, edges := f.count(t, TableControls), f.count(t, TableCanonicalEdges)

	second, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	assert.Equal(t, 0, second.Created())
	assert.Equal(t, 4, second.ControlsAlreadyThere)
	assert.Equal(t, controls, f.count(t, TableControls))
	assert.Equal(t, edges, f.count(t, TableCanonicalEdges))
	assert.Equal(t, first.LinksCarried, second.LinksCarried)
	assert.Equal(t, first.MappingsCarried, second.MappingsCarried)

	late := uuid.New()
	require.NoError(t, f.db.Exec(`INSERT INTO compliance_controls (id, tenant_id, name, description, status, created_at, updated_at)
		VALUES (?, ?, 'late', 'd', 'in_progress', ?, ?)`, late, tenantB, t0, t0).Error)
	third, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	assert.Equal(t, 1, third.ControlsCreated)
	assert.Equal(t, 1, third.AddressedByCreated)
	assert.Equal(t, 2, third.Created())
}

// TestReconcile_NeverRewritesExistingControl: insert-only. A control the user
// has edited since is left as it is by a later run (the #1001 lesson).
func TestReconcile_NeverRewritesExistingControl(t *testing.T) {
	f := seed(t)
	_, err := Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	id := canonical.ControlIDFor(f.reqA)
	require.NoError(t, f.db.Exec(`UPDATE controls SET name = 'Renamed by the user', declared_status = 'in_progress' WHERE id = ? AND tenant_id = ?`, id, tenantA).Error)
	require.NoError(t, f.db.Exec(`UPDATE compliance_controls SET name = 'changed upstream', status = 'not_implemented' WHERE id = ? AND tenant_id = ?`, f.reqA, tenantA).Error)

	_, err = Reconcile(context.Background(), f.db)
	require.NoError(t, err)
	var c ControlRow
	require.NoError(t, f.db.Where("id = ? AND tenant_id = ?", id, tenantA).First(&c).Error)
	assert.Equal(t, "Renamed by the user", c.Name)
	assert.Equal(t, "in_progress", c.DeclaredStatus)
}

// TestReconcile_FailsWhenCountsDoNotReconcile: a control that occupies a split
// control's identity under another id stops the boot and rolls everything back.
func TestReconcile_FailsWhenCountsDoNotReconcile(t *testing.T) {
	f := seed(t)
	squatter, _ := SplitControl(RequirementRow{ID: f.reqA, TenantID: tenantA, Name: "x", Status: "implemented", CreatedAt: t0, UpdatedAt: t0}, canonical.MethodMigration)
	row, err := ControlToRow(squatter, nil)
	require.NoError(t, err)
	row.ID = uuid.New() // same (tenant, source, source_id), different id
	require.NoError(t, f.db.Create(&row).Error)

	_, err = Reconcile(context.Background(), f.db)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotReconciled)
	assert.Contains(t, err.Error(), "nothing was written")
	assert.Equal(t, int64(1), f.count(t, TableControls), "the transaction rolled back")
	assert.Equal(t, int64(0), f.count(t, TableCanonicalEdges))
}

// TestReconcileAtBoot_NotReconciledLetsTheBootGoOn: a split defect rolls back,
// is logged, and does not stop the boot (D-108 amended 2026-11-12).
func TestReconcileAtBoot_NotReconciledLetsTheBootGoOn(t *testing.T) {
	f := seed(t)
	squatter, _ := SplitControl(RequirementRow{ID: f.reqA, TenantID: tenantA, Name: "x", Status: "implemented", CreatedAt: t0, UpdatedAt: t0}, canonical.MethodMigration)
	row, err := ControlToRow(squatter, nil)
	require.NoError(t, err)
	row.ID = uuid.New()
	require.NoError(t, f.db.Create(&row).Error)
	legacyBefore := f.count(t, "compliance_controls")

	require.NoError(t, ReconcileAtBoot(context.Background(), f.db))
	assert.Equal(t, int64(1), f.count(t, TableControls), "nothing was written")
	assert.Equal(t, int64(0), f.count(t, TableCanonicalEdges))
	assert.Equal(t, legacyBefore, f.count(t, "compliance_controls"), "the legacy table is untouched")
}

// TestReconcileAtBoot_DatabaseErrorStopsTheBoot: an unreadable source table is
// not a split defect, so the boot stops on it.
func TestReconcileAtBoot_DatabaseErrorStopsTheBoot(t *testing.T) {
	f := seed(t)
	require.NoError(t, f.db.Migrator().DropTable("evidence_control_links"))

	err := ReconcileAtBoot(context.Background(), f.db)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotReconciled)
	assert.Equal(t, int64(0), f.count(t, TableControls), "the transaction rolled back")
}

// TestCanonical_IdentityKeyIncludesTenant: (tenant_id, source, source_id) is the
// identity, so the same source and source_id in two tenants are two controls.
func TestCanonical_IdentityKeyIncludesTenant(t *testing.T) {
	db := legacyDB(t)
	mk := func(tenant uuid.UUID) ControlRow {
		c, _ := SplitControl(RequirementRow{ID: uuid.New(), TenantID: tenant, Name: "Shared", Status: "implemented", CreatedAt: t0, UpdatedAt: t0}, canonical.MethodMigration)
		c.SourceID = "ERP-AC-001" // the same external key in both tenants
		row, err := ControlToRow(c, nil)
		require.NoError(t, err)
		return row
	}
	a, b := mk(tenantA), mk(tenantB)
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, db.Create(&b).Error, "the same source and source_id in another tenant is not a conflict")

	dup := mk(tenantA)
	err := db.Create(&dup).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrDuplicatedKey, "inside one tenant it is")

	var n int64
	require.NoError(t, db.Model(&ControlRow{}).Where("source = ? AND source_id = ?", "openrisk_user", "ERP-AC-001").Count(&n).Error)
	assert.Equal(t, int64(2), n)
}

// TestControlRow_ChecksRefuseBadRows: an AutoMigrate-only database carries the
// constraints too.
func TestControlRow_ChecksRefuseBadRows(t *testing.T) {
	db := legacyDB(t)
	base := func() ControlRow {
		c, _ := SplitControl(requirement(tenantA, "implemented"), canonical.MethodMigration)
		r, err := ControlToRow(c, nil)
		require.NoError(t, err)
		return r
	}
	bad := base()
	bad.Applicability, bad.ApplicabilityReviewPending = "not_applicable", false
	assert.Error(t, db.Create(&bad).Error, "not_applicable needs a justification or the pending flag")

	bad = base()
	bad.DeclaredStatus = "not_applicable"
	assert.Error(t, db.Create(&bad).Error)

	bad = base()
	bad.Confidence = 1.5
	assert.Error(t, db.Create(&bad).Error)

	var edge CanonicalEdgeRow
	e := Edge(tenantA, canonical.TypeRisk, uuid.New(), canonical.VerbMitigatedBy, canonical.TypeControl, uuid.New(), canonical.EdgeAttributes{}, "s", t0)
	edge, err := EdgeToRow(e)
	require.NoError(t, err)
	edge.Verb = "implemented_by" // a field, never a row (A10.4)
	assert.Error(t, db.Create(&edge).Error)

	ok := base()
	require.NoError(t, db.Create(&ok).Error)
}
