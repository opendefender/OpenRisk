// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormMFARepository implements MFARepository using GORM
type GormMFARepository struct {
	db *gorm.DB
}

// NewGormMFARepository creates a new GORM MFA repository
func NewGormMFARepository(db *gorm.DB) *GormMFARepository {
	return &GormMFARepository{db: db}
}

// CreateMFASecret creates a new MFA secret
func (r *GormMFARepository) CreateMFASecret(ctx context.Context, secret *domain.MFASecret) error {
	return r.db.WithContext(ctx).Create(secret).Error
}

// GetMFASecret retrieves MFA secret for user (Tenant-scoped)
func (r *GormMFARepository) GetMFASecret(ctx context.Context, userID, tenantID uuid.UUID) (*domain.MFASecret, error) {
	var secret domain.MFASecret
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		First(&secret).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &secret, nil
}

// UpdateMFASecret updates an existing MFA secret
func (r *GormMFARepository) UpdateMFASecret(ctx context.Context, secret *domain.MFASecret) error {
	return r.db.WithContext(ctx).Save(secret).Error
}

// DisableMFA disables MFA for user (soft delete)
func (r *GormMFARepository) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Delete(&domain.MFASecret{}).Error
}

// SaveBackupCodes saves backup codes in batch
func (r *GormMFARepository) SaveBackupCodes(ctx context.Context, codes []*domain.MFABackupCode) error {
	if len(codes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(codes, 100).Error
}

// GetUnusedBackupCodes retrieves unused backup codes (Tenant-scoped)
func (r *GormMFARepository) GetUnusedBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.MFABackupCode, error) {
	var codes []*domain.MFABackupCode
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND used_at IS NULL", userID, tenantID).
		Find(&codes).Error

	if err != nil {
		return nil, err
	}
	return codes, nil
}

// MarkBackupCodeAsUsed marks a backup code as used (Tenant-scoped)
func (r *GormMFARepository) MarkBackupCodeAsUsed(ctx context.Context, codeID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.MFABackupCode{}).
		Where("id = ?", codeID).
		Update("used_at", now).Error
}

// DeleteBackupCodes deletes all backup codes for user (Tenant-scoped)
func (r *GormMFARepository) DeleteBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Delete(&domain.MFABackupCode{}).Error
}

// GormOAuthProviderRepository implements OAuthProviderRepository using GORM
type GormOAuthProviderRepository struct {
	db *gorm.DB
}

// NewGormOAuthProviderRepository creates a new GORM OAuth provider repository
func NewGormOAuthProviderRepository(db *gorm.DB) *GormOAuthProviderRepository {
	return &GormOAuthProviderRepository{db: db}
}

// CreateOAuthProvider creates a new OAuth provider link
func (r *GormOAuthProviderRepository) CreateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// GetOAuthProvider retrieves OAuth provider by user and provider name (Tenant-scoped)
func (r *GormOAuthProviderRepository) GetOAuthProvider(ctx context.Context, userID, tenantID uuid.UUID, providerName string) (*domain.OAuthProvider, error) {
	var provider domain.OAuthProvider
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND provider = ?", userID, tenantID, providerName).
		First(&provider).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// GetOAuthProviderByEmail retrieves OAuth provider by email and provider name (NO tenant filter - for OAuth flow)
func (r *GormOAuthProviderRepository) GetOAuthProviderByEmail(ctx context.Context, email, provider string) (*domain.OAuthProvider, error) {
	var oauthProvider domain.OAuthProvider
	err := r.db.WithContext(ctx).
		Where("email = ? AND provider = ?", email, provider).
		First(&oauthProvider).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &oauthProvider, nil
}

// UpdateOAuthProvider updates an existing OAuth provider
func (r *GormOAuthProviderRepository) UpdateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// ListOAuthProviders lists all OAuth providers for user (Tenant-scoped)
func (r *GormOAuthProviderRepository) ListOAuthProviders(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.OAuthProvider, error) {
	var providers []*domain.OAuthProvider
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Find(&providers).Error

	if err != nil {
		return nil, err
	}
	return providers, nil
}

// DeleteOAuthProvider deletes an OAuth provider link (Tenant-scoped)
func (r *GormOAuthProviderRepository) DeleteOAuthProvider(ctx context.Context, providerID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", providerID, tenantID).
		Delete(&domain.OAuthProvider{}).Error
}
