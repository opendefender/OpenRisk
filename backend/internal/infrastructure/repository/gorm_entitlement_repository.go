// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"gorm.io/gorm"
)

// GormEntitlementRepository satisfies the application entitlements ports
// (OrgReader + UsageCounter) with tenant-scoped COUNT queries.
type GormEntitlementRepository struct {
	db *gorm.DB
}

func NewGormEntitlementRepository(db *gorm.DB) *GormEntitlementRepository {
	return &GormEntitlementRepository{db: db}
}

// PlanAndRegion reads the organization's stored plan and region default. Region
// lives in the org settings JSON ("region": "africa"); absent → EU.
func (r *GormEntitlementRepository) PlanAndRegion(ctx context.Context, tenant uuid.UUID) (string, string, error) {
	var org domain.Organization
	if err := r.db.WithContext(ctx).
		Select("plan", "settings").
		Where("id = ?", tenant).
		First(&org).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return string(domain.PlanFree), string(ent.RegionEU), nil
		}
		return "", "", err
	}
	region := string(ent.RegionEU)
	if s := org.GetSettings(); s != nil {
		if v, ok := s["region"].(string); ok && v != "" {
			region = v
		}
	}
	return string(org.Plan), region, nil
}

// Count returns the current usage of a capped resource for a tenant. "Integrations"
// aggregates the two configurable outbound connectors (vulnerability integrations +
// scanner configs). Unknown keys count as 0 (never blocks).
func (r *GormEntitlementRepository) Count(ctx context.Context, tenant uuid.UUID, key ent.LimitKey) (int, error) {
	db := r.db.WithContext(ctx)
	var n int64
	switch key {
	case ent.LimitUsers:
		if err := db.Model(&domain.OrganizationMember{}).
			Where("organization_id = ? AND is_active = ?", tenant, true).Count(&n).Error; err != nil {
			return 0, err
		}
	case ent.LimitRisks:
		if err := db.Model(&domain.Risk{}).
			Where("tenant_id = ? AND deleted_at IS NULL", tenant).Count(&n).Error; err != nil {
			return 0, err
		}
	case ent.LimitAssets:
		if err := db.Model(&domain.Asset{}).
			Where("tenant_id = ? AND deleted_at IS NULL", tenant).Count(&n).Error; err != nil {
			return 0, err
		}
	case ent.LimitIntegrations:
		var vi, sc int64
		if err := db.Model(&domain.VulnIntegration{}).Where("tenant_id = ?", tenant).Count(&vi).Error; err != nil {
			return 0, err
		}
		if err := db.Model(&domain.ScanConfig{}).Where("tenant_id = ?", tenant).Count(&sc).Error; err != nil {
			return 0, err
		}
		n = vi + sc
	default:
		return 0, nil
	}
	return int(n), nil
}
