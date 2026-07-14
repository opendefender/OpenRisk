// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package scanner

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

// ListScanJobsUseCase returns a tenant's scan jobs (history + in-flight).
type ListScanJobsUseCase struct {
	repo domain.ScanJobRepository
}

func NewListScanJobsUseCase(repo domain.ScanJobRepository) *ListScanJobsUseCase {
	return &ListScanJobsUseCase{repo: repo}
}

func (uc *ListScanJobsUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ScanJob, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	jobs, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	return jobs, nil
}

// GetScanPreviewUseCase loads a scan's Redis preview (tenant-scoped by key).
type GetScanPreviewUseCase struct {
	preview *scanpkg.PreviewStore
}

func NewGetScanPreviewUseCase(preview *scanpkg.PreviewStore) *GetScanPreviewUseCase {
	return &GetScanPreviewUseCase{preview: preview}
}

func (uc *GetScanPreviewUseCase) Execute(ctx context.Context, tenantID, jobID uuid.UUID) (*scanpkg.ScanPreview, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	p, err := uc.preview.Load(ctx, tenantID, jobID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if p == nil {
		return nil, domain.NewNotFoundError("scan preview", jobID)
	}
	return p, nil
}

// ImportSelection selects one discovered asset to promote to inventory, with an
// optional criticality override from the user (the editable field on the preview
// page). If Criticality is empty, the scanner's inferred value is used.
type ImportSelection struct {
	ExternalID  string
	Criticality domain.AssetCriticality
}

// ImportPreviewInput is the user's manual import decision.
type ImportPreviewInput struct {
	JobID      uuid.UUID
	Selections []ImportSelection
}

// ImportPreviewResult reports what the import created.
type ImportPreviewResult struct {
	AssetsImported int `json:"assets_imported"`
}

// ImportPreviewUseCase promotes selected discovered assets from a Redis preview
// into the real Asset inventory. THIS is the only place a scan result becomes a
// DB row, and only on explicit user action. Findings/risk creation from the
// preview is a deliberate follow-up (findings are shown for triage first).
type ImportPreviewUseCase struct {
	preview   *scanpkg.PreviewStore
	assetRepo domain.AssetRepository
}

func NewImportPreviewUseCase(preview *scanpkg.PreviewStore, assetRepo domain.AssetRepository) *ImportPreviewUseCase {
	return &ImportPreviewUseCase{preview: preview, assetRepo: assetRepo}
}

func (uc *ImportPreviewUseCase) Execute(ctx context.Context, tenantID uuid.UUID, in ImportPreviewInput) (*ImportPreviewResult, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	if len(in.Selections) == 0 {
		return nil, domain.NewValidationError("no assets selected for import")
	}
	p, err := uc.preview.Load(ctx, tenantID, in.JobID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if p == nil {
		return nil, domain.NewNotFoundError("scan preview", in.JobID)
	}

	// Index the preview's assets by ExternalID for O(1) selection lookup.
	byExt := make(map[string]scanpkg.AssetDiscovery, len(p.Assets))
	for _, a := range p.Assets {
		byExt[a.ExternalID] = a
	}

	result := &ImportPreviewResult{}
	for _, sel := range in.Selections {
		disc, ok := byExt[sel.ExternalID]
		if !ok {
			// Selection not in the preview (stale UI / tampering) — skip, don't fail
			// the whole import.
			continue
		}
		criticality := sel.Criticality
		if criticality == "" {
			criticality = scanpkg.CriticalityLabel(disc.Criticality)
		}
		asset := &domain.Asset{
			ID:          uuid.New(),
			TenantID:    tenantID,
			Name:        disc.Name,
			Type:        string(disc.Type),
			Criticality: criticality,
			Source:      "SCANNER",
			ExternalID:  disc.ExternalID,
		}
		if err := uc.assetRepo.Create(ctx, asset); err != nil {
			return nil, domain.NewInternalError(err.Error())
		}
		result.AssetsImported++
	}
	return result, nil
}

// IgnorePreviewUseCase discards a scan preview (the user chose not to import).
type IgnorePreviewUseCase struct {
	preview *scanpkg.PreviewStore
}

func NewIgnorePreviewUseCase(preview *scanpkg.PreviewStore) *IgnorePreviewUseCase {
	return &IgnorePreviewUseCase{preview: preview}
}

func (uc *IgnorePreviewUseCase) Execute(ctx context.Context, tenantID, jobID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewUnauthorizedError("missing tenant")
	}
	if err := uc.preview.Delete(ctx, tenantID, jobID); err != nil {
		return domain.NewInternalError(err.Error())
	}
	return nil
}
