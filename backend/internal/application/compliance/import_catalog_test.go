// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

// #809: the catalogs went from summaries (20 NIST families, 12 NIS2 rows, 19
// DORA articles) to control-level depth. A tenant who imported the old shape
// and assessed it re-imports to get the new rows. The upgrade is additive: the
// rows they already hold — and the status they recorded on them — are never
// touched, rows whose code did not change are not duplicated, and a second
// re-import adds nothing.
func TestImportCatalogUseCase_UpgradeFromSummaryCatalog_IsAdditive(t *testing.T) {
	cases := []struct {
		key string
		old []string // codes of the earlier catalog a tenant may already hold
		// codes the earlier and the current catalog share, which must be skipped
		kept []string
	}{
		{"nist-800-53-r5", []string{"AC", "AT", "AU", "CA", "CM", "CP", "IA", "IR", "MA", "MP", "PE", "PL", "PM", "PS", "PT", "RA", "SA", "SC", "SI", "SR"}, nil},
		{"nis2-2022-2555", []string{"Art.20", "Art.21(a)", "Art.21(b)", "Art.21(c)", "Art.21(d)", "Art.21(e)", "Art.21(f)", "Art.21(g)", "Art.21(h)", "Art.21(i)", "Art.21(j)", "Art.23"},
			[]string{"Art.20", "Art.21(a)", "Art.21(b)", "Art.21(c)", "Art.21(d)", "Art.21(e)", "Art.21(f)", "Art.21(g)", "Art.21(h)", "Art.21(i)", "Art.21(j)"}},
		{"dora-2022-2554", []string{"Art.5", "Art.6", "Art.7", "Art.8", "Art.9", "Art.10", "Art.11", "Art.12", "Art.13", "Art.17", "Art.18", "Art.19", "Art.24", "Art.25", "Art.26", "Art.28", "Art.29", "Art.30", "Art.45"},
			[]string{"Art.7", "Art.45"}},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			tenantID, fwID := uuid.New(), uuid.New()
			// The tenant's existing rows, each with an assessment on it.
			stored := map[string]domain.ComplianceControl{}
			for _, code := range tc.old {
				stored[code] = domain.ComplianceControl{
					ID: uuid.New(), TenantID: tenantID, FrameworkID: fwID,
					ReferenceCode: code, Name: "old " + code, Status: domain.ControlStatusImplemented,
				}
			}
			before := make(map[string]domain.ComplianceControl, len(stored))
			for k, v := range stored {
				before[k] = v
			}

			repo := &MockComplianceRepository{
				getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
					require.Equal(t, tenantID, tid)
					return &domain.ComplianceFramework{ID: fwID, TenantID: tid, CatalogKey: tc.key}, nil
				},
				listControlsByFrameworkFunc: func(_ context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
					require.Equal(t, tenantID, tid)
					out := make([]domain.ComplianceControl, 0, len(stored))
					for _, c := range stored {
						out = append(out, c)
					}
					return out, nil
				},
				createControlFunc: func(_ context.Context, c *domain.ComplianceControl) error {
					require.Equal(t, tenantID, c.TenantID, "a created row carries the importing tenant")
					require.Equal(t, fwID, c.FrameworkID)
					if _, dup := stored[c.ReferenceCode]; dup {
						t.Errorf("re-created %q, which the tenant already holds", c.ReferenceCode)
					}
					stored[c.ReferenceCode] = *c
					return nil
				},
				updateControlFunc: func(_ context.Context, c *domain.ComplianceControl) error {
					t.Errorf("an import must not update an existing control (%q)", c.ReferenceCode)
					return nil
				},
				deleteControlFunc: func(_ context.Context, id, _ uuid.UUID) error {
					t.Errorf("an import must not delete a control (%s)", id)
					return nil
				},
			}
			uc := NewImportCatalogUseCase(repo)

			first, err := uc.Execute(context.Background(), tenantID, ImportCatalogInput{FrameworkID: fwID, CatalogKey: tc.key})
			require.NoError(t, err)
			assert.Equal(t, len(tc.kept), first.Skipped, "codes shared with the earlier catalog are skipped")
			assert.Equal(t, first.Total-len(tc.kept), first.Imported)

			// Nothing the tenant held was changed or removed.
			for code, was := range before {
				now, ok := stored[code]
				require.True(t, ok, "%s disappeared", code)
				assert.Equal(t, was, now, "%s was modified", code)
			}
			assert.Len(t, stored, len(tc.old)+first.Imported)

			second, err := uc.Execute(context.Background(), tenantID, ImportCatalogInput{FrameworkID: fwID, CatalogKey: tc.key})
			require.NoError(t, err)
			assert.Equal(t, 0, second.Imported, "a second re-import adds nothing")
			assert.Equal(t, second.Total, second.Skipped)
		})
	}
}
