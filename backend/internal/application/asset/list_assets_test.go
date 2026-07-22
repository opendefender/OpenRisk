// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAssets_Success(t *testing.T) {
	tenantID := uuid.New()
	repo := &MockAssetRepository{
		listFunc: func(ctx context.Context, tid uuid.UUID) ([]domain.Asset, error) {
			assert.Equal(t, tenantID, tid)
			return []domain.Asset{{Name: "A1"}, {Name: "A2"}}, nil
		},
	}
	uc := NewListAssetsUseCase(repo)

	got, err := uc.Execute(context.Background(), tenantID)

	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListAssets_EmptyInventory(t *testing.T) {
	repo := &MockAssetRepository{}
	uc := NewListAssetsUseCase(repo)

	got, err := uc.Execute(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestListAssets_RepositoryError(t *testing.T) {
	repo := &MockAssetRepository{
		listFunc: func(ctx context.Context, tid uuid.UUID) ([]domain.Asset, error) {
			return nil, errors.New("db down")
		},
	}
	uc := NewListAssetsUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInternal)
}
