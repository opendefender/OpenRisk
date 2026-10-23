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

// TestHostedPlanRoutes_EveryRouteCarriesItsGate ties #783's gates to the routes
// in the composition root, the way the TPRM test does for featVendor.
//
// The middleware tests prove a gate answers 402; the matrix tests prove which
// plans it refuses. Neither proves anybody attached it to a route. This scans
// main.go: every authenticated route under a gated prefix must carry its gate,
// and the count is pinned so a new route cannot slip in ungated.
func TestHostedPlanRoutes_EveryRouteCarriesItsGate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	for _, decl := range []string{
		`featAutomation := middleware.RequireFeature(entitlementService, ent.FeatAutomation)`,
		`featScanner := middleware.RequireFeature(entitlementService, ent.FeatScanner)`,
		`featCTI := middleware.RequireFeature(entitlementService, ent.FeatCTI)`,
		`featGovernance := middleware.RequireFeature(entitlementService, ent.FeatGovernance)`,
		`capFrameworks := middleware.RequireCapacity(entitlementService, ent.LimitFrameworks)`,
	} {
		require.Contains(t, src, decl)
	}

	gates := []struct {
		prefix string
		gate   string
		// exempt lists sub-prefixes that stay open, each for a stated reason.
		exempt []string
		count  int
	}{
		// Channels carry incident alerts too, and incidents are on every plan.
		{`"/automation/`, "featAutomation", []string{`"/automation/channels`}, 19},
		// The mitigation hook is the risk module's, not the scanner console's.
		{`"/scanner/`, "featScanner", []string{`"/scanner/mitigations/`}, 12},
		{`"/cti/`, "featCTI", nil, 8},
		{`"/governance/`, "featGovernance", nil, 21},
	}
	for _, g := range gates {
		var mounted int
	lines:
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, "protected.") || !strings.Contains(line, g.prefix) {
				continue
			}
			for _, ex := range g.exempt {
				if strings.Contains(line, ex) {
					require.NotContains(t, line, g.gate, "an exempt route gained the gate: %s", strings.TrimSpace(line))
					continue lines
				}
			}
			mounted++
			require.Contains(t, line, g.gate, "a %s route without its plan gate: %s", g.prefix, strings.TrimSpace(line))
		}
		require.Equal(t, g.count, mounted, "%s routes were added or removed; update this test and ADR 0007", g.prefix)
	}

	// The framework cap sits on the one route that creates a framework.
	require.Contains(t, src,
		`protected.Post("/compliance/frameworks", complianceFrameworkCreate, capFrameworks, complianceHandler.CreateFramework)`)
	require.Equal(t, 1, strings.Count(src, "capFrameworks,"), "capFrameworks is mounted once")
}
