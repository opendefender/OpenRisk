// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

	// Crosswalks reports the curated correspondences materialised against the
	// frameworks the tenant already held.
	Crosswalks *MaterialiseResult `json:"crosswalks,omitempty"`
	// InheritedCoverage is the head start: how much of what was just imported is
	// already answered by proof the tenant holds elsewhere. Returned WITH the
	// import so the answer arrives at the moment the question is asked, rather
	// than waiting for the user to go looking for it.
	InheritedCoverage *InheritedCoverage `json:"inherited_coverage,omitempty"`
}

// ImportCatalogUseCase bulk-creates controls for a tenant from a static regulatory catalog
// (e.g. ISO 27001:2022's 93 Annex A controls), rather than requiring an admin to enter each
// one by hand via CreateControlUseCase. See ROADMAP.md M2.
// ActivationRecorder notes the "imported a framework" milestone. Narrow port,
// satisfied structurally by application/activation.Recorder; nil-safe.
type ActivationRecorder interface {
	Record(ctx context.Context, tenantID uuid.UUID, key string, payload map[string]interface{})
}

type ImportCatalogUseCase struct {
	repo       domain.ComplianceRepository
	activation ActivationRecorder
	// Optional and nil-safe, like every other collaborator in this codebase: an
	// import must still succeed on a deployment where crosswalks are not wired.
	materialiser *CrosswalkMaterialiser
	coverage     *GetInheritedCoverageUseCase
}

func NewImportCatalogUseCase(repo domain.ComplianceRepository) *ImportCatalogUseCase {
	return &ImportCatalogUseCase{repo: repo}
}

// WithActivation attaches the optional activation recorder.
func (uc *ImportCatalogUseCase) WithActivation(rec ActivationRecorder) *ImportCatalogUseCase {
	uc.activation = rec
	return uc
}

// WithCrosswalks makes the import materialise curated correspondences and report
// the head start they produce. Optional; absent, an import behaves as before.
func (uc *ImportCatalogUseCase) WithCrosswalks(m *CrosswalkMaterialiser, cov *GetInheritedCoverageUseCase) *ImportCatalogUseCase {
	uc.materialiser, uc.coverage = m, cov
	return uc
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

	// Stamp what this framework IS, so crosswalks keep working after a rename.
	// Best-effort: failing the import over a label would be a poor trade, and the
	// controls — the thing the user asked for — are already in.
	if fw.CatalogKey != input.CatalogKey {
		fw.CatalogKey = input.CatalogKey
		_ = uc.repo.UpdateFramework(ctx, fw)
	}

	// Materialise the curated crosswalks against everything the tenant already
	// holds, then answer the question they are about to ask: how much of this do
	// I already have? Both are best-effort — a tenant who just imported 93
	// controls must not be told the import failed because a head-start number
	// could not be computed.
	if uc.materialiser != nil {
		catalogOf := func(id uuid.UUID) string {
			f, err := uc.repo.GetFrameworkByID(ctx, id, tenantID)
			if err != nil || f == nil {
				return ""
			}
			return f.CatalogKey
		}
		if cw, err := uc.materialiser.ForFramework(ctx, tenantID, input.FrameworkID, input.CatalogKey, catalogOf); err == nil {
			result.Crosswalks = cw
		}
	}
	if uc.coverage != nil {
		if cov, err := uc.coverage.Execute(ctx, tenantID, input.FrameworkID); err == nil && cov.CrosswalkedControls > 0 {
			result.InheritedCoverage = cov
		}
	}

	// ONE event for the whole import, whatever the number of controls. This is
	// precisely the bug that struck two checklist rows through at once: the panel
	// used to derive two steps from the same import. Here, one import → one event
	// → one step (domain.ValidateActivationSteps enforces the bijection).
	if uc.activation != nil && result.Imported > 0 {
		uc.activation.Record(ctx, tenantID, string(domain.ActivationFrameworkImported), map[string]interface{}{
			"framework_id": input.FrameworkID.String(),
			"catalog_key":  input.CatalogKey,
			"imported":     result.Imported,
		})
	}

	return result, nil
}
