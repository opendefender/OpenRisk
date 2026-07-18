// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormIncidentCounter counts past incidents recorded against an asset for the
// smart-risk "incident history" factor (spec §8, factor 5). The legacy incident
// model has no clean foreign key to an asset (RiskID is a *uint that cannot match
// the uuid risk PKs — see ROADMAP), so this matches best-effort on the asset id /
// name inside the incident's impacted_assets JSON or title. Tenant-scoped.
type GormIncidentCounter struct {
	db *gorm.DB
}

func NewGormIncidentCounter(db *gorm.DB) *GormIncidentCounter {
	return &GormIncidentCounter{db: db}
}

// CountForAsset returns how many non-deleted incidents in the tenant reference the
// given asset (by id in impacted_assets, or by name in impacted_assets/title).
func (c *GormIncidentCounter) CountForAsset(ctx context.Context, tenantID uuid.UUID, assetID uuid.UUID, assetName string) (int, error) {
	idLike := "%" + assetID.String() + "%"
	q := c.db.WithContext(ctx).
		Model(&domain.Incident{}).
		Where("tenant_id = ?", tenantID.String()).
		Where("impacted_assets::text ILIKE ?", idLike)

	// Only widen the match on the asset name when it is specific enough to avoid
	// false positives (a 1-2 char name would match almost anything).
	if len(assetName) >= 3 {
		nameLike := "%" + assetName + "%"
		q = c.db.WithContext(ctx).
			Model(&domain.Incident{}).
			Where("tenant_id = ?", tenantID.String()).
			Where("impacted_assets::text ILIKE ? OR impacted_assets::text ILIKE ? OR title ILIKE ?",
				idLike, nameLike, nameLike)
	}

	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n), nil
}
