// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// TrustedDeviceTTL is how long a device stays trusted after its last sign-in.
const TrustedDeviceTTL = 90 * 24 * time.Hour

const trustedDeviceVersion = "v1"

// DeviceCookieSigner signs and checks the trusted-device cookie (D-064, #879).
//
// The value is `v1.<user>.<device>.<issued, unix µs>.<signature>`, signed with
// HMAC-SHA256. It is signed with the first key and accepted under any: the
// second is the previous key, so rotating it does not untrust every device at
// once.
type DeviceCookieSigner struct {
	keys [][]byte
	now  func() time.Time
}

// NewDeviceCookieSigner takes the current key, then optionally the previous one.
func NewDeviceCookieSigner(current []byte, previous ...[]byte) *DeviceCookieSigner {
	keys := [][]byte{current}
	for _, k := range previous {
		if len(k) > 0 {
			keys = append(keys, k)
		}
	}
	return &DeviceCookieSigner{keys: keys, now: time.Now}
}

// Issue returns a cookie value for a new trusted device of this account.
func (s *DeviceCookieSigner) Issue(userID uuid.UUID) string {
	payload := strings.Join([]string{
		trustedDeviceVersion, userID.String(), uuid.NewString(),
		// Microseconds, like devices_revoked_at: a cookie re-issued right after
		// a revocation must read as later than it, even within the same second.
		strconv.FormatInt(s.now().UnixMicro(), 10),
	}, ".")
	return payload + "." + sign(s.keys[0], payload)
}

// Verify returns the device a cookie value names, or nil when the value is
// malformed, signed with no known key, or older than TrustedDeviceTTL.
func (s *DeviceCookieSigner) Verify(value string) *domain.TrustedDevice {
	i := strings.LastIndexByte(value, '.')
	if i < 0 {
		return nil
	}
	payload, sig := value[:i], value[i+1:]
	ok := false
	for _, k := range s.keys {
		if hmac.Equal([]byte(sig), []byte(sign(k, payload))) {
			ok = true
			break
		}
	}
	if !ok {
		return nil
	}
	parts := strings.Split(payload, ".")
	if len(parts) != 4 || parts[0] != trustedDeviceVersion {
		return nil
	}
	userID, err1 := uuid.Parse(parts[1])
	deviceID, err2 := uuid.Parse(parts[2])
	usec, err3 := strconv.ParseInt(parts[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	issued := time.UnixMicro(usec)
	if age := s.now().Sub(issued); age < 0 || age > TrustedDeviceTTL {
		return nil
	}
	return &domain.TrustedDevice{UserID: userID, DeviceID: deviceID, IssuedAt: issued}
}

func sign(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
