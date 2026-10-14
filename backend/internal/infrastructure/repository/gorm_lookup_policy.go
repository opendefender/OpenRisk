// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormLookupPolicy reads a tenant's online-lookup switch (#910, D-071) from
// its organization's settings. The organization id is the tenant id.
type GormLookupPolicy struct{ db *gorm.DB }

// NewGormLookupPolicy builds the policy.
func NewGormLookupPolicy(db *gorm.DB) *GormLookupPolicy { return &GormLookupPolicy{db: db} }

// OnlineLookupEnabled is false when the tenant switched lookups off, and also
// when the setting cannot be read: without a confirmed yes, nothing is sent.
func (p *GormLookupPolicy) OnlineLookupEnabled(ctx context.Context, tenantID uuid.UUID) bool {
	var org domain.Organization
	if err := p.db.WithContext(ctx).Select("id", "settings").
		Where("id = ?", tenantID).First(&org).Error; err != nil {
		return false
	}
	return domain.OnlineVulnLookupEnabled(org.GetSettings())
}
