// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

// #1518: registration writes the RBAC mirror (tenants row, root's user_tenants
// row with the predefined Admin role) in its one transaction. These tests run
// the real use case over the real GORM repositories on sqlite, because the
// property is what the database holds afterwards.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/registrationdb"
)

func realRegisterUseCase(db *gorm.DB, withWriter bool) *RegisterUseCase {
	uc := NewRegisterUseCase(repository.NewGormUserRepository(db), repository.NewGormOrganizationRepository(db), nil, registerHasher{})
	if withWriter {
		uc = uc.WithAccounts(repository.NewGormRegistrationRepository(db))
	}
	return uc
}

func TestRegister_Success_WritesTenantMirrorAndAdminBinding(t *testing.T) {
	db, admin := registrationdb.Open(t, true)

	out, err := realRegisterUseCase(db, true).Execute(context.Background(), validInput())
	require.NoError(t, err)

	var tenant domain.Tenant
	require.NoError(t, db.First(&tenant, "id = ?", out.Organization.ID).Error, "tenants.id must equal the organisation id")
	assert.Equal(t, out.Organization.Slug, tenant.Slug)
	assert.Equal(t, out.User.ID, tenant.OwnerID)

	var bindings []domain.UserTenant
	require.NoError(t, db.Where("user_id = ?", out.User.ID).Find(&bindings).Error)
	require.Len(t, bindings, 1)
	assert.Equal(t, out.Organization.ID, bindings[0].TenantID)
	assert.Equal(t, admin.ID, bindings[0].RoleID)
}

func TestRegister_NotFound_AdminRoleAbsentPersistsNothing(t *testing.T) {
	db, _ := registrationdb.Open(t, false)

	out, err := realRegisterUseCase(db, true).Execute(context.Background(), validInput())

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, out)
	for _, m := range []any{&domain.Organization{}, &domain.User{}, &domain.OrganizationMember{}, &domain.Tenant{}, &domain.UserTenant{}} {
		var n int64
		require.NoError(t, db.Unscoped().Model(m).Count(&n).Error)
		assert.EqualValues(t, 0, n, "%T", m)
	}
}

// Registration itself is unauthenticated, so "unauthorized" is read as: the new
// root is bound to its OWN tenant only (no row for any other existing tenant),
// and a use case with no account writer refuses instead of writing unprotected.
func TestRegister_Unauthorized_BindingCoversOnlyTheOwnTenant(t *testing.T) {
	db, _ := registrationdb.Open(t, true)
	other := uuid.New()
	require.NoError(t, db.Create(&domain.Tenant{ID: other, Name: "Other", Slug: "other", OwnerID: uuid.New()}).Error)

	out, err := realRegisterUseCase(db, true).Execute(context.Background(), validInput())
	require.NoError(t, err)

	var foreign int64
	require.NoError(t, db.Model(&domain.UserTenant{}).Where("user_id = ? AND tenant_id = ?", out.User.ID, other).Count(&foreign).Error)
	assert.EqualValues(t, 0, foreign, "the root must hold no binding to another tenant")

	_, err = realRegisterUseCase(db, false).Execute(context.Background(), RegisterInput{
		Email: "second@beta.test", Password: "Ancre-Vitrail7-Cobalt", FullName: "B", CompanyName: "Beta",
	})
	require.Error(t, err)
	var appErr *domain.AppError
	require.ErrorAs(t, err, &appErr)
	assert.ErrorIs(t, appErr, domain.ErrInternal)
}
