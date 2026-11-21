// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
)

func importFixture(t *testing.T) (*GormComplianceRepository, uuid.UUID, *domain.ComplianceFramework) {
	t.Helper()
	repo := setupComplianceRepo(t)
	require.NoError(t, repo.db.AutoMigrate(&domain.ControlCrosswalk{}))
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "ISO", Version: "2022"}
	require.NoError(t, repo.CreateFramework(context.Background(), fw))
	return repo, tenant, fw
}

func packControl(code string) *domain.ComplianceControl {
	base, hash := `{"name":"`+code+`"}`, "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	return &domain.ComplianceControl{
		ID: uuid.New(), ReferenceCode: code, Name: code,
		PackKey: "iso", PackItemKey: code, PackVersion: "1.0.0", PackBase: &base, PackBaseHash: &hash,
	}
}

func TestApplyCatalogImport_Success(t *testing.T) {
	repo, tenant, fw := importFixture(t)
	ctx := context.Background()
	a, b := packControl("A.1"), packControl("A.2")
	other := &domain.ControlCrosswalk{SourceControlID: a.ID, TargetControlID: b.ID, Coverage: domain.CrosswalkCoverage("full"),
		Origin: domain.CrosswalkOriginCurated, PackKey: "crosswalks", PackItemKey: "iso:A.1|iso:A.2", PackVersion: "1.0.0"}

	err := repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{
		TenantID: tenant, FrameworkID: fw.ID, CatalogKey: "iso",
		Stamp:    &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"},
		Controls: []*domain.ComplianceControl{a, b}, Links: []*domain.ControlCrosswalk{other},
	})
	require.NoError(t, err)

	got, err := repo.GetFrameworkByID(ctx, fw.ID, tenant)
	require.NoError(t, err)
	assert.Equal(t, "iso", got.CatalogKey)
	assert.Equal(t, "1.0.0", got.CatalogContentVersion)
	assert.Equal(t, "1.0.0", got.CatalogCrosswalksVersion)
	assert.Equal(t, "[]", got.CatalogPendingKinds)
	ctls, err := repo.ListControlsByFramework(ctx, tenant, fw.ID)
	require.NoError(t, err)
	require.Len(t, ctls, 2)
	for _, c := range ctls {
		assert.Equal(t, tenant, c.TenantID)
		assert.Equal(t, "iso", c.PackKey)
		require.NotNil(t, c.PackBaseHash)
	}
	assert.EqualValues(t, 2, tableCount(t, repo.db, controlchain.TableControls), "mirrored on the chain")
	var link domain.ControlCrosswalk
	require.NoError(t, repo.db.Where("tenant_id = ?", tenant).First(&link).Error)
	assert.Equal(t, "crosswalks", link.PackKey)
}

func TestApplyCatalogImport_Unauthorized_NilTenant(t *testing.T) {
	repo, _, fw := importFixture(t)
	err := repo.ApplyCatalogImport(context.Background(), &domain.CatalogImportPlan{
		FrameworkID: fw.ID, Controls: []*domain.ComplianceControl{packControl("A.1")}})
	require.ErrorIs(t, err, domain.ErrForbidden)
	assert.Zero(t, tableCount(t, repo.db, "compliance_controls"))
	_, err = repo.ListControlsByFrameworkIncludingDeleted(context.Background(), uuid.Nil, fw.ID)
	require.Error(t, err)
}

func TestApplyCatalogImport_NotFound_ForeignFramework(t *testing.T) {
	repo, _, fw := importFixture(t)
	err := repo.ApplyCatalogImport(context.Background(), &domain.CatalogImportPlan{
		TenantID: uuid.New(), FrameworkID: fw.ID, CatalogKey: "iso",
		Stamp:    &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"},
		Controls: []*domain.ComplianceControl{packControl("A.1")}})
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Zero(t, tableCount(t, repo.db, "compliance_controls"))
	var got domain.ComplianceFramework
	require.NoError(t, repo.db.First(&got, "id = ?", fw.ID).Error)
	assert.Empty(t, got.CatalogContentVersion, "the owner's framework was not stamped")
	assert.Empty(t, got.CatalogKey)
}

func TestApplyCatalogImport_EmptyPlanWritesNothing(t *testing.T) {
	repo, tenant, fw := importFixture(t)
	ctx := context.Background()
	before, err := repo.GetFrameworkByID(ctx, fw.ID, tenant)
	require.NoError(t, err)
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID}))
	after, err := repo.GetFrameworkByID(ctx, fw.ID, tenant)
	require.NoError(t, err)
	assert.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "updated_at must not move")
}

// Criterion 8: any step failing leaves nothing behind.
func TestApplyCatalogImport_RollsBackOnInjectedFailure(t *testing.T) {
	for _, failAt := range []string{"second_control", "link"} {
		t.Run(failAt, func(t *testing.T) {
			repo, tenant, fw := importFixture(t)
			db := repo.db
			boom := errors.New("injected write failure")
			calls := 0
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_catalog_import", func(tx *gorm.DB) {
				switch {
				case failAt == "second_control" && tx.Statement.Table == "compliance_controls":
					calls++
					if calls == 2 {
						_ = tx.AddError(boom)
					}
				case failAt == "link" && tx.Statement.Table == "control_crosswalks":
					_ = tx.AddError(boom)
				}
			}))
			t.Cleanup(func() { _ = db.Callback().Create().Remove("test:fail_catalog_import") })

			a, b := packControl("A.1"), packControl("A.2")
			err := repo.ApplyCatalogImport(context.Background(), &domain.CatalogImportPlan{
				TenantID: tenant, FrameworkID: fw.ID, CatalogKey: "iso",
				Stamp:    &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"},
				Controls: []*domain.ComplianceControl{a, b},
				Links:    []*domain.ControlCrosswalk{{SourceControlID: a.ID, TargetControlID: b.ID, Coverage: "full", Origin: domain.CrosswalkOriginCurated}},
			})
			require.ErrorIs(t, err, boom)
			assert.Zero(t, tableCount(t, db, "compliance_controls"))
			assert.Zero(t, tableCount(t, db, "control_crosswalks"))
			assert.Zero(t, tableCount(t, db, controlchain.TableControls))
			assert.Zero(t, tableCount(t, db, controlchain.TableCanonicalEdges))
			var got domain.ComplianceFramework
			require.NoError(t, db.First(&got, "id = ?", fw.ID).Error)
			assert.Empty(t, got.CatalogContentVersion, "no framework stamp remains")
			assert.Empty(t, got.CatalogKey)
		})
	}
}

// Criteria 5 and 12: tombstones are visible only inside their tenant+framework,
// and the unique index is per tenant.
func TestListControlsIncludingDeleted_TenantScoped_AndIndexPerTenant(t *testing.T) {
	repo, tenantA, fwA := importFixture(t)
	ctx := context.Background()
	tenantB := uuid.New()
	fwB := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenantB, Name: "ISO", Version: "2022"}
	require.NoError(t, repo.CreateFramework(ctx, fwB))

	ctlA := packControl("A.1")
	ctlB := packControl("A.1")
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenantA, FrameworkID: fwA.ID, Controls: []*domain.ComplianceControl{ctlA}}))
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenantB, FrameworkID: fwB.ID, Controls: []*domain.ComplianceControl{ctlB}}),
		"the same pack item under another tenant is another row")
	require.NoError(t, repo.DeleteControl(ctx, ctlA.ID, tenantA))

	live, err := repo.ListControlsByFramework(ctx, tenantA, fwA.ID)
	require.NoError(t, err)
	assert.Empty(t, live)
	all, err := repo.ListControlsByFrameworkIncludingDeleted(ctx, tenantA, fwA.ID)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.True(t, all[0].DeletedAt.Valid)
	assert.Equal(t, "A.1", all[0].PackItemKey)

	// Tenant B's deleted row is invisible to A, and A's to B.
	require.NoError(t, repo.DeleteControl(ctx, ctlB.ID, tenantB))
	cross, err := repo.ListControlsByFrameworkIncludingDeleted(ctx, tenantB, fwA.ID)
	require.NoError(t, err)
	assert.Empty(t, cross, "another tenant cannot read this framework's tombstones")

	// The index includes deleted rows: re-inserting the same item in the same framework conflicts.
	err = repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenantA, FrameworkID: fwA.ID, Controls: []*domain.ComplianceControl{packControl("A.1")}})
	require.ErrorIs(t, err, domain.ErrConflict)
}

// A hand-made row (empty pack_key) is outside the index (criterion 7).
func TestApplyCatalogImport_IndexIgnoresHandMadeRows(t *testing.T) {
	repo, tenant, fw := importFixture(t)
	ctx := context.Background()
	h1 := &domain.ComplianceControl{ID: uuid.New(), ReferenceCode: "X.1", Name: "one"}
	h2 := &domain.ComplianceControl{ID: uuid.New(), ReferenceCode: "X.2", Name: "two"}
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Controls: []*domain.ComplianceControl{h1, h2}}))
}

// Criterion 10: the framework update path cannot write the stamp.
func TestUpdateFramework_CannotWriteStamp(t *testing.T) {
	repo, tenant, fw := importFixture(t)
	ctx := context.Background()
	fw.Name = "Renamed"
	fw.CatalogContentVersion, fw.CatalogCrosswalksVersion, fw.CatalogPendingKinds = "9.9.9", "9.9.9", `["x"]`
	require.NoError(t, repo.UpdateFramework(ctx, fw))
	got, err := repo.GetFrameworkByID(ctx, fw.ID, tenant)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.Empty(t, got.CatalogContentVersion)
	assert.Empty(t, got.CatalogCrosswalksVersion)
	assert.Equal(t, "[]", got.CatalogPendingKinds)
}

// Criterion 10: UpdateControl cannot write provenance either.
func TestUpdateControl_CannotWriteProvenance(t *testing.T) {
	repo, tenant, fw := importFixture(t)
	ctx := context.Background()
	c := packControl("A.1")
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Controls: []*domain.ComplianceControl{c}}))
	forged := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	upd := *c
	upd.Name, upd.PackKey, upd.PackItemKey, upd.PackVersion, upd.PackBaseHash, upd.PackRetiredIn = "Edited", "x", "x", "9.9.9", &forged, "9.9.9"
	require.NoError(t, repo.UpdateControl(ctx, &upd))
	var got domain.ComplianceControl
	require.NoError(t, repo.db.First(&got, "id = ?", c.ID).Error)
	assert.Equal(t, "Edited", got.Name)
	assert.Equal(t, "iso", got.PackKey)
	assert.Equal(t, "A.1", got.PackItemKey)
	assert.Equal(t, "1.0.0", got.PackVersion)
	assert.NotEqual(t, forged, *got.PackBaseHash)
	assert.Empty(t, got.PackRetiredIn)
}

// Link ends are verified inside the transaction: tenant, liveness, framework.
func TestApplyCatalogImport_LinkEnds_Refused(t *testing.T) {
	linkTo := func(src, tgt uuid.UUID) *domain.ControlCrosswalk {
		return &domain.ControlCrosswalk{SourceControlID: src, TargetControlID: tgt, Coverage: "full", Origin: domain.CrosswalkOriginCurated}
	}
	assertNothing := func(t *testing.T, repo *GormComplianceRepository, fw *domain.ComplianceFramework, controls int64) {
		assert.Zero(t, tableCount(t, repo.db, "control_crosswalks"), "no link written")
		assert.Equal(t, controls, tableCount(t, repo.db, "compliance_controls"), "no new control written")
		var got domain.ComplianceFramework
		require.NoError(t, repo.db.First(&got, "id = ?", fw.ID).Error)
		assert.Empty(t, got.CatalogContentVersion, "no stamp")
	}
	stamp := &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"}

	t.Run("foreign tenant control", func(t *testing.T) {
		repo, tenant, fw := importFixture(t)
		ctx := context.Background()
		otherTenant := uuid.New()
		otherFw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: otherTenant, Name: "X", Version: "1"}
		require.NoError(t, repo.CreateFramework(ctx, otherFw))
		foreign := packControl("F.1")
		require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: otherTenant, FrameworkID: otherFw.ID, Controls: []*domain.ComplianceControl{foreign}}))

		a := packControl("A.1")
		err := repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Stamp: stamp,
			Controls: []*domain.ComplianceControl{a}, Links: []*domain.ControlCrosswalk{linkTo(a.ID, foreign.ID)}})
		require.ErrorIs(t, err, domain.ErrNotFound)
		assertNothing(t, repo, fw, 1) // only the other tenant's control exists
	})

	t.Run("deleted end", func(t *testing.T) {
		repo, tenant, fw := importFixture(t)
		ctx := context.Background()
		dead := packControl("D.1")
		require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Controls: []*domain.ComplianceControl{dead}}))
		require.NoError(t, repo.DeleteControl(ctx, dead.ID, tenant))

		a := packControl("A.1")
		err := repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Stamp: stamp,
			Controls: []*domain.ComplianceControl{a}, Links: []*domain.ControlCrosswalk{linkTo(a.ID, dead.ID)}})
		require.ErrorIs(t, err, domain.ErrNotFound)
		assertNothing(t, repo, fw, 1) // the deleted row only
	})

	t.Run("no end in the framework", func(t *testing.T) {
		repo, tenant, fw := importFixture(t)
		ctx := context.Background()
		fw2 := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "Two", Version: "1"}
		require.NoError(t, repo.CreateFramework(ctx, fw2))
		x, y := packControl("X.1"), packControl("Y.1")
		require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw2.ID, Controls: []*domain.ComplianceControl{x, y}}))

		err := repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID, Stamp: stamp,
			Links: []*domain.ControlCrosswalk{linkTo(x.ID, y.ID)}})
		require.ErrorIs(t, err, domain.ErrNotFound)
		assertNothing(t, repo, fw, 2)
	})

	t.Run("accepted: one end created here, one existing in another framework", func(t *testing.T) {
		repo, tenant, fw := importFixture(t)
		ctx := context.Background()
		fw2 := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "Two", Version: "1"}
		require.NoError(t, repo.CreateFramework(ctx, fw2))
		x := packControl("X.1")
		require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw2.ID, Controls: []*domain.ComplianceControl{x}}))
		a := packControl("A.1")
		require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenant, FrameworkID: fw.ID,
			Controls: []*domain.ComplianceControl{a}, Links: []*domain.ControlCrosswalk{linkTo(a.ID, x.ID)}}))
		assert.EqualValues(t, 1, tableCount(t, repo.db, "control_crosswalks"))
	})
}
