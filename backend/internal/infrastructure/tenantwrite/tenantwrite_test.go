// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantwrite_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantwrite"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

type thing struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;index"`
	Name     string
}

type orgThing struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index"`
	Name           string
}

func setup(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&thing{}, &orgThing{}))
	return db
}

func names(t *testing.T, db *gorm.DB, tenant uuid.UUID) []string {
	t.Helper()
	var rows []thing
	require.NoError(t, db.Where("tenant_id = ?", tenant).Order("name").Find(&rows).Error)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func TestUpdateInTenant_Success(t *testing.T) {
	db := setup(t)
	tenant := uuid.New()
	row := &thing{ID: uuid.New(), TenantID: tenant, Name: "before"}
	require.NoError(t, db.Create(row).Error)

	row.Name = "after"
	require.NoError(t, tenantwrite.UpdateInTenant(db, tenant, row))
	assert.Equal(t, []string{"after"}, names(t, db, tenant))

	org := uuid.New()
	o := &orgThing{ID: uuid.New(), OrganizationID: org, Name: "x"}
	require.NoError(t, db.Create(o).Error)
	o.Name = "y"
	require.NoError(t, tenantwrite.UpdateInOrganization(db, org, o))
}

func TestUpdateInTenant_NotFound(t *testing.T) {
	// A row of another tenant with the same id is never overwritten, and never
	// inserted over: Save's upsert fallback must not happen.
	db := setup(t)
	owner, intruder := uuid.New(), uuid.New()
	row := &thing{ID: uuid.New(), TenantID: owner, Name: "owner's"}
	require.NoError(t, db.Create(row).Error)

	err := tenantwrite.UpdateInTenant(db, intruder, &thing{ID: row.ID, TenantID: intruder, Name: "intruder's"})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, []string{"owner's"}, names(t, db, owner))
	assert.Empty(t, names(t, db, intruder))
}

func TestUpdateInTenant_Unauthorized(t *testing.T) {
	db := setup(t)
	row := &thing{ID: uuid.New(), TenantID: uuid.New(), Name: "n"}
	require.NoError(t, db.Create(row).Error)
	assert.ErrorIs(t, tenantwrite.UpdateInTenant(db, uuid.Nil, row), domain.ErrUnauthorized)
}

func TestUpdateInTenant_ZeroPrimaryKeyTouchesNothing(t *testing.T) {
	// A zero primary key adds no id condition: it used to rewrite every row of
	// the tenant.
	db := setup(t)
	tenant := uuid.New()
	for _, n := range []string{"a", "b", "c"} {
		require.NoError(t, db.Create(&thing{ID: uuid.New(), TenantID: tenant, Name: n}).Error)
	}
	err := tenantwrite.UpdateInTenant(db, tenant, &thing{TenantID: tenant, Name: "overwrite"})
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, []string{"a", "b", "c"}, names(t, db, tenant))
}

func TestUpdateInTenant_ModelOfAnotherTenantIsForbidden(t *testing.T) {
	db := setup(t)
	a, b := uuid.New(), uuid.New()
	row := &thing{ID: uuid.New(), TenantID: a, Name: "a's"}
	require.NoError(t, db.Create(row).Error)
	// The caller claims tenant b but hands in a model stamped with tenant a.
	assert.ErrorIs(t, tenantwrite.UpdateInTenant(db, b, row), domain.ErrForbidden)
	assert.Equal(t, []string{"a's"}, names(t, db, a))
}
