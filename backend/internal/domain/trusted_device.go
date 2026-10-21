// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
)

// TrustedDevice is a browser that completed a sign-in to one account (D-064,
// #879). It comes from a signed cookie the server checked; it says nothing
// until the account it names is the one being signed into.
type TrustedDevice struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	IssuedAt time.Time
}

// TrustsFor reports whether the device may stand in for the address lock on
// this account: issued to it, and after its sessions were last revoked.
func (d *TrustedDevice) TrustsFor(user *User) bool {
	if d == nil || user == nil || d.UserID != user.ID {
		return false
	}
	// Compared at the cookie's precision (µs): a cookie re-issued to the
	// device that asked for the revocation is stamped right after it, and must
	// not read as earlier because the stamp carried nanoseconds.
	return user.DevicesRevokedAt == nil ||
		!d.IssuedAt.Before(user.DevicesRevokedAt.Truncate(time.Microsecond))
}

// DeriveFromAddressHashKey derives a key for another purpose from the server's
// AUDIT_HASH_KEY: HMAC-SHA256(key, label). The label separates the purposes, so
// the derived key reveals nothing of the original. Used when DEVICE_COOKIE_KEY
// is not set, so the device cookie needs no secret of its own to deploy.
func DeriveFromAddressHashKey(label string) []byte {
	mac := hmac.New(sha256.New, addressHashKey())
	mac.Write([]byte(label))
	return mac.Sum(nil)
}
