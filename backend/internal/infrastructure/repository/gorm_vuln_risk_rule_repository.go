// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormVulnRiskRuleRepository persists the tenant's vulnerability→risk rule.
//
// ABSOLUTE RULE #2: tenant-scoped on every query.
type GormVulnRiskRuleRepository struct {
	db *gorm.DB
}

func NewGormVulnRiskRuleRepository(db *gorm.DB) *GormVulnRiskRuleRepository {
	return &GormVulnRiskRuleRepository{db: db}
}

var _ domain.VulnRiskRuleRepository = (*GormVulnRiskRuleRepository)(nil)

// Get returns the tenant's rule, or (nil, nil) when they have none.
//
// Deliberately NOT seeded on read (unlike the asset schemas): the default rule
// is disabled, and a nil rule and a disabled rule behave identically, so there
// is nothing to gain from writing a row before the tenant has an opinion.
func (r *GormVulnRiskRuleRepository) Get(ctx context.Context, tenantID uuid.UUID) (*domain.VulnRiskRule, error) {
	var rule domain.VulnRiskRule
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&rule).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *GormVulnRiskRuleRepository) Upsert(ctx context.Context, rule *domain.VulnRiskRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled", "min_cvss", "require_internet_exposure", "min_asset_criticality",
			"require_kev", "require_asset", "notify_on_create", "updated_at", "updated_by",
		}),
	}).Create(rule).Error
}

// GormAssetExposureLookup answers the rule's "is this asset internet-exposed?"
// condition from the asset's typed attributes.
type GormAssetExposureLookup struct {
	db *gorm.DB
}

func NewGormAssetExposureLookup(db *gorm.DB) *GormAssetExposureLookup {
	return &GormAssetExposureLookup{db: db}
}

// IsInternetExposed reads the asset and asks the domain.
//
// Returns false on any error or unknown asset. That is the safe direction for a
// rule that CREATES records: an unreadable asset makes an exposure-gated rule
// fire less, never more.
func (l *GormAssetExposureLookup) IsInternetExposed(ctx context.Context, tenantID, assetID uuid.UUID) bool {
	var a domain.Asset
	if err := l.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", assetID, tenantID).
		First(&a).Error; err != nil {
		return false
	}
	return domain.IsInternetExposed(&a)
}
