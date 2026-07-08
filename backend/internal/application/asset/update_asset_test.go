// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package asset

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateAsset_Success_CriticalityChanged(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	var snapshotTaken *domain.AssetSnapshot
	var updated *domain.Asset

	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid, Name: "Server", Criticality: domain.CriticalityLow}, nil
		},
		createSnapshotFunc: func(ctx context.Context, s *domain.AssetSnapshot) error {
			snapshotTaken = s
			return nil
		},
		updateFunc: func(ctx context.Context, a *domain.Asset) error {
			updated = a
			return nil
		},
	}
	uc := NewUpdateAssetUseCase(repo)
	newCriticality := domain.CriticalityCritical

	result, err := uc.Execute(context.Background(), tenantID, assetID, UpdateAssetInput{Criticality: &newCriticality})

	require.NoError(t, err)
	assert.True(t, result.CriticalityChanged)
	assert.Equal(t, domain.CriticalityLow, result.OldCriticality)
	assert.Equal(t, domain.CriticalityCritical, result.NewCriticality)
	require.NotNil(t, snapshotTaken, "must snapshot the pre-update state")
	assert.Equal(t, domain.CriticalityLow, snapshotTaken.Criticality, "snapshot must capture the OLD value")
	assert.Equal(t, "update", snapshotTaken.Reason)
	require.NotNil(t, updated)
	assert.Equal(t, domain.CriticalityCritical, updated.Criticality)
}

func TestUpdateAsset_NoCriticalityChange(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid, Name: "Server", Criticality: domain.CriticalityMedium}, nil
		},
	}
	uc := NewUpdateAssetUseCase(repo)
	name := "Renamed Server"

	result, err := uc.Execute(context.Background(), tenantID, assetID, UpdateAssetInput{Name: &name})

	require.NoError(t, err)
	assert.False(t, result.CriticalityChanged)
	assert.Equal(t, "Renamed Server", result.Asset.Name)
}

func TestUpdateAsset_NotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewUpdateAssetUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateAssetInput{})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateAsset_CrossTenantReturnsNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil // repo already scopes by tenant_id — foreign asset looks absent
		},
	}
	uc := NewUpdateAssetUseCase(repo)
	name := "Hijacked"

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateAssetInput{Name: &name})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateAsset_EmptyNameRejected(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid, Name: "Server"}, nil
		},
	}
	uc := NewUpdateAssetUseCase(repo)
	empty := ""

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), UpdateAssetInput{Name: &empty})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}
