// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync/atomic"
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

	// EmailHash is the keyed hash of the normalised address (HashEmailForReset).
	// Always set.
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

// HashEmailForReset returns the keyed hash of an address: HMAC-SHA256 of the
// normalised address under the server's AUDIT_HASH_KEY (D-065, #879).
//
// It keys the password-reset limiter, the sign-in and sign-up limits in Redis,
// and the address written into the auth audit trail. It used to be a plain
// SHA-256, which anyone holding the trail and a list of candidate addresses
// could reverse by hashing each one. Under a key they do not hold, the trail
// keeps the "one address hammered from many sources" signal and says nothing
// about which address it was.
func HashEmailForReset(email string) string {
	mac := hmac.New(sha256.New, addressHashKey())
	mac.Write([]byte(NormaliseEmail(email)))
	return hex.EncodeToString(mac.Sum(nil))
}

// devAddressHashKey is used until SetAddressHashKey is called: by tests, and
// by a development server started without AUDIT_HASH_KEY. Production refuses
// to boot without a real key (cmd/server), so this never keys a real trail.
var devAddressHashKey = []byte("openrisk-dev-audit-hash-key-not-a-secret")

var configuredAddressHashKey atomic.Pointer[[]byte]

// SetAddressHashKey installs the server key. main.go calls it once at boot,
// before serving; every replica must be given the same key, or each one keys
// the limits and the trail differently.
func SetAddressHashKey(key []byte) {
	k := append([]byte(nil), key...)
	configuredAddressHashKey.Store(&k)
}

// KeyFingerprint identifies a key without revealing it: the first 8 bytes of
// HMAC-SHA256(key, a fixed label), in hex. Replicas compare fingerprints to
// notice they were given different keys (#1063).
func KeyFingerprint(key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("openrisk-key-fingerprint"))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

// AddressHashFingerprint is the fingerprint of the key HashEmailForReset uses.
func AddressHashFingerprint() string { return KeyFingerprint(addressHashKey()) }

func addressHashKey() []byte {
	if k := configuredAddressHashKey.Load(); k != nil && len(*k) > 0 {
		return *k
	}
	return devAddressHashKey
}

// HashResetToken returns the stored form of a reset secret.
func HashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
