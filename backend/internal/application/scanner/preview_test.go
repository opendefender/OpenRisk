// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

func seedPreview(t *testing.T, ps *scanpkg.PreviewStore, tenant, job uuid.UUID) {
	t.Helper()
	err := ps.Store(context.Background(), &scanpkg.ScanPreview{
		JobID: job, ConfigID: uuid.New(), TenantID: tenant, CreatedAt: time.Now(),
		Assets: []scanpkg.AssetDiscovery{{ExternalID: "h1", Name: "host-1", Type: domain.AssetTypeServer, Criticality: 2.5}},
	})
	require.NoError(t, err)
}

func TestImportPreview_Success(t *testing.T) {
	kv := newFakeKV()
	ps := scanpkg.NewPreviewStore(kv)
	tenant, job := uuid.New(), uuid.New()
	seedPreview(t, ps, tenant, job)
	assetRepo := &mockAssetRepo{}
	uc := NewImportPreviewUseCase(ps, assetRepo)

	res, err := uc.Execute(context.Background(), tenant, ImportPreviewInput{
		JobID: job, Selections: []ImportSelection{{ExternalID: "h1"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.AssetsImported)
	require.Len(t, assetRepo.created, 1)
	assert.Equal(t, "host-1", assetRepo.created[0].Name)
	assert.Equal(t, tenant, assetRepo.created[0].TenantID)
	assert.Equal(t, domain.CriticalityHigh, assetRepo.created[0].Criticality) // 2.5 → HIGH
	assert.Equal(t, "SCANNER", assetRepo.created[0].Source)
}

func TestImportPreview_CriticalityOverride(t *testing.T) {
	kv := newFakeKV()
	ps := scanpkg.NewPreviewStore(kv)
	tenant, job := uuid.New(), uuid.New()
	seedPreview(t, ps, tenant, job)
	assetRepo := &mockAssetRepo{}
	uc := NewImportPreviewUseCase(ps, assetRepo)

	_, err := uc.Execute(context.Background(), tenant, ImportPreviewInput{
		JobID: job, Selections: []ImportSelection{{ExternalID: "h1", Criticality: domain.CriticalityLow}},
	})
	require.NoError(t, err)
	require.Len(t, assetRepo.created, 1)
	assert.Equal(t, domain.CriticalityLow, assetRepo.created[0].Criticality)
}

func TestImportPreview_NotFound(t *testing.T) {
	uc := NewImportPreviewUseCase(scanpkg.NewPreviewStore(newFakeKV()), &mockAssetRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), ImportPreviewInput{
		JobID: uuid.New(), Selections: []ImportSelection{{ExternalID: "h1"}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestImportPreview_Unauthorized(t *testing.T) {
	uc := NewImportPreviewUseCase(scanpkg.NewPreviewStore(newFakeKV()), &mockAssetRepo{})
	_, err := uc.Execute(context.Background(), uuid.Nil, ImportPreviewInput{
		JobID: uuid.New(), Selections: []ImportSelection{{ExternalID: "h1"}},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestImportPreview_NoSelections(t *testing.T) {
	uc := NewImportPreviewUseCase(scanpkg.NewPreviewStore(newFakeKV()), &mockAssetRepo{})
	_, err := uc.Execute(context.Background(), uuid.New(), ImportPreviewInput{JobID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

func TestGetScanPreview_NotFound(t *testing.T) {
	uc := NewGetScanPreviewUseCase(scanpkg.NewPreviewStore(newFakeKV()))
	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetScanPreview_Success(t *testing.T) {
	kv := newFakeKV()
	ps := scanpkg.NewPreviewStore(kv)
	tenant, job := uuid.New(), uuid.New()
	seedPreview(t, ps, tenant, job)
	uc := NewGetScanPreviewUseCase(ps)
	p, err := uc.Execute(context.Background(), tenant, job)
	require.NoError(t, err)
	assert.Equal(t, job, p.JobID)
}
