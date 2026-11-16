// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package tenantwrite is the only way to rewrite a whole row of a tenant-scoped
// table (#808, D-111).
//
// `tx.Where("tenant_id = ?", t).Save(&x)` is a trap: in GORM v1.31 Save falls
// back to INSERT ... ON CONFLICT UPDATE ALL when its UPDATE matches 0 rows, so a
// row of another tenant with the same id would be overwritten. save_ratchet_test
// fails on any `.Save(` outside a closed list.
package tenantwrite

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// UpdateInTenant writes every column of model (Save semantics, associations
// included) but only if the row belongs to tenantID. It refuses, before any
// statement, a zero tenant, a zero primary key (which would match every row of
// the tenant) and a model whose own tenant_id is not tenantID. No matching row
// is domain.ErrNotFound.
func UpdateInTenant(db *gorm.DB, tenantID uuid.UUID, model any, omit ...string) error {
	return update(db, "tenant_id", tenantID, model, omit...)
}

// UpdateInOrganization is UpdateInTenant for a table keyed by organization_id.
func UpdateInOrganization(db *gorm.DB, organizationID uuid.UUID, model any, omit ...string) error {
	return update(db, "organization_id", organizationID, model, omit...)
}

func update(db *gorm.DB, column string, id uuid.UUID, model any, omit ...string) error {
	if id == uuid.Nil {
		return domain.NewUnauthorizedError("no tenant in context")
	}
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return fmt.Errorf("tenantwrite: parse %T: %w", model, err)
	}
	rv := reflect.ValueOf(model)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if len(stmt.Schema.PrimaryFields) == 0 {
		return domain.NewValidationError("tenantwrite: model has no primary key")
	}
	// The row is named by an explicit condition on every key column, so the
	// guarantee does not rest on the condition GORM adds from the model.
	type keyCond struct {
		col string
		val any
	}
	keys := make([]keyCond, 0, len(stmt.Schema.PrimaryFields))
	for _, pk := range stmt.Schema.PrimaryFields {
		v, zero := pk.ValueOf(ctx, rv)
		if zero {
			return domain.NewValidationError("primary key is required")
		}
		keys = append(keys, keyCond{pk.DBName, v})
	}
	owner := stmt.Schema.FieldsByDBName[column]
	if owner == nil {
		return domain.NewValidationError("tenantwrite: model has no " + column)
	}
	if v, _ := owner.ValueOf(ctx, rv); fmt.Sprint(v) != id.String() {
		return domain.NewForbiddenError("the record belongs to another tenant")
	}

	q := db.Model(model).Where(column+" = ?", id)
	for _, k := range keys {
		q = q.Where(k.col+" = ?", k.val)
	}
	q = q.Select("*")
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
