// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

// GormRegistrationRepository writes a whole new account in ONE transaction:
// the organization, its owner, and the owner's root membership (#687).
//
// Registration used to write those three rows with three independent calls.
// A failure of the second deleted the first best-effort; a failure of the third
// was printed and ignored, which left a user who could sign in but belonged to
// no organization — an account with no permissions and no way to fix itself.
// ABSOLUTE RULE #7 asks for a transaction on every multi-table operation, and
// this is the clearest case of it in the codebase.
type GormRegistrationRepository struct {
	db *gorm.DB
}

func NewGormRegistrationRepository(db *gorm.DB) *GormRegistrationRepository {
	return &GormRegistrationRepository{db: db}
}

// CreateAccount inserts the organization, the user, the membership and the RBAC
// mirror of the account together (#1518).
//
// The mirror is a `tenants` row with id = organization id (the JWT's tenant is
// the organization id) and the root's `user_tenants` row carrying the predefined
// Admin role. Without them the legacy /rbac/* routes answered 403/404 to a
// freshly registered root until the next boot's SeedRBAC repaired it.
//
// The caller assigns the user's id before calling, so the organization can carry
// its real owner from the first insert rather than a placeholder that a later
// UPDATE had to correct — an update whose failure used to leave the row claiming
// an owner that never existed.
//
// Either all rows are committed, or none is. The Admin role is resolved inside
// the transaction and its absence fails closed with domain.ErrNotFound: a root
// with no role would be the half-made account #687 removed.
func (r *GormRegistrationRepository) CreateAccount(
	ctx context.Context,
	org *domain.Organization,
	user *domain.User,
	member *domain.OrganizationMember,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(org).Error; err != nil {
			return fmt.Errorf("create organization: %w", err)
		}
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("create membership: %w", err)
		}

		var admin domain.RoleEnhanced
		// Predefined roles are shared by every tenant and hold no tenant of their
		// own (same lookup as RoleService.GetPredefinedRoleByName), so the read is
		// cross-tenant by nature.
		err := tenantguard.CrossTenant(tx, "predefined Admin role is shared by every tenant and holds no tenant of its own").
			Where("is_predefined = ? AND name = ? AND deleted_at IS NULL", true, "Admin").
			First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("resolve predefined Admin role: %w", domain.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("resolve predefined Admin role: %w", err)
		}

		slug, err := r.tenantSlug(tx, org)
		if err != nil {
			return err
		}
		if err := tx.Create(&domain.Tenant{
			ID:       org.ID,
			Name:     org.Name,
			Slug:     slug,
			OwnerID:  user.ID,
			Status:   "active",
			IsActive: true,
		}).Error; err != nil {
			return fmt.Errorf("create tenant mirror: %w", err)
		}
		if err := tx.Create(&domain.UserTenant{
			UserID:   user.ID,
			TenantID: org.ID,
			RoleID:   admin.ID,
		}).Error; err != nil {
			return fmt.Errorf("create user tenant: %w", err)
		}
		return nil
	})
}

// tenantSlug is the organization's slug unless a tenants row, soft-deleted ones
// included (the unique index covers them), already holds it; then the id, as the
// boot seed does. A slug clash must never fail a registration.
func (r *GormRegistrationRepository) tenantSlug(tx *gorm.DB, org *domain.Organization) (string, error) {
	if org.Slug == "" {
		return org.ID.String(), nil
	}
	var n int64
	// tenants carries no tenant_id: it IS the tenant registry, and the slug index is global.
	if err := tx.Unscoped().Model(&domain.Tenant{}).Where("slug = ?", org.Slug).Count(&n).Error; err != nil {
		return "", fmt.Errorf("check tenant slug: %w", err)
	}
	if n > 0 {
		return org.ID.String(), nil
	}
	return org.Slug, nil
}
