// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBusinessRoles_VendorKeysMirrorAssetKeys pins ADR 0004 D7: the vendor keys
// are granted exactly where the asset keys are. A vendor IS an asset, so a role
// that can read assets but not vendors (or update assets but not manage vendors)
// would see a vendor in the inventory and be refused the same row in the
// register — two answers to one permission question.
func TestBusinessRoles_VendorKeysMirrorAssetKeys(t *testing.T) {
	for _, r := range businessRoles {
		has := make(map[PermissionKey]bool, len(r.Permissions))
		for _, p := range r.Permissions {
			has[p] = true
		}
		assert.Equal(t, has["assets:read"], has["vendors:read"],
			"role %s: vendors:read must follow assets:read", r.Key)
		assert.Equal(t, has["assets:update"], has["vendors:manage"],
			"role %s: vendors:manage must follow assets:update", r.Key)
	}

	assert.True(t, IsCatalogPermission("vendors:read"))
	assert.True(t, IsCatalogPermission("vendors:manage"))
}
