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

// TestHostedPlanRoutes_CapsAndGatesAreMounted ties #783's plan grid (D-078) to
// the composition root, the way the TPRM test does for featVendor.
//
// The middleware tests prove a cap answers 402 at its limit; the matrix tests
// prove the numbers. Neither proves anybody attached the cap to a route. This
// scans main.go: each cap sits on the route that creates what it counts, the
// two use-case quotas are wired, threat intel carries its gate on every route,
// and automation, the scanner and governance carry no plan gate (D-078 opened
// them to every plan).
func TestHostedPlanRoutes_CapsAndGatesAreMounted(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	for _, line := range []string{
		`featCTI := middleware.RequireFeature(entitlementService, ent.FeatCTI)`,
		`capUsers := middleware.RequireCapacity(entitlementService, ent.LimitUsers)`,
		`capFrameworks := middleware.RequireCapacity(entitlementService, ent.LimitFrameworks)`,
		`capScanSources := middleware.RequireCapacity(entitlementService, ent.LimitScanSources)`,
		`capWorkflows := middleware.RequireCapacity(entitlementService, ent.LimitApprovalWorkflows)`,
		`protected.Post("/compliance/frameworks", complianceFrameworkCreate, capFrameworks, complianceHandler.CreateFramework)`,
		`protected.Post("/scanner/configs", scannerCreate, capScanSources, scannerHandler.CreateScanConfig)`,
		`protected.Post("/governance/workflows", governanceAdmin, capWorkflows, governanceHandler.CreateWorkflow)`,
		`protected.Post("/users", adminRole, capUsers, handlers.CreateUser)`,
		`middleware.RequirePermission("organization:members:invite"), capUsers, memberHandler.CreateInvitation)`,
		// Active rules and monthly scans are capped where every path passes.
		`appauto.NewRuleService(automationRuleRepo).WithQuota(entitlementService),`,
		`).WithQuota(entitlementService)`,
	} {
		require.Contains(t, src, line)
	}
	for _, c := range []string{"capFrameworks,", "capScanSources,", "capWorkflows,"} {
		require.Equal(t, 1, strings.Count(src, c), "%s is mounted once", c)
	}
	require.Equal(t, 2, strings.Count(src, "capUsers,"), "capUsers sits on the two ways to add a member")

	var cti int
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "protected.") {
			continue
		}
		for _, open := range []string{`"/automation/`, `"/scanner/`, `"/governance/`} {
			if strings.Contains(line, open) {
				require.NotContains(t, line, "RequireFeature", "D-078 opened %s to every plan: %s", open, strings.TrimSpace(line))
				require.NotRegexp(t, `feat[A-Z]\w*,`, line, "D-078 opened %s to every plan: %s", open, strings.TrimSpace(line))
			}
		}
		if strings.Contains(line, `"/cti/`) {
			cti++
			require.Contains(t, line, "featCTI", "a threat intel route without its plan gate: %s", strings.TrimSpace(line))
		}
	}
	require.Equal(t, 8, cti, "threat intel routes were added or removed; update this test and ADR 0007")
}
