// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"fmt"
	"strings"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// trustedDeviceSigner builds the trusted-device cookie signer (D-064, #879).
//
// DEVICE_COOKIE_KEY is optional: without it the key is derived from
// AUDIT_HASH_KEY under its own label, so this feature adds no secret an
// install must provision, and production already refuses to boot without
// AUDIT_HASH_KEY. A dedicated key can be rotated on its own:
// DEVICE_COOKIE_PREVIOUS_KEY keeps the cookies signed with the old one valid
// until they age out, instead of untrusting every device at once.
//
// In production a key that IS set must be as strong as AUDIT_HASH_KEY (#879
// review): one cookie for the attacker's own account is enough to try to crack
// a weak key offline, and the key forges a cookie for any account.
func trustedDeviceSigner(getenv func(string) string, production bool) (*coreauth.DeviceCookieSigner, error) {
	current := []byte(strings.TrimSpace(getenv("DEVICE_COOKIE_KEY")))
	previous := []byte(strings.TrimSpace(getenv("DEVICE_COOKIE_PREVIOUS_KEY")))
	if production {
		for name, k := range map[string][]byte{"DEVICE_COOKIE_KEY": current, "DEVICE_COOKIE_PREVIOUS_KEY": previous} {
			if len(k) > 0 && len(k) < minAuditHashKeyLength {
				return nil, fmt.Errorf("%s is too short: %d characters, production needs at least %d (or leave DEVICE_COOKIE_KEY unset to derive it from AUDIT_HASH_KEY)", name, len(k), minAuditHashKeyLength)
			}
		}
	}
	if len(current) == 0 {
		current = domain.DeriveFromAddressHashKey("openrisk/trusted-device-cookie/v1")
	}
	return coreauth.NewDeviceCookieSigner(current, previous), nil
}
