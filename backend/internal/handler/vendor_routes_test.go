// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestVendorRoutes_EveryRouteCarriesItsPermissionAndTheEntitlement ties #669's
// guards to the routes in the composition root.
//
// pkg/entitlements TestVendorRisk_IsBusinessAndEnterpriseOnly proves the matrix
// refuses vendor_risk to Free and Pro, and the RequireFeature middleware has its
// own tests for answering 402. Neither says whether anybody attached the gate to
// a /vendors route: a route mounted without featVendor would serve TPRM to every
// plan with both of those tests green. This is the assertion that closes that.
func TestVendorRoutes_EveryRouteCarriesItsPermissionAndTheEntitlement(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	require.Contains(t, src, `featVendor := middleware.RequireFeature(entitlementService, ent.FeatVendorRisk)`)
	require.Contains(t, src, `vendorRead := middleware.RequirePermission("vendors:read")`)
	require.Contains(t, src, `vendorManage := middleware.RequirePermission("vendors:manage")`)

	want := []string{
		`protected.Get("/vendors", vendorRead, featVendor, vendorHandler.ListVendors)`,
		`protected.Get("/vendors/:id/chain", vendorRead, featVendor, vendorHandler.GetVendorChain)`,
		`protected.Post("/vendors/:id/assets", vendorManage, featVendor, vendorHandler.LinkVendorAsset)`,
		`protected.Delete("/vendors/:id/assets/:linkId", vendorManage, featVendor, vendorHandler.UnlinkVendorAsset)`,
	}
	for _, line := range want {
		require.Contains(t, src, line)
	}

	// No other /vendors route may be mounted without the entitlement.
	var mounted int
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `protected.`) && strings.Contains(line, `"/vendors`) {
			mounted++
			require.Contains(t, line, "featVendor", "a /vendors route without the vendor_risk entitlement: %s", strings.TrimSpace(line))
		}
	}
	require.Equal(t, len(want), mounted, "a /vendors route was added or removed; update this test and ADR 0004")
}
