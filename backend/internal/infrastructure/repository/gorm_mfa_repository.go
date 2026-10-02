// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

// UpdateMFASecret updates an existing MFA secret.
//
// last_totp_step is left out: the struct in hand may predate the last accepted
// code, and writing it back would lower the mark and re-open the replay #849
// closes. ConsumeTOTPStep is its only writer.
func (r *GormMFARepository) UpdateMFASecret(ctx context.Context, secret *domain.MFASecret) error {
	return r.db.WithContext(ctx).Omit("last_totp_step").Save(secret).Error
}

// ConsumeTOTPStep records that a code for step was accepted, and reports false
// when a code for that step or a later one already was (#849).
//
// One conditional UPDATE does the check and the write, so two requests racing
// with the same code cannot both win: Postgres serialises them on the row, and
// the second re-evaluates the WHERE after the first commits.
func (r *GormMFARepository) ConsumeTOTPStep(ctx context.Context, userID, tenantID uuid.UUID, step int64) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.MFASecret{}).
		Where("user_id = ? AND tenant_id = ? AND (last_totp_step IS NULL OR last_totp_step < ?)", userID, tenantID, step).
		Updates(map[string]any{"last_totp_step": step, "last_used_at": time.Now()})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// DisableMFA removes the TOTP secret and every backup code of one user, in one
// transaction (#754).
//
// Both or neither: backup codes that survive their secret are a second factor
// nobody can see or revoke from the UI, and a secret that survives its codes
// leaves an account the user believes is unprotected still demanding a code.
//
// The secret is hard-deleted. It is key material, so a soft-deleted copy has no
// business lingering, and mfa_secrets.user_id is UNIQUE without a deleted_at
// clause — a tombstone would make the next enrolment fail on that constraint.
func (r *GormMFARepository) DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("user_id = ? AND tenant_id = ?", userID, tenantID).
			Delete(&domain.MFASecret{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
			Delete(&domain.MFABackupCode{}).Error
	})
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
