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
	"gorm.io/gorm/clause"
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

// StartMFAEnrolment stores a new unverified secret and replaces every backup
// code of the user, in one transaction (#714).
//
// The secret is written by one upsert, not a read and then a write: a first
// enrolment creates the row, and an unfinished one has its key replaced
// (#889). Two setups racing on an account with no row therefore both land,
// the later key winning, instead of the second hitting the unique user_id.
// The update only applies to an unverified row of the same tenant; a
// verified secret makes it affect nothing, and the call reports false and
// rolls back, leaving that secret and its codes untouched.
//
// A soft-deleted row counts as no secret, as it does for every read. Before
// #754, turning MFA off soft-deleted the secret, and that tombstone still
// holds the unique user_id: it is brought back as a fresh, unverified
// enrolment rather than updated out of sight or refused as "already enabled".
//
// last_totp_step and last_used_at are reset here explicitly rather than by a
// Save, which ConsumeTOTPStep's contract forbids (#849): they belonged to the
// abandoned key.
//
// The codes arrive already hashed. Hashing is slow, and must not happen while
// the row is locked.
func (r *GormMFARepository) StartMFAEnrolment(ctx context.Context, userID, tenantID uuid.UUID, secretEncrypted string, codes []*domain.MFABackupCode) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		secret := &domain.MFASecret{
			ID:              uuid.New(),
			UserID:          userID,
			TenantID:        tenantID,
			SecretEncrypted: secretEncrypted,
		}
		res := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"secret_encrypted": secretEncrypted,
				"is_verified":      false,
				"verified_at":      nil,
				"last_totp_step":   nil,
				"last_used_at":     nil,
				"deleted_at":       nil,
				"updated_at":       time.Now(),
			}),
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Expr{
					SQL:  "(mfa_secrets.is_verified = ? OR mfa_secrets.deleted_at IS NOT NULL) AND mfa_secrets.tenant_id = ?",
					Vars: []any{false, tenantID},
				},
			}},
		}).Create(secret)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errEnrolmentRefused
		}
		if err := tx.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
			Delete(&domain.MFABackupCode{}).Error; err != nil {
			return err
		}
		for _, c := range codes {
			c.UserID, c.TenantID = userID, tenantID
			if c.ID == uuid.Nil {
				c.ID = uuid.New()
			}
		}
		if len(codes) > 0 {
			if err := tx.CreateInBatches(codes, 100).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errEnrolmentRefused) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// errEnrolmentRefused rolls StartMFAEnrolment back when a verified secret is
// already in place. It never leaves the repository.
var errEnrolmentRefused = errors.New("mfa enrolment refused: secret already verified")

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
