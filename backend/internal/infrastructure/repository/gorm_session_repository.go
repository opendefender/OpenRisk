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
	FamilyID          uuid.UUID
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
		// A rotated token is spent: it stays in the table for reuse detection
		// only, and its successor is the live session. Listing it would show one
		// device once per refresh.
		Where("user_id = ? AND expires_at > ? AND rotated_at IS NULL", userID, time.Now()).
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
			FamilyID:   t.FamilyID,
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
//
// It also untrusts every device cookie the user holds (D-064, #879); the caller
// re-issues one to the device that pressed it. See untrustDevices for why the
// stamp and the sweep are not one transaction.
func (r *GormSessionRepository) RevokeAllExcept(ctx context.Context, userID uuid.UUID, keepHash string) (int64, error) {
	if err := r.untrustDevices(ctx, userID); err != nil {
		return 0, err
	}
	if keepHash == "" {
		// Account-wide on purpose, no tenant_id: signing out other devices ends the
		// user's sessions in every org, as RevokeAllUserTokens does (D-047).
		return r.sweep(ctx, "user_id = ?", userID)
	}
	// Account-wide on purpose, no tenant_id: as above.
	return r.sweep(ctx, "user_id = ? AND token_hash <> ?", userID, keepHash)
}

// RevokeAllExceptFamily signs out every device but the session the caller's
// access token names. Keeping the whole family, rather than one row, keeps the
// caller's rotated tokens too, so a replay inside the grace window still gets
// the successor instead of tripping reuse detection.
//
// Like RevokeAllExcept, it untrusts every device cookie the user holds first.
func (r *GormSessionRepository) RevokeAllExceptFamily(ctx context.Context, userID, keepFamily uuid.UUID) (int64, error) {
	if err := r.untrustDevices(ctx, userID); err != nil {
		return 0, err
	}
	// Account-wide on purpose, no tenant_id: see RevokeAllExcept.
	return r.sweep(ctx, "user_id = ? AND family_id <> ?", userID, keepFamily)
}

// HasLiveSession reports whether the user holds a live, unexpired token in the
// family. Scoped by user_id, so a family id from someone else's token reads as
// absent.
func (r *GormSessionRepository) HasLiveSession(ctx context.Context, userID, family uuid.UUID) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&sessionRow{}).
		// Account-wide on purpose, no tenant_id: a session belongs to its user.
		Where("user_id = ? AND family_id = ? AND rotated_at IS NULL AND expires_at > ?", userID, family, time.Now()).
		Count(&n).Error
	return n > 0, err
}

// untrustDevices ends every trusted-device cookie the user holds by stamping
// users.devices_revoked_at (D-064, #879).
//
// Not in one transaction with the sweep, on purpose, as in the token manager's
// RevokeAllUserTokens: the sweep is only correct because each of its passes
// commits on its own. The stamp comes first, so a sweep that fails after it
// leaves the stricter state (devices untrusted), and the error is returned
// either way.
func (r *GormSessionRepository) untrustDevices(ctx context.Context, userID uuid.UUID) error {
	// Account-wide on purpose, no tenant_id: trust belongs to the user's browser,
	// not to one org.
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).
		UpdateColumn("devices_revoked_at", time.Now()).Error
}

// sweep deletes the refresh tokens matching the condition until a pass removes
// nothing, like the token manager's revocations (sweepRefreshTokens, #725): on
// Postgres a single DELETE misses a successor that another device's rotation
// stores while it runs, and that device would stay signed in.
//
// It returns the number of live sessions removed. Spent (rotated) rows go too,
// but they are not devices, and counting them would tell the user more
// devices were signed out than they ever saw in the list.
func (r *GormSessionRepository) sweep(ctx context.Context, cond string, args ...interface{}) (int64, error) {
	var live int64
	for i := 0; i < maxRevocationSweeps; i++ {
		// cond carries the user_id scope; see the callers.
		active := r.db.WithContext(ctx).Where(cond, args...).Where("rotated_at IS NULL").Delete(&sessionRow{})
		if active.Error != nil {
			return live, active.Error
		}
		// cond carries the user_id scope; see the callers.
		spent := r.db.WithContext(ctx).Where(cond, args...).Where("rotated_at IS NOT NULL").Delete(&sessionRow{})
		if spent.Error != nil {
			return live, spent.Error
		}
		live += active.RowsAffected
		if active.RowsAffected+spent.RowsAffected == 0 {
			break
		}
	}
	return live, nil
}

// maxRevocationSweeps mirrors the bound in internal/auth, which this package
// cannot import (see sessionRow).
const maxRevocationSweeps = 5

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
