// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/tenantwrite"
)

// updateInTenant and updateInOrganization are the repository-side names of
// tenantwrite.UpdateInTenant / UpdateInOrganization. Never `.Save(` a
// tenant-scoped model (#808, D-111).
func updateInTenant(db *gorm.DB, tenantID uuid.UUID, model any, omit ...string) error {
	return tenantwrite.UpdateInTenant(db, tenantID, model, omit...)
}

func updateInOrganization(db *gorm.DB, organizationID uuid.UUID, model any, omit ...string) error {
	return tenantwrite.UpdateInOrganization(db, organizationID, model, omit...)
}
