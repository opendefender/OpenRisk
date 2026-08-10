// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// PasswordResetTTL is how long a reset link stays usable.
	//
	// Thirty minutes: long enough to survive a slow mail relay and someone
	// switching devices, short enough that a link sitting in an unattended inbox
	// or a forwarded mail stops being a credential.
	PasswordResetTTL = 30 * time.Minute

	// PasswordResetMaxPerHour caps requests per email address per hour.
	PasswordResetMaxPerHour = 3

	// PasswordResetWindow is the window that cap is measured over.
	PasswordResetWindow = time.Hour
)

// PasswordResetToken is one password-reset request.
//
// A row is written for EVERY request, including ones naming an address with no
// account. That is deliberate and it is what makes the rate limiter safe: if we
// only recorded attempts for real accounts, the limiter itself would answer the
// question the uniform response is designed to hide — three tries against a real
// address would start returning 429 while an unknown address never would. The
// counter has to see the same traffic the attacker generates.
//
// Hence UserID and TokenHash are both nullable: a request for an unknown address
// records the attempt and nothing else.
type PasswordResetToken struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	// EmailHash is SHA-256 of the normalised address. Always set.
	//
	// Hashed rather than plain so this table is not a harvestable list of which
	// addresses have accounts — and, for unknown addresses, not a log of what
	// someone probed for. It only ever needs equality, never readback.
	EmailHash string `gorm:"type:varchar(64);index;not null" json:"-"`

	// UserID is nil when the address matched no account.
	UserID *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`

	// TokenHash is SHA-256 of the secret handed out in the link, nil when no
	// account matched. Storing the hash means a database leak does not yield
	// usable reset links.
	TokenHash *string `gorm:"type:varchar(64);uniqueIndex" json:"-"`

	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`

	// Request provenance, for the audit trail and for the "someone requested a
	// reset" notice.
	RequestIP        string `gorm:"type:varchar(64)"  json:"request_ip,omitempty"`
	RequestUserAgent string `gorm:"type:varchar(512)" json:"request_user_agent,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName specifies the table name.
func (PasswordResetToken) TableName() string { return "password_reset_tokens" }

// AuditEntityType opts this entity into the automatic governance audit trail.
func (PasswordResetToken) AuditEntityType() string { return "password_reset_token" }

// IsExpired reports whether the reset window has closed.
func (t *PasswordResetToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }

// IsUsed reports whether the token has already been spent.
func (t *PasswordResetToken) IsUsed() bool { return t.UsedAt != nil }

// IsUsable reports whether the token can still set a password: it must belong to
// an account, be unspent, and be inside its window.
func (t *PasswordResetToken) IsUsable() bool {
	return t.UserID != nil && t.TokenHash != nil && !t.IsUsed() && !t.IsExpired()
}

// NormaliseEmail lowercases and trims an address so that "Alex@Corp.io " and
// "alex@corp.io" are one identity for lookup and for rate limiting. Without this
// the per-email cap would be trivially bypassed by varying the case.
func NormaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// HashEmailForReset returns the rate-limit key for an address.
func HashEmailForReset(email string) string {
	sum := sha256.Sum256([]byte(NormaliseEmail(email)))
	return hex.EncodeToString(sum[:])
}

// HashResetToken returns the stored form of a reset secret.
func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
