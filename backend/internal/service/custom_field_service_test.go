// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// setupCustomFieldDB builds the custom_fields/templates tables by hand — GORM's
// AutoMigrate would emit the Postgres-only `gen_random_uuid()` default, which
// sqlite rejects (the service always sets the ID explicitly anyway).
func setupCustomFieldDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE custom_fields (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			display_name TEXT,
			description TEXT,
			field_type TEXT,
			scope TEXT,
			default_value TEXT,
			placeholder TEXT,
			validation TEXT,
			position INTEGER DEFAULT 0,
			visible NUMERIC DEFAULT 1,
			read_only NUMERIC DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);`).Error)
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX idx_tenant_name_scope ON custom_fields (tenant_id, name, scope) WHERE deleted_at IS NULL;`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE custom_field_templates (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			scope TEXT,
			fields TEXT NOT NULL,
			is_public NUMERIC DEFAULT 1,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);`).Error)
	return db
}

func newField(name string) *domain.CreateCustomFieldRequest {
	return &domain.CreateCustomFieldRequest{
		Name:      name,
		FieldType: domain.CustomFieldTypeText,
		Scope:     domain.CustomFieldScopeRisk,
		Visible:   true,
	}
}

// TestCustomField_TenantIsolation proves that custom fields — which used to have
// NO tenant column at all (every CRUD op was global) — are now fully scoped:
// tenant B can neither see, read, update nor delete tenant A's field, and both
// tenants may independently define a field with the same name (RULE #2).
func TestCustomField_TenantIsolation(t *testing.T) {
	db := setupCustomFieldDB(t)
	svc := &CustomFieldService{db: db}

	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	fieldA, err := svc.CreateCustomField(tenantA, userA, newField("Department"))
	require.NoError(t, err)
	require.Equal(t, tenantA, fieldA.TenantID)

	// Both tenants can define the same field name (no global unique collision).
	fieldB, err := svc.CreateCustomField(tenantB, userB, newField("Department"))
	require.NoError(t, err)
	require.Equal(t, tenantB, fieldB.TenantID)

	// List is scoped: each tenant sees only its own field.
	aList, err := svc.ListCustomFields(tenantA, nil)
	require.NoError(t, err)
	require.Len(t, aList, 1)
	require.Equal(t, fieldA.ID, aList[0].ID)

	// Read across tenants is denied.
	_, err = svc.GetCustomField(tenantB, fieldA.ID)
	require.Error(t, err, "tenant B must not read tenant A's field")

	// Update across tenants is denied and does not mutate A's field.
	vis := false
	_, err = svc.UpdateCustomField(tenantB, fieldA.ID, &domain.UpdateCustomFieldRequest{DisplayName: "hijacked", Visible: &vis})
	require.Error(t, err)
	reloaded, err := svc.GetCustomField(tenantA, fieldA.ID)
	require.NoError(t, err)
	require.Empty(t, reloaded.DisplayName)

	// Delete across tenants is denied (ErrNotFound) and leaves A's field intact.
	err = svc.DeleteCustomField(tenantB, fieldA.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.GetCustomField(tenantA, fieldA.ID)
	require.NoError(t, err)

	// Owner can delete its own field.
	require.NoError(t, svc.DeleteCustomField(tenantA, fieldA.ID))
}

// TestCustomField_DuplicateWithinTenant keeps the same-tenant duplicate guard.
func TestCustomField_DuplicateWithinTenant(t *testing.T) {
	db := setupCustomFieldDB(t)
	svc := &CustomFieldService{db: db}
	tenant := uuid.New()
	user := uuid.New()

	_, err := svc.CreateCustomField(tenant, user, newField("Owner"))
	require.NoError(t, err)
	_, err = svc.CreateCustomField(tenant, user, newField("Owner"))
	require.Error(t, err, "a tenant cannot define the same field name twice for a scope")
}
