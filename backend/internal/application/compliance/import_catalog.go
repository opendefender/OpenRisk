// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

// ImportCatalogInput selects which regulatory catalog (see pkg/compliance) to instantiate
// as controls under an existing, tenant-owned framework.
type ImportCatalogInput struct {
	FrameworkID uuid.UUID
	CatalogKey  string
}

// ImportCatalogResult reports what happened — importing is idempotent, so re-running it
// after partial progress (or just to pick up newly-added catalog controls) is safe.
type ImportCatalogResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"` // already existed for this (tenant, framework) by reference_code
	Total    int `json:"total"`
}

// ImportCatalogUseCase bulk-creates controls for a tenant from a static regulatory catalog
// (e.g. ISO 27001:2022's 93 Annex A controls), rather than requiring an admin to enter each
// one by hand via CreateControlUseCase. See ROADMAP.md M2.
type ImportCatalogUseCase struct {
	repo domain.ComplianceRepository
}

func NewImportCatalogUseCase(repo domain.ComplianceRepository) *ImportCatalogUseCase {
	return &ImportCatalogUseCase{repo: repo}
}

func (uc *ImportCatalogUseCase) Execute(ctx context.Context, tenantID uuid.UUID, input ImportCatalogInput) (*ImportCatalogResult, error) {
	if input.FrameworkID == uuid.Nil {
		return nil, domain.NewValidationError("framework_id is required")
	}

	fw, err := uc.repo.GetFrameworkByID(ctx, input.FrameworkID, tenantID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", input.FrameworkID)
	}

	catalog, ok := pkgcompliance.Get(input.CatalogKey)
	if !ok {
		return nil, domain.NewValidationError("unknown catalog: " + input.CatalogKey)
	}
	if !catalog.Available {
		return nil, domain.NewValidationError("catalog " + input.CatalogKey + " is not yet available — no reviewed control content")
	}

	existing, err := uc.repo.ListControlsByFramework(ctx, tenantID, input.FrameworkID)
	if err != nil {
		return nil, err
	}
	existingCodes := make(map[string]bool, len(existing))
	for _, c := range existing {
		if c.ReferenceCode != "" {
			existingCodes[c.ReferenceCode] = true
		}
	}

	result := &ImportCatalogResult{Total: len(catalog.Controls)}
	for _, cc := range catalog.Controls {
		if existingCodes[cc.ReferenceCode] {
			result.Skipped++
			continue
		}

		control := &domain.ComplianceControl{
			ID:              uuid.New(),
			TenantID:        tenantID,
			FrameworkID:     input.FrameworkID,
			ReferenceCode:   cc.ReferenceCode,
			Name:            cc.Name,
			Description:     cc.Description,
			SourceReference: cc.SourceReference,
			Status:          domain.ControlStatusNotImplemented,
		}
		if err := uc.repo.CreateControl(ctx, control); err != nil {
			return nil, err
		}
		result.Imported++
	}

	return result, nil
}
