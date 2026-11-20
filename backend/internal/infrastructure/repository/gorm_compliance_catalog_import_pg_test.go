// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

// #1483 on PostgreSQL: jsonb and char(64) columns, the partial unique index and
// the single transaction run on the engine that matters. Skips without DATABASE_URL.
func newCatalogImportPG(t *testing.T) *GormComplianceRepository {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true}
	admin, err := testdb.Open(t, postgres.Open(dsn), cfg)
	require.NoError(t, err)
	schema := "catimp1483_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		if s, err := admin.DB(); err == nil {
			_ = s.Close()
		}
	})
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := testdb.Open(t, postgres.Open(dsn+sep+"search_path="+schema), cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&domain.ComplianceFramework{}, &domain.ComplianceControl{}, &domain.ControlCrosswalk{}))
	require.NoError(t, db.AutoMigrate(controlchain.Models()...))
	return NewGormComplianceRepository(db)
}

func TestApplyCatalogImport_Postgres(t *testing.T) {
	repo := newCatalogImportPG(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	fwA := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenantA, Name: "ISO", Version: "2022"}
	fwB := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenantB, Name: "ISO", Version: "2022"}
	require.NoError(t, repo.CreateFramework(ctx, fwA))
	require.NoError(t, repo.CreateFramework(ctx, fwB))

	mk := func(code string) *domain.ComplianceControl {
		cc := pkgcompliance.CatalogControl{ReferenceCode: code, Name: code, Description: "d", SourceReference: "s"}
		pb := pkgcompliance.RequirementPackBase(cc)
		return &domain.ComplianceControl{ID: uuid.New(), ReferenceCode: code, Name: code, PackKey: "iso", PackItemKey: code,
			PackVersion: "1.0.0", PackBase: &pb.JSON, PackBaseHash: &pb.Hash}
	}
	a1, a2 := mk("A.1"), mk("A.2")
	link := &domain.ControlCrosswalk{SourceControlID: a1.ID, TargetControlID: a2.ID, Coverage: "full", Origin: domain.CrosswalkOriginCurated,
		PackKey: "crosswalks", PackItemKey: "iso:A.1|iso:A.2", PackVersion: "1.0.0"}
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{
		TenantID: tenantA, FrameworkID: fwA.ID, CatalogKey: "iso",
		Stamp:    &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"},
		Controls: []*domain.ComplianceControl{a1, a2}, Links: []*domain.ControlCrosswalk{link},
	}))

	// The stored hash is the hash recomputed from the stored jsonb, whatever
	// key order and spacing Postgres gave it back.
	var got domain.ComplianceControl
	require.NoError(t, repo.db.First(&got, "id = ? AND tenant_id = ?", a1.ID, tenantA).Error)
	require.NotNil(t, got.PackBase)
	require.NotNil(t, got.PackBaseHash)
	assert.Len(t, *got.PackBaseHash, 64)
	again, err := pkgcompliance.HashPackBase([]byte(*got.PackBase))
	require.NoError(t, err)
	assert.Equal(t, *got.PackBaseHash, again)
	fw, err := repo.GetFrameworkByID(ctx, fwA.ID, tenantA)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", fw.CatalogContentVersion)
	assert.Equal(t, "[]", strings.ReplaceAll(fw.CatalogPendingKinds, " ", ""))
	assert.EqualValues(t, 2, tableCount(t, repo.db, controlchain.TableControls))

	// Same item under another tenant: allowed. Tombstone in the same framework: refused.
	require.NoError(t, repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenantB, FrameworkID: fwB.ID, Controls: []*domain.ComplianceControl{mk("A.1")}}))
	require.NoError(t, repo.DeleteControl(ctx, a1.ID, tenantA))
	all, err := repo.ListControlsByFrameworkIncludingDeleted(ctx, tenantA, fwA.ID)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	err = repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{TenantID: tenantA, FrameworkID: fwA.ID, Controls: []*domain.ComplianceControl{mk("A.1")}})
	require.ErrorIs(t, err, domain.ErrConflict)

	// Rollback on Postgres: a failing link leaves no control, mirror or stamp.
	fwC := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenantA, Name: "Other", Version: "1"}
	require.NoError(t, repo.CreateFramework(ctx, fwC))
	boom := errors.New("injected")
	require.NoError(t, repo.db.Callback().Create().Before("gorm:create").Register("test:fail_pg_link", func(tx *gorm.DB) {
		if tx.Statement.Table == "control_crosswalks" {
			_ = tx.AddError(boom)
		}
	}))
	t.Cleanup(func() { _ = repo.db.Callback().Create().Remove("test:fail_pg_link") })
	c1, c2 := mk("B.1"), mk("B.2")
	tableBefore := tableCount(t, repo.db, controlchain.TableControls)
	err = repo.ApplyCatalogImport(ctx, &domain.CatalogImportPlan{
		TenantID: tenantA, FrameworkID: fwC.ID, CatalogKey: "iso",
		Stamp:    &domain.CatalogFrameworkStamp{ContentVersion: "1.0.0", CrosswalksVersion: "1.0.0"},
		Controls: []*domain.ComplianceControl{c1, c2},
		Links:    []*domain.ControlCrosswalk{{SourceControlID: c1.ID, TargetControlID: c2.ID, Coverage: "full", Origin: domain.CrosswalkOriginCurated}},
	})
	require.ErrorIs(t, err, boom)
	var n int64
	require.NoError(t, repo.db.Unscoped().Model(&domain.ComplianceControl{}).Where("tenant_id = ? AND framework_id = ?", tenantA, fwC.ID).Count(&n).Error)
	assert.Zero(t, n)
	assert.Equal(t, tableBefore, tableCount(t, repo.db, controlchain.TableControls), "no mirror from the failed import remains")
	stamped, err := repo.GetFrameworkByID(ctx, fwC.ID, tenantA)
	require.NoError(t, err)
	assert.Empty(t, stamped.CatalogContentVersion)
}
