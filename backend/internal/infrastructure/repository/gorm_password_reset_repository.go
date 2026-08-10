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
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormPasswordResetRepository stores password-reset requests.
//
// Unlike most repositories here there is no tenant predicate, and that is
// correct rather than an oversight: a reset is requested from the login screen,
// where there is no session and therefore no tenant. The scoping key is the
// token secret itself — 256 bits, stored hashed, and bound to exactly one user.
type GormPasswordResetRepository struct {
	db *gorm.DB
}

// NewGormPasswordResetRepository builds the repository.
func NewGormPasswordResetRepository(db *gorm.DB) *GormPasswordResetRepository {
	return &GormPasswordResetRepository{db: db}
}

// Create records a reset request.
func (r *GormPasswordResetRepository) Create(ctx context.Context, token *domain.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// FindByTokenHash returns the request for a token hash, or (nil, nil) when there
// is none. A miss is not an error: an unknown token is an ordinary outcome on a
// public endpoint, and the caller collapses it into the same answer as expired.
func (r *GormPasswordResetRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error) {
	if tokenHash == "" {
		return nil, nil
	}
	var token domain.PasswordResetToken
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// ClaimToken marks a token used, and reports whether this caller was the one to
// do it.
//
// The `used_at IS NULL` predicate is the whole mechanism. Two requests carrying
// the same link race here; the database serialises them on the row and exactly
// one sees RowsAffected == 1. Reading the row and then writing it would let both
// observe "unused" before either wrote, and both would proceed.
func (r *GormPasswordResetRepository) ClaimToken(ctx context.Context, id uuid.UUID) (bool, error) {
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&domain.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// CountRecentByEmailHash counts requests for an address inside the window.
//
// Counts rows for the ADDRESS, not the account, so requests naming an address
// with no account are counted too. That is what stops the rate limiter from
// becoming an account-existence oracle.
func (r *GormPasswordResetRepository) CountRecentByEmailHash(ctx context.Context, emailHash string, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.PasswordResetToken{}).
		Where("email_hash = ? AND created_at >= ?", emailHash, since).
		Count(&count).Error
	return count, err
}

// InvalidateOutstandingForUser spends every other live token for a user.
//
// Called right after a successful reset: if an attacker had also requested a
// link, theirs must not outlive the legitimate change.
func (r *GormPasswordResetRepository) InvalidateOutstandingForUser(ctx context.Context, userID uuid.UUID, except uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.PasswordResetToken{}).
		Where("user_id = ? AND id <> ? AND used_at IS NULL", userID, except).
		Update("used_at", now).Error
}
