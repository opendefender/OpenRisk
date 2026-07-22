// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormMemberRBACRepository is the tenant-scoped store behind the RBAC business
// role use cases. Every query filters by organization_id so a tenant admin can
// only ever see or modify their own organization's memberships (RULE #2).
type GormMemberRBACRepository struct {
	db *gorm.DB
}

// NewGormMemberRBACRepository builds the repository.
func NewGormMemberRBACRepository(db *gorm.DB) *GormMemberRBACRepository {
	return &GormMemberRBACRepository{db: db}
}

// GetMember returns the membership of userID in tenantID, or nil if none.
func (r *GormMemberRBACRepository) GetMember(ctx context.Context, tenantID, userID uuid.UUID) (*domain.OrganizationMember, error) {
	var m domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Profile.Permissions").
		Where("organization_id = ? AND user_id = ?", tenantID, userID).
		First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// ListMembers returns every membership of the tenant, User preloaded, newest
// first.
func (r *GormMemberRBACRepository) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.OrganizationMember, error) {
	var members []domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Profile.Permissions").
		Where("organization_id = ?", tenantID).
		Order("created_at DESC").
		Find(&members).Error
	return members, err
}

// UpdateMember persists role / business-role changes. Scoped by id AND
// organization_id so a forged/cross-tenant id can never write another tenant's
// row. Uses Select so an empty business_role (a deliberate clear) is written
// rather than skipped as a GORM zero value.
func (r *GormMemberRBACRepository) UpdateMember(ctx context.Context, member *domain.OrganizationMember) error {
	return r.db.WithContext(ctx).
		Model(&domain.OrganizationMember{}).
		Where("id = ? AND organization_id = ?", member.ID, member.OrganizationID).
		Select("role", "business_role").
		Updates(map[string]interface{}{
			"role":          member.Role,
			"business_role": member.BusinessRole,
		}).Error
}
