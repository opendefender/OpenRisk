// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// updateInTenant writes every column of model (Save semantics, associations
// included) but only if the row belongs to tenantID. 0 rows is domain.ErrNotFound.
//
// It exists because `tx.Where("tenant_id = ?", t).Save(&x)` is a trap: in GORM
// v1.31 Save falls back to INSERT ... ON CONFLICT UPDATE ALL when its UPDATE
// matches 0 rows, so a row of another tenant with the same id would be
// overwritten. Never `.Save(` a tenant-scoped model (#808, D-110).
func updateInTenant(db *gorm.DB, tenantID uuid.UUID, model any, omit ...string) error {
	return updateScoped(db, "tenant_id", tenantID, model, omit...)
}

// updateInOrganization is updateInTenant for a table keyed by organization_id.
func updateInOrganization(db *gorm.DB, organizationID uuid.UUID, model any, omit ...string) error {
	return updateScoped(db, "organization_id", organizationID, model, omit...)
}

func updateScoped(db *gorm.DB, column string, id uuid.UUID, model any, omit ...string) error {
	if id == uuid.Nil {
		return domain.NewUnauthorizedError("no tenant in context")
	}
	q := db.Model(model).Where(column+" = ?", id).Select("*")
	if len(omit) > 0 {
		q = q.Omit(omit...)
	}
	res := q.Updates(model)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("record", "")
	}
	return nil
}
