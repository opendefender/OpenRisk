// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportCatalogUseCase_Success(t *testing.T) {
	fwID := uuid.New()
	tenantID := uuid.New()
	created := []domain.ComplianceControl{}

	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID, Name: "ISO 27001", Version: "2022"}, nil
		},
		createControlFunc: func(ctx context.Context, c *domain.ComplianceControl) error {
			created = append(created, *c)
			return nil
		},
	}
	uc := NewImportCatalogUseCase(repo)

	result, err := uc.Execute(context.Background(), tenantID, ImportCatalogInput{
		FrameworkID: fwID, CatalogKey: "iso27001-2022",
	})

	require.NoError(t, err)
	assert.Equal(t, 93, result.Total)
	assert.Equal(t, 93, result.Imported)
	assert.Equal(t, 0, result.Skipped)
	require.Len(t, created, 93)
	for _, c := range created {
		assert.Equal(t, tenantID, c.TenantID)
		assert.Equal(t, fwID, c.FrameworkID)
		assert.NotEmpty(t, c.ReferenceCode)
		assert.NotEmpty(t, c.SourceReference)
		assert.Equal(t, domain.ControlStatusNotImplemented, c.Status)
	}
}

func TestImportCatalogUseCase_Idempotent_SkipsExisting(t *testing.T) {
	fwID := uuid.New()
	tenantID := uuid.New()

	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID}, nil
		},
		listControlsByFrameworkFunc: func(ctx context.Context, tid, frameworkID uuid.UUID) ([]domain.ComplianceControl, error) {
			// Pretend a prior import already created every control.
			return []domain.ComplianceControl{
				{ReferenceCode: "A.5.1"}, {ReferenceCode: "A.5.2"}, {ReferenceCode: "A.5.3"},
			}, nil
		},
		createControlFunc: func(ctx context.Context, c *domain.ComplianceControl) error {
			if c.ReferenceCode == "A.5.1" || c.ReferenceCode == "A.5.2" || c.ReferenceCode == "A.5.3" {
				t.Errorf("re-created control %q that should have been skipped", c.ReferenceCode)
			}
			return nil
		},
	}
	uc := NewImportCatalogUseCase(repo)

	result, err := uc.Execute(context.Background(), tenantID, ImportCatalogInput{
		FrameworkID: fwID, CatalogKey: "iso27001-2022",
	})

	require.NoError(t, err)
	assert.Equal(t, 93, result.Total)
	assert.Equal(t, 3, result.Skipped)
	assert.Equal(t, 90, result.Imported)
}

func TestImportCatalogUseCase_FrameworkNotFound(t *testing.T) {
	repo := &MockComplianceRepository{}
	uc := NewImportCatalogUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), ImportCatalogInput{
		FrameworkID: uuid.New(), CatalogKey: "iso27001-2022",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestImportCatalogUseCase_UnknownCatalog_Validation(t *testing.T) {
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID}, nil
		},
	}
	uc := NewImportCatalogUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), ImportCatalogInput{
		FrameworkID: fwID, CatalogKey: "does-not-exist",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestImportCatalogUseCase_UnavailableCatalog_Validation(t *testing.T) {
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID}, nil
		},
	}
	uc := NewImportCatalogUseCase(repo)

	// cm-loi-2024-017 is registered but not yet available (see pkg/compliance/catalog_placeholders.go).
	_, err := uc.Execute(context.Background(), uuid.New(), ImportCatalogInput{
		FrameworkID: fwID, CatalogKey: "cm-loi-2024-017",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}
