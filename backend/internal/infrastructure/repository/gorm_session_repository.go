// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// sessionRow maps the refresh_tokens table locally.
//
// It deliberately does NOT reuse internal/auth.RefreshToken: internal/auth
// already imports this package (for the audit store), so importing it back here
// would close an import cycle. The two structs describe the same table and only
// the columns this projection reads are declared.
type sessionRow struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	TokenHash         string
	DeviceFingerprint string
	IPAddress         string
	UserAgent         string
	ExpiresAt         time.Time
	LastUsedAt        *time.Time
	CreatedAt         time.Time
}

func (sessionRow) TableName() string { return "refresh_tokens" }

// GormSessionRepository projects refresh tokens as user-visible sessions.
//
// Every query is scoped by user_id. That predicate is the whole authorisation
// model here: session IDs are UUIDs handed to the browser, and without it a
// caller could revoke a stranger's session by presenting its ID.
type GormSessionRepository struct {
	db *gorm.DB
}

// NewGormSessionRepository builds the repository.
func NewGormSessionRepository(db *gorm.DB) *GormSessionRepository {
	return &GormSessionRepository{db: db}
}

// ListByUser returns a user's live sessions, most recently used first.
//
// Expired rows are filtered rather than shown greyed out: a session that can no
// longer be refreshed is not something anyone needs to act on, and listing it
// invites pointless "revoke" clicks on a credential that is already dead.
func (r *GormSessionRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.SessionRecord, error) {
	var tokens []sessionRow
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("COALESCE(last_used_at, created_at) DESC").
		Find(&tokens).Error
	if err != nil {
		return nil, err
	}

	records := make([]domain.SessionRecord, 0, len(tokens))
	for _, t := range tokens {
		records = append(records, domain.SessionRecord{
			ID:         t.ID,
			TokenHash:  t.TokenHash,
			Device:     domain.DescribeDevice(t.UserAgent),
			UserAgent:  t.UserAgent,
			IPAddress:  t.IPAddress,
			CreatedAt:  t.CreatedAt,
			LastUsedAt: t.LastUsedAt,
			ExpiresAt:  t.ExpiresAt,
		})
	}
	return records, nil
}

// Revoke deletes one session belonging to the user, reporting whether it existed.
func (r *GormSessionRepository) Revoke(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Delete(&sessionRow{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// RevokeAllExcept signs out every device but the one holding keepHash.
//
// When keepHash is empty this revokes everything, which is what the password
// reset path wants.
func (r *GormSessionRepository) RevokeAllExcept(ctx context.Context, userID uuid.UUID, keepHash string) (int64, error) {
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if keepHash != "" {
		q = q.Where("token_hash <> ?", keepHash)
	}
	res := q.Delete(&sessionRow{})
	return res.RowsAffected, res.Error
}

// HasSeenDevice reports whether this user ever held a session on a fingerprint.
//
// Deliberately ignores expiry: the question is "have I seen this device before",
// and a device someone used last month is still familiar to them. Filtering on
// live sessions would fire a "new device" alert every time a token aged out,
// training people to ignore the alert that matters.
func (r *GormSessionRepository) HasSeenDevice(ctx context.Context, userID uuid.UUID, fingerprint string) (bool, error) {
	if fingerprint == "" {
		// No signal: report "seen" so the caller stays silent rather than alerting
		// on every sign-in from a client that sends no fingerprint.
		return true, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&sessionRow{}).
		Where("user_id = ? AND device_fingerprint = ?", userID, fingerprint).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
