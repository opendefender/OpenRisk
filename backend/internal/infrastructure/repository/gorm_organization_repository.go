// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormOrganizationRepository implements OrganizationRepository using GORM
type GormOrganizationRepository struct {
	db *gorm.DB
}

// NewGormOrganizationRepository creates a new GORM organization repository
func NewGormOrganizationRepository(db *gorm.DB) *GormOrganizationRepository {
	return &GormOrganizationRepository{db: db}
}

// Create creates a new organization
func (r *GormOrganizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	return r.db.WithContext(ctx).Create(org).Error
}

// GetByID retrieves an organization by ID
func (r *GormOrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).First(&org, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}

// GetBySlug retrieves an organization by slug
func (r *GormOrganizationRepository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&org).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}

// Update updates an existing organization
func (r *GormOrganizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	return r.db.WithContext(ctx).Save(org).Error
}

// Delete deletes an organization
func (r *GormOrganizationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.Organization{}, "id = ?", id).Error
}

// SlugExists checks if a slug already exists
func (r *GormOrganizationRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Organization{}).Where("slug = ?", slug).Count(&count).Error
	return count > 0, err
}