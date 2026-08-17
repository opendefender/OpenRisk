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

// GormBillingRepository persists subscriptions and invoices, tenant-scoped.
type GormBillingRepository struct {
	db *gorm.DB
}

func NewGormBillingRepository(db *gorm.DB) *GormBillingRepository {
	return &GormBillingRepository{db: db}
}

// GetByTenant returns the tenant's subscription, or (nil, nil) if it has none.
// Satisfies application/entitlements.SubscriptionReader.
func (r *GormBillingRepository) GetByTenant(ctx context.Context, tenant uuid.UUID) (*domain.Subscription, error) {
	var sub domain.Subscription
	err := r.db.WithContext(ctx).Where("organization_id = ?", tenant).First(&sub).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// Upsert creates or updates the tenant's single subscription row.
func (r *GormBillingRepository) Upsert(ctx context.Context, sub *domain.Subscription) error {
	existing, err := r.GetByTenant(ctx, sub.OrganizationID)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.db.WithContext(ctx).Create(sub).Error
	}
	sub.ID = existing.ID
	sub.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).
		Model(&domain.Subscription{}).
		Where("id = ? AND organization_id = ?", existing.ID, sub.OrganizationID).
		Save(sub).Error
}

// ListInvoices returns a tenant's invoices, newest first.
func (r *GormBillingRepository) ListInvoices(ctx context.Context, tenant uuid.UUID) ([]domain.Invoice, error) {
	var out []domain.Invoice
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", tenant).
		Order("created_at DESC").
		Find(&out).Error
	return out, err
}

// CreateInvoice records an invoice for a tenant.
func (r *GormBillingRepository) CreateInvoice(ctx context.Context, inv *domain.Invoice) error {
	return r.db.WithContext(ctx).Create(inv).Error
}
