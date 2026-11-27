// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/registrationdb"
)

func newAccount(slug string) (*domain.Organization, *domain.User, *domain.OrganizationMember) {
	now := time.Now()
	userID := uuid.New()
	org := &domain.Organization{ID: uuid.New(), Name: "Alpha", Slug: slug, Plan: domain.PlanFree, OwnerID: userID, IsActive: true}
	user := &domain.User{ID: userID, Email: userID.String() + "@alpha.test", Username: userID.String(), Password: "x", FullName: "A", DefaultOrgID: &org.ID, IsActive: true}
	member := &domain.OrganizationMember{OrganizationID: org.ID, UserID: userID, Role: domain.RoleRoot, IsActive: true, JoinedAt: now}
	return org, user, member
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Unscoped().Model(model).Count(&n).Error)
	return n
}

func TestCreateAccount_WritesTenantMirrorAndRootBinding(t *testing.T) {
	db, admin := registrationdb.Open(t, true)
	org, user, member := newAccount("alpha")

	require.NoError(t, repository.NewGormRegistrationRepository(db).CreateAccount(context.Background(), org, user, member))

	var tenant domain.Tenant
	require.NoError(t, db.First(&tenant, "id = ?", org.ID).Error)
	assert.Equal(t, "alpha", tenant.Slug)
	assert.Equal(t, user.ID, tenant.OwnerID)
	var ut domain.UserTenant
	require.NoError(t, db.First(&ut, "user_id = ? AND tenant_id = ?", user.ID, org.ID).Error)
	assert.Equal(t, admin.ID, ut.RoleID)
}

func TestCreateAccount_SlugHeldByATenantFallsBackToTheID(t *testing.T) {
	db, _ := registrationdb.Open(t, true)
	// A soft-deleted tenant still holds the slug in the unique index.
	require.NoError(t, db.Create(&domain.Tenant{ID: uuid.New(), Name: "old", Slug: "alpha", OwnerID: uuid.New(), DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}}).Error)
	org, user, member := newAccount("alpha")

	require.NoError(t, repository.NewGormRegistrationRepository(db).CreateAccount(context.Background(), org, user, member))

	var tenant domain.Tenant
	require.NoError(t, db.First(&tenant, "id = ?", org.ID).Error)
	assert.Equal(t, org.ID.String(), tenant.Slug)
}

func TestCreateAccount_AdminRoleAbsentIsNotFoundAndRollsBack(t *testing.T) {
	db, _ := registrationdb.Open(t, false)
	org, user, member := newAccount("alpha")

	err := repository.NewGormRegistrationRepository(db).CreateAccount(context.Background(), org, user, member)

	require.ErrorIs(t, err, domain.ErrNotFound)
	assertNothingPersisted(t, db)
}

// A failure of the LAST write (the user_tenants row) leaves no row anywhere.
func TestCreateAccount_FailureOfTheLastWriteLeavesNoRows(t *testing.T) {
	db, admin := registrationdb.Open(t, true)
	org, user, member := newAccount("alpha")
	// Same primary key as the row registration is about to insert.
	require.NoError(t, db.Create(&domain.UserTenant{UserID: user.ID, TenantID: org.ID, RoleID: admin.ID}).Error)

	err := repository.NewGormRegistrationRepository(db).CreateAccount(context.Background(), org, user, member)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create user tenant")
	assert.EqualValues(t, 0, countRows(t, db, &domain.Organization{}))
	assert.EqualValues(t, 0, countRows(t, db, &domain.User{}))
	assert.EqualValues(t, 0, countRows(t, db, &domain.OrganizationMember{}))
	assert.EqualValues(t, 0, countRows(t, db, &domain.Tenant{}))
	assert.EqualValues(t, 1, countRows(t, db, &domain.UserTenant{}), "only the pre-existing row")
}

func assertNothingPersisted(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, m := range []any{&domain.Organization{}, &domain.User{}, &domain.OrganizationMember{}, &domain.Tenant{}, &domain.UserTenant{}} {
		assert.EqualValues(t, 0, countRows(t, db, m), "%T", m)
	}
}
