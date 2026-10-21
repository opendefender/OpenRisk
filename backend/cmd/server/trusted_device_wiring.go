// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
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
func trustedDeviceSigner(getenv func(string) string) *coreauth.DeviceCookieSigner {
	current := []byte(strings.TrimSpace(getenv("DEVICE_COOKIE_KEY")))
	if len(current) == 0 {
		current = domain.DeriveFromAddressHashKey("openrisk/trusted-device-cookie/v1")
	}
	previous := []byte(strings.TrimSpace(getenv("DEVICE_COOKIE_PREVIOUS_KEY")))
	return coreauth.NewDeviceCookieSigner(current, previous)
}
