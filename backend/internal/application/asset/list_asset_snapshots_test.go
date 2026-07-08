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

func TestListAssetSnapshots_Success(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return &domain.Asset{ID: id, TenantID: tid}, nil
		},
		listSnapshotsFunc: func(ctx context.Context, aid, tid uuid.UUID) ([]domain.AssetSnapshot, error) {
			assert.Equal(t, assetID, aid)
			assert.Equal(t, tenantID, tid)
			return []domain.AssetSnapshot{{Criticality: domain.CriticalityLow}, {Criticality: domain.CriticalityHigh}}, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	got, err := uc.Execute(context.Background(), tenantID, assetID)

	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListAssetSnapshots_AssetNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListAssetSnapshots_CrossTenantReturnsNotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewListAssetSnapshotsUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
