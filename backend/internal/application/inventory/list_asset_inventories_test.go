// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// tenantAssets mirrors AssetRepository.GetByID: (nil, nil) when absent or foreign.
type tenantAssets map[uuid.UUID]uuid.UUID // asset id -> owning tenant

func (a tenantAssets) GetByID(_ context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	if owner, ok := a[id]; ok && owner == tenantID {
		return &domain.Asset{ID: id, TenantID: tenantID}, nil
	}
	return nil, nil
}

func TestListAssetInventories_Success(t *testing.T) {
	tenant, asset := uuid.New(), uuid.New()
	repo := newFakeRepo()
	inv := &domain.HostInventory{TenantID: tenant, HostKey: "h1", AssetID: &asset}
	require.NoError(t, repo.Replace(context.Background(), inv, nil))
	uc := NewListAssetInventoriesUseCase(repo, tenantAssets{asset: tenant})

	out, err := uc.Execute(context.Background(), tenant, asset)
	require.NoError(t, err)
	assert.Len(t, out, 1)

	// An own asset with no inventory answers an empty list, not a not-found.
	empty := uuid.New()
	uc = NewListAssetInventoriesUseCase(repo, tenantAssets{empty: tenant})
	out, err = uc.Execute(context.Background(), tenant, empty)
	require.NoError(t, err)
	assert.Equal(t, []domain.HostInventory{}, out)
}

// #1520: foreign and unknown assets answer the same not-found.
func TestListAssetInventories_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	tenant, other, foreign := uuid.New(), uuid.New(), uuid.New()
	repo := newFakeRepo()
	inv := &domain.HostInventory{TenantID: other, HostKey: "h1", AssetID: &foreign}
	require.NoError(t, repo.Replace(context.Background(), inv, nil))
	uc := NewListAssetInventoriesUseCase(repo, tenantAssets{foreign: other})

	_, errForeign := uc.Execute(context.Background(), tenant, foreign)
	_, errUnknown := uc.Execute(context.Background(), tenant, uuid.New())

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
}

func TestListAssetInventories_Unauthorized(t *testing.T) {
	uc := NewListAssetInventoriesUseCase(newFakeRepo(), tenantAssets{})

	_, err := uc.Execute(context.Background(), uuid.Nil, uuid.New())

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
