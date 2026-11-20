// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

func fwRepo(fw *domain.ComplianceFramework) *MockComplianceRepository {
	return &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			if id != fw.ID || tid != fw.TenantID {
				return nil, nil
			}
			c := *fw
			return &c, nil
		},
	}
}

// Criteria 1 and 2: provenance on every created control, stamp on a first import.
func TestImportCatalog_Success_WritesProvenanceAndStamp(t *testing.T) {
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant}
	repo := fwRepo(fw)
	_, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.NoError(t, err)

	require.Len(t, repo.appliedPlans, 1, "one plan, one transaction")
	plan := repo.appliedPlans[0]
	cat, _ := pkgcompliance.Get("iso27001-2022")
	require.Len(t, plan.Controls, 93)
	for _, c := range plan.Controls {
		assert.Equal(t, "iso27001-2022", c.PackKey)
		assert.Equal(t, c.ReferenceCode, c.PackItemKey)
		assert.Equal(t, cat.ContentVersion, c.PackVersion)
		require.NotNil(t, c.PackBase)
		require.NotNil(t, c.PackBaseHash)
		assert.Len(t, *c.PackBaseHash, 64)
		again, err := pkgcompliance.HashPackBase([]byte(*c.PackBase))
		require.NoError(t, err)
		assert.Equal(t, *c.PackBaseHash, again, "hash equals the hash recomputed from pack_base")
		assert.Empty(t, c.PackRetiredIn)
		assert.Nil(t, c.PackRevertedBy)
	}
	require.NotNil(t, plan.Stamp)
	assert.Equal(t, cat.ContentVersion, plan.Stamp.ContentVersion)
	assert.Equal(t, pkgcompliance.CrosswalksVersion, plan.Stamp.CrosswalksVersion)
	assert.Equal(t, "iso27001-2022", plan.CatalogKey)
}

// Criterion 13: no tenant context is unauthorized, before any read.
func TestImportCatalog_Unauthorized_NilTenant(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(context.Context, uuid.UUID, uuid.UUID) (*domain.ComplianceFramework, error) {
			t.Fatal("must not read with a nil tenant")
			return nil, nil
		},
	}
	_, err := NewImportCatalogUseCase(repo).Execute(context.Background(), uuid.Nil, ImportCatalogInput{FrameworkID: uuid.New(), CatalogKey: "iso27001-2022"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, repo.appliedPlans)
}

// Criterion 13: another tenant's framework is not found, nothing written.
func TestImportCatalog_NotFound_ForeignTenantFramework(t *testing.T) {
	owner := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: owner}
	repo := fwRepo(fw)
	_, err := NewImportCatalogUseCase(repo).Execute(context.Background(), uuid.New(), ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Empty(t, repo.appliedPlans)
}

// Criterion 5: a provenance tombstone is never re-created; others are unchanged.
func TestImportCatalog_TombstoneNotRecreated(t *testing.T) {
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, CatalogKey: "iso27001-2022"}
	repo := fwRepo(fw)
	repo.listControlsAllFunc = func(_ context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
		require.Equal(t, tenant, tid)
		require.Equal(t, fw.ID, fid)
		return []domain.ComplianceControl{{
			ReferenceCode: "A.5.1", PackKey: "iso27001-2022", PackItemKey: "A.5.1",
			DeletedAt: gorm.DeletedAt{Valid: true},
		}}, nil
	}
	res, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 92, res.Imported)
	for _, c := range repo.appliedPlans[0].Controls {
		assert.NotEqual(t, "A.5.1", c.ReferenceCode)
	}
	assert.Nil(t, repo.appliedPlans[0].Stamp, "a framework that ever held controls is not stamped")
}

// Criterion 7: a deleted row WITHOUT provenance does not block re-creation.
func TestImportCatalog_DeletedHandMadeControlIsRecreated(t *testing.T) {
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, CatalogKey: "iso27001-2022"}
	repo := fwRepo(fw)
	repo.listControlsAllFunc = func(context.Context, uuid.UUID, uuid.UUID) ([]domain.ComplianceControl, error) {
		return []domain.ComplianceControl{{ReferenceCode: "A.5.1", DeletedAt: gorm.DeletedAt{Valid: true}}}, nil
	}
	res, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.NoError(t, err)
	assert.Equal(t, 93, res.Imported)
}

// Criterion 2 / brief rule 5: a legacy framework (live controls, empty stamp) keeps ”.
func TestImportCatalog_LegacyFrameworkNotStamped(t *testing.T) {
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, CatalogKey: "iso27001-2022"}
	repo := fwRepo(fw)
	repo.listControlsByFrameworkFunc = func(context.Context, uuid.UUID, uuid.UUID) ([]domain.ComplianceControl, error) {
		return []domain.ComplianceControl{{ReferenceCode: "A.5.1"}}, nil
	}
	_, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.NoError(t, err)
	assert.Nil(t, repo.appliedPlans[0].Stamp)
	assert.Len(t, repo.appliedPlans[0].Controls, 92, "missing items are still created, with provenance")
	assert.Equal(t, "iso27001-2022", repo.appliedPlans[0].Controls[0].PackKey)
}

// Criterion 14: fully imported and unchanged writes nothing.
func TestImportCatalog_NoOpReimportPlansNothing(t *testing.T) {
	tenant := uuid.New()
	cat, _ := pkgcompliance.Get("iso27001-2022")
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, CatalogKey: "iso27001-2022",
		CatalogContentVersion: cat.ContentVersion, CatalogCrosswalksVersion: pkgcompliance.CrosswalksVersion}
	repo := fwRepo(fw)
	var all []domain.ComplianceControl
	for _, c := range cat.Controls {
		all = append(all, domain.ComplianceControl{ReferenceCode: c.ReferenceCode, PackKey: cat.Key, PackItemKey: c.ReferenceCode})
	}
	repo.listControlsAllFunc = func(context.Context, uuid.UUID, uuid.UUID) ([]domain.ComplianceControl, error) { return all, nil }
	res, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Imported)
	require.Len(t, repo.appliedPlans, 1)
	assert.True(t, repo.appliedPlans[0].Empty(), "an empty plan is a no-op in the repository")
}

// Criterion 8 (use-case half): a repository failure fails the import.
func TestImportCatalog_RepositoryFailureFailsImport(t *testing.T) {
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant}
	repo := fwRepo(fw)
	repo.applyCatalogImportFunc = func(context.Context, *domain.CatalogImportPlan) error {
		return domain.NewConflictError("control", "pack_item_key")
	}
	_, err := NewImportCatalogUseCase(repo).Execute(context.Background(), tenant, ImportCatalogInput{FrameworkID: fw.ID, CatalogKey: "iso27001-2022"})
	assert.ErrorIs(t, err, domain.ErrConflict)
}
