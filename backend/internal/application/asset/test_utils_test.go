// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package asset

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MockAssetRepository is a hand-rolled mock implementing domain.AssetRepository,
// mirroring internal/application/compliance's MockComplianceRepository pattern
// (function fields, nil-safe defaults).
type MockAssetRepository struct {
	createFunc         func(ctx context.Context, a *domain.Asset) error
	getByIDFunc        func(ctx context.Context, id, tenantID uuid.UUID) (*domain.Asset, error)
	listFunc           func(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error)
	updateFunc         func(ctx context.Context, a *domain.Asset) error
	deleteFunc         func(ctx context.Context, id, tenantID uuid.UUID) error
	createSnapshotFunc func(ctx context.Context, s *domain.AssetSnapshot) error
	listSnapshotsFunc  func(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetSnapshot, error)
}

func (m *MockAssetRepository) Create(ctx context.Context, a *domain.Asset) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, a)
	}
	return nil
}

func (m *MockAssetRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Asset, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id, tenantID)
	}
	return nil, nil
}

func (m *MockAssetRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.Asset, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, tenantID)
	}
	return []domain.Asset{}, nil
}

func (m *MockAssetRepository) Update(ctx context.Context, a *domain.Asset) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, a)
	}
	return nil
}

func (m *MockAssetRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id, tenantID)
	}
	return nil
}

func (m *MockAssetRepository) CreateSnapshot(ctx context.Context, s *domain.AssetSnapshot) error {
	if m.createSnapshotFunc != nil {
		return m.createSnapshotFunc(ctx, s)
	}
	return nil
}

func (m *MockAssetRepository) ListSnapshots(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetSnapshot, error) {
	if m.listSnapshotsFunc != nil {
		return m.listSnapshotsFunc(ctx, assetID, tenantID)
	}
	return []domain.AssetSnapshot{}, nil
}

// MockAssetDependencyRepository is a hand-rolled mock implementing
// domain.AssetDependencyRepository (same function-field pattern as above).
type MockAssetDependencyRepository struct {
	createFunc        func(ctx context.Context, d *domain.AssetDependency) error
	getByIDFunc       func(ctx context.Context, id, tenantID uuid.UUID) (*domain.AssetDependency, error)
	listByTenantFunc  func(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error)
	listByAssetFunc   func(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetDependency, error)
	existsFunc        func(ctx context.Context, tenantID, src, tgt uuid.UUID, t domain.DependencyType) (bool, error)
	deleteFunc        func(ctx context.Context, id, tenantID uuid.UUID) error
	deleteByAssetFunc func(ctx context.Context, assetID, tenantID uuid.UUID) error
}

func (m *MockAssetDependencyRepository) Create(ctx context.Context, d *domain.AssetDependency) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, d)
	}
	return nil
}

func (m *MockAssetDependencyRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.AssetDependency, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id, tenantID)
	}
	return nil, nil
}

func (m *MockAssetDependencyRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	if m.listByTenantFunc != nil {
		return m.listByTenantFunc(ctx, tenantID)
	}
	return []domain.AssetDependency{}, nil
}

func (m *MockAssetDependencyRepository) ListByAsset(ctx context.Context, assetID, tenantID uuid.UUID) ([]domain.AssetDependency, error) {
	if m.listByAssetFunc != nil {
		return m.listByAssetFunc(ctx, assetID, tenantID)
	}
	return []domain.AssetDependency{}, nil
}

func (m *MockAssetDependencyRepository) Exists(ctx context.Context, tenantID, src, tgt uuid.UUID, t domain.DependencyType) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, tenantID, src, tgt, t)
	}
	return false, nil
}

func (m *MockAssetDependencyRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id, tenantID)
	}
	return nil
}

func (m *MockAssetDependencyRepository) DeleteByAsset(ctx context.Context, assetID, tenantID uuid.UUID) error {
	if m.deleteByAssetFunc != nil {
		return m.deleteByAssetFunc(ctx, assetID, tenantID)
	}
	return nil
}
