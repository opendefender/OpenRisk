// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteAsset_Success(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	var snapshotTaken *domain.AssetSnapshot
	deleteCalled := false

	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid, Name: "Server", Criticality: domain.CriticalityHigh}, nil
		},
		createSnapshotFunc: func(ctx context.Context, s *domain.AssetSnapshot) error {
			snapshotTaken = s
			return nil
		},
		deleteFunc: func(ctx context.Context, id, tid uuid.UUID) error {
			deleteCalled = true
			assert.Equal(t, assetID, id)
			assert.Equal(t, tenantID, tid)
			return nil
		},
	}
	uc := NewDeleteAssetUseCase(repo)
	changedBy := uuid.New()

	err := uc.Execute(context.Background(), tenantID, assetID, changedBy)

	require.NoError(t, err)
	assert.True(t, deleteCalled)
	require.NotNil(t, snapshotTaken)
	assert.Equal(t, "delete", snapshotTaken.Reason)
	assert.Equal(t, domain.CriticalityHigh, snapshotTaken.Criticality)
	assert.Equal(t, changedBy, snapshotTaken.ChangedBy, "final snapshot must record WHO deleted the asset")
}

func TestDeleteAsset_NotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewDeleteAssetUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteAsset_CrossTenantReturnsNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewDeleteAssetUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
