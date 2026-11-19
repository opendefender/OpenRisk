// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// GormRiskAssetStore implements risk.RiskAssetStore: it resolves a tenant's
// assets and writes a risk with its asset links in one transaction (#792).
type GormRiskAssetStore struct {
	db *gorm.DB
}

func NewGormRiskAssetStore(db *gorm.DB) *GormRiskAssetStore {
	return &GormRiskAssetStore{db: db}
}

// FindByIDs returns the tenant's assets among ids; foreign or unknown ids are
// simply absent.
func (s *GormRiskAssetStore) FindByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*domain.Asset, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}
	assets := []*domain.Asset{}
	if len(ids) == 0 {
		return assets, nil
	}
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("failed to find assets: %w", err)
	}
	return assets, nil
}

// SaveWithAssets writes the risk and replaces its links in risk_assets.
//
// risk_assets has no tenant_id: it is gated through its two parents. The risk
// is the caller's (created here, or loaded tenant-scoped by the use case) and
// every asset must carry the risk's tenant, checked before anything is written.
func (s *GormRiskAssetStore) SaveWithAssets(ctx context.Context, risk *domain.Risk, assets []*domain.Asset, create bool) error {
	if risk.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	for _, a := range assets {
		if a == nil || a.TenantID != risk.TenantID {
			return fmt.Errorf("asset does not belong to the risk's tenant")
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The links are written by Replace below, not as a side effect of the
		// row write, so a removed asset is unlinked rather than left behind.
		write := tx.Omit(clause.Associations)
		var err error
		if create {
			err = write.Create(risk).Error
		} else {
			err = updateInTenant(write, risk.TenantID, risk)
		}
		if err != nil {
			return err
		}
		// GORM re-issues an id-only UPDATE on the owner row while replacing a
		// many2many. The risk was written above under its tenant and every asset
		// was checked against that tenant, so the statement is allowlisted.
		link := tenantguard.CrossTenant(tx, "association replace re-writes the risk this transaction just updated under its tenant")
		if err := link.Model(risk).Association("Assets").Replace(assets); err != nil {
			return fmt.Errorf("failed to link assets: %w", err)
		}
		return nil
	})
}
