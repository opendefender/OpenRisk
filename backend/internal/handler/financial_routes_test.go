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

// TestFinancialRoutes_DashboardNeedsTheMonteCarloDepth ties the financial gates
// to the routes in the composition root (#613).
//
// pkg/entitlements TestFinancialDashboard_OpensAtPro proves the "on" depth of
// financial quantification is Pro or above on every plan. That only protects
// the dashboard if the dashboard's routes are mounted behind the "on" gate and
// not the basic one, which a single wrong identifier in main.go would undo.
func TestFinancialRoutes_DashboardNeedsTheMonteCarloDepth(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	require.Contains(t, src, `featFinancial := middleware.RequireLevel(entitlementService, ent.FeatFinancialQuant, ent.LevelOn)`)
	require.Contains(t, src, `featFinancialBasic := middleware.RequireLevel(entitlementService, ent.FeatFinancialQuant, ent.LevelBasic)`)

	// Each handler, and the gate it must sit behind. Only the per-risk read may
	// take the basic gate; everything the dashboard draws needs "on" (Pro+).
	want := map[string]string{
		"financialAnalyticsHandler.GetFinancialSummary)": "featFinancial,",
		"financialAnalyticsHandler.SetAppetite)":         "featFinancial,",
		"riskHandler.SimulateRiskFinancial)":             "featFinancial,",
		"riskHandler.GetRiskFinancial)":                  "featFinancialBasic,",
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(src, "\n") {
		for handler, gate := range want {
			if strings.Contains(line, handler) {
				seen[handler] = true
				require.Contains(t, line, " "+gate, "%s must be mounted behind %s: %s", handler, gate, strings.TrimSpace(line))
			}
		}
	}
	for handler := range want {
		require.True(t, seen[handler], "%s is not mounted in main.go", handler)
	}
}
