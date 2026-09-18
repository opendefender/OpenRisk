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

// TestVendorRoutes_EveryRouteCarriesItsPermissionAndTheEntitlement ties TPRM's
// guards to the routes in the composition root (#669, #670).
//
// pkg/entitlements TestVendorRisk_IsBusinessAndEnterpriseOnly proves the matrix
// refuses vendor_risk to Free and Pro, and the RequireFeature middleware has its
// own tests for answering 402. Neither says whether anybody attached the gate to
// a TPRM route: a route mounted without featVendor would serve TPRM to every
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
		`protected.Get("/vendor-questionnaire-templates", vendorRead, featVendor, vendorAssessmentHandler.ListTemplates)`,
		`protected.Post("/vendor-questionnaire-templates", vendorManage, featVendor, vendorAssessmentHandler.CreateTemplate)`,
		`protected.Get("/vendor-questionnaire-templates/:id", vendorRead, featVendor, vendorAssessmentHandler.GetTemplate)`,
		`protected.Put("/vendor-questionnaire-templates/:id", vendorManage, featVendor, vendorAssessmentHandler.UpdateTemplate)`,
		`protected.Post("/vendor-questionnaire-templates/:id/archive", vendorManage, featVendor, vendorAssessmentHandler.ArchiveTemplate)`,
		`protected.Get("/vendors/:id/assessments", vendorRead, featVendor, vendorAssessmentHandler.ListAssessments)`,
		`protected.Post("/vendors/:id/assessments", vendorManage, featVendor, vendorAssessmentHandler.SendAssessment)`,
		`protected.Get("/vendor-assessments/:id", vendorRead, featVendor, vendorAssessmentHandler.GetAssessment)`,
		`protected.Post("/vendor-assessments/:id/revoke", vendorManage, featVendor, vendorAssessmentHandler.RevokeAssessment)`,
		`protected.Post("/vendor-assessments/:id/resend", vendorManage, featVendor, vendorAssessmentHandler.ResendAssessment)`,
	}
	for _, line := range want {
		require.Contains(t, src, line)
	}

	// No other authenticated TPRM route may be mounted without the entitlement.
	var mounted int
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `protected.`) && strings.Contains(line, `"/vendor`) {
			mounted++
			require.Contains(t, line, "featVendor", "a TPRM route without the vendor_risk entitlement: %s", strings.TrimSpace(line))
		}
	}
	require.Equal(t, len(want), mounted, "a TPRM route was added or removed; update this test and ADR 0004")
}

// TestVendorReminderWorker_IsStarted pins #672's wiring: a reminder use case
// that is fully tested but never started reminds nobody.
func TestVendorReminderWorker_IsStarted(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	require.Contains(t, src, `tprmapp.NewSendVendorAssessmentRemindersUseCase(vendorAssessmentDeps, vendorAssessmentRepo,`)
	require.Contains(t, src, `domain.NotificationTypeVendorAssessmentReminder`)
	require.Contains(t, src, `go vendorReminderWorker.Start(context.Background())`)
}

// TestPublicVendorAssessmentRoutes_AreMountedBeforeTheGateWithNoAuth pins
// ADR 0004 D4's mounting: on `app`, rate-limited per IP, and with NO auth
// middleware — the vendor holds no account.
func TestPublicVendorAssessmentRoutes_AreMountedBeforeTheGateWithNoAuth(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	src := string(raw)

	for _, line := range []string{
		`app.Get("/api/v1/public/vendor-assessment", vendorPublicRateLimit, func(c *fiber.Ctx) error {`,
		`app.Put("/api/v1/public/vendor-assessment/answers", vendorPublicRateLimit, func(c *fiber.Ctx) error {`,
		`app.Post("/api/v1/public/vendor-assessment/submit", vendorPublicRateLimit, func(c *fiber.Ctx) error {`,
	} {
		require.Contains(t, src, line)
	}

	public := strings.Index(src, `app.Get("/api/v1/public/vendor-assessment"`)
	protectedGroup := strings.Index(src, `// --- Routes Protégées (Nécessitent JWT) ---`)
	require.Greater(t, protectedGroup, 0)
	require.Less(t, public, protectedGroup, "the public routes must be registered before the JWT gate")

	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `"/api/v1/public/vendor-assessment`) {
			for _, forbidden := range []string{"optionalAuth", "RequirePermission", "featVendor", "protected."} {
				require.NotContains(t, line, forbidden, "the public questionnaire takes no auth middleware: %s", strings.TrimSpace(line))
			}
		}
	}
}
