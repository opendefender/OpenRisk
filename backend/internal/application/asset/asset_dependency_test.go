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

// assetsExist returns a MockAssetRepository whose GetByID resolves the two
// given IDs (within the tenant) and nothing else.
func assetsExist(tenantID uuid.UUID, ids ...uuid.UUID) *MockAssetRepository {
	set := map[uuid.UUID]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return &MockAssetRepository{
		getByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.Asset, error) {
			if tid == tenantID && set[id] {
				return &domain.Asset{ID: id, TenantID: tid}, nil
			}
			return nil, nil // not found / foreign tenant
		},
	}
}

func TestCreateAssetDependency_Success(t *testing.T) {
	tenantID, src, tgt := uuid.New(), uuid.New(), uuid.New()
	var created *domain.AssetDependency
	deps := &MockAssetDependencyRepository{
		createFunc: func(_ context.Context, d *domain.AssetDependency) error { created = d; return nil },
	}
	uc := NewCreateAssetDependencyUseCase(deps, assetsExist(tenantID, src, tgt))

	got, err := uc.Execute(context.Background(), tenantID, CreateAssetDependencyInput{
		SourceAssetID: src, TargetAssetID: tgt, Type: domain.DepRunsOn,
	})

	require.NoError(t, err)
	assert.Equal(t, src, got.SourceAssetID)
	assert.Equal(t, tgt, got.TargetAssetID)
	assert.Equal(t, domain.DepRunsOn, got.Type)
	assert.Equal(t, tenantID, got.TenantID)
	require.NotNil(t, created)
}

func TestCreateAssetDependency_DefaultsType(t *testing.T) {
	tenantID, src, tgt := uuid.New(), uuid.New(), uuid.New()
	uc := NewCreateAssetDependencyUseCase(&MockAssetDependencyRepository{}, assetsExist(tenantID, src, tgt))

	got, err := uc.Execute(context.Background(), tenantID, CreateAssetDependencyInput{
		SourceAssetID: src, TargetAssetID: tgt,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.DepDependsOn, got.Type)
}

func TestCreateAssetDependency_SelfReference(t *testing.T) {
	tenantID, a := uuid.New(), uuid.New()
	uc := NewCreateAssetDependencyUseCase(&MockAssetDependencyRepository{}, assetsExist(tenantID, a))

	_, err := uc.Execute(context.Background(), tenantID, CreateAssetDependencyInput{
		SourceAssetID: a, TargetAssetID: a,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}

// NotFound: the target asset does not exist within the tenant.
func TestCreateAssetDependency_TargetNotFound(t *testing.T) {
	tenantID, src, tgt := uuid.New(), uuid.New(), uuid.New()
	uc := NewCreateAssetDependencyUseCase(&MockAssetDependencyRepository{}, assetsExist(tenantID, src)) // only src exists

	_, err := uc.Execute(context.Background(), tenantID, CreateAssetDependencyInput{
		SourceAssetID: src, TargetAssetID: tgt,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// Unauthorized / cross-tenant: the source asset belongs to another tenant, so
// GetByID (scoped to the caller's tenant) resolves nil → not found, never a leak.
func TestCreateAssetDependency_CrossTenantSourceNotFound(t *testing.T) {
	ownerTenant, otherTenant := uuid.New(), uuid.New()
	src, tgt := uuid.New(), uuid.New()
	// Assets exist only for ownerTenant; the caller is otherTenant.
	uc := NewCreateAssetDependencyUseCase(&MockAssetDependencyRepository{}, assetsExist(ownerTenant, src, tgt))

	_, err := uc.Execute(context.Background(), otherTenant, CreateAssetDependencyInput{
		SourceAssetID: src, TargetAssetID: tgt,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCreateAssetDependency_Duplicate(t *testing.T) {
	tenantID, src, tgt := uuid.New(), uuid.New(), uuid.New()
	deps := &MockAssetDependencyRepository{
		existsFunc: func(_ context.Context, _, _, _ uuid.UUID, _ domain.DependencyType) (bool, error) {
			return true, nil
		},
	}
	uc := NewCreateAssetDependencyUseCase(deps, assetsExist(tenantID, src, tgt))

	_, err := uc.Execute(context.Background(), tenantID, CreateAssetDependencyInput{
		SourceAssetID: src, TargetAssetID: tgt, Type: domain.DepDependsOn,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestListAssetDependencies_Success(t *testing.T) {
	tenantID := uuid.New()
	want := []domain.AssetDependency{{ID: uuid.New(), TenantID: tenantID}}
	deps := &MockAssetDependencyRepository{
		listByTenantFunc: func(_ context.Context, tid uuid.UUID) ([]domain.AssetDependency, error) {
			assert.Equal(t, tenantID, tid)
			return want, nil
		},
	}
	uc := NewListAssetDependenciesUseCase(deps)

	got, err := uc.Execute(context.Background(), tenantID)

	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestDeleteAssetDependency_Success(t *testing.T) {
	tenantID, id := uuid.New(), uuid.New()
	deleted := false
	deps := &MockAssetDependencyRepository{
		getByIDFunc: func(_ context.Context, gid, tid uuid.UUID) (*domain.AssetDependency, error) {
			return &domain.AssetDependency{ID: gid, TenantID: tid}, nil
		},
		deleteFunc: func(_ context.Context, _, _ uuid.UUID) error { deleted = true; return nil },
	}
	uc := NewDeleteAssetDependencyUseCase(deps)

	err := uc.Execute(context.Background(), tenantID, id)

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestDeleteAssetDependency_NotFound(t *testing.T) {
	uc := NewDeleteAssetDependencyUseCase(&MockAssetDependencyRepository{}) // GetByID → nil,nil

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// Cross-tenant delete: the edge belongs to another tenant, so the scoped
// GetByID resolves nil → NotFound (never 403, never a foreign delete).
func TestDeleteAssetDependency_CrossTenantNotFound(t *testing.T) {
	deps := &MockAssetDependencyRepository{
		getByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.AssetDependency, error) {
			return nil, nil
		},
	}
	uc := NewDeleteAssetDependencyUseCase(deps)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
