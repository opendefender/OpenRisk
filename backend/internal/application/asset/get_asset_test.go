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

func TestGetAsset_Success(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			assert.Equal(t, assetID, id)
			assert.Equal(t, tenantID, tid)
			return &domain.Asset{ID: id, TenantID: tid, Name: "Server"}, nil
		},
	}
	uc := NewGetAssetUseCase(repo)

	got, err := uc.Execute(context.Background(), tenantID, assetID)

	require.NoError(t, err)
	assert.Equal(t, "Server", got.Name)
}

func TestGetAsset_NotFound(t *testing.T) {
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil
		},
	}
	uc := NewGetAssetUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetAsset_CrossTenantReturnsNotFound(t *testing.T) {
	// The repository is the tenant boundary: a foreign-tenant asset must
	// surface identically to a nonexistent one — never a 403.
	repo := &MockAssetRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			return nil, nil // repo already filters by tenant_id
		},
	}
	uc := NewGetAssetUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
