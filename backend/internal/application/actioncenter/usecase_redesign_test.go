// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package actioncenter

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// seedRedesign adds one item of each #902 category.
func seedRedesign(s *stubRepo) {
	seedAll(s)
	s.vulns = []domain.Vulnerability{{
		ID: uuid.New(), TenantID: tenantA, Title: "SSL-VPN RCE", CVEID: "CVE-2026-21873",
		Severity: domain.VulnSeverityCritical, KEV: true, AssetName: "fw-edge-01",
		Status: domain.VulnStatusOpen, SLADueAt: ptime(testNow.AddDate(0, 0, 1)),
	}}
	s.vendors = []VendorFollowUp{{
		Assessment: domain.VendorAssessment{
			ID: uuid.New(), TenantID: tenantA, VendorAssetID: uuid.New(),
			Status: domain.VendorAssessmentSent, DueAt: testNow.AddDate(0, 0, -2), SentAt: testNow.AddDate(0, 0, -9),
		},
		VendorName: "PayLink Afrique",
	}}
	s.reviews = []domain.Mitigation{{
		ID: uuid.New(), TenantID: tenantA, Title: "Immutable backups",
		Status: domain.MitigationReview, DueDate: ptime(testNow.AddDate(0, 0, 2)),
	}}
}

// An organisation admin sees every category of the tenant (owner decision,
// 2026-10-08) even without a business role.
func TestActionCenter_AdminSeesEverything_Success(t *testing.T) {
	s := newStub()
	seedRedesign(s)
	c := caller()
	c.IsAdmin = true
	res, err := newUC(s).GetActionCenter(c, 50, 0)
	require.NoError(t, err)
	got := types(res.Data)
	for _, want := range []ItemType{
		ItemTypeOverdueMitigation, ItemTypeCriticalRisk, ItemTypeOpenIncident, ItemTypeExpiringEvidence,
		ItemTypeOverdueRemediation, ItemTypeVulnerabilitySLA, ItemTypeVendorFollowUp, ItemTypeMitigationReview,
	} {
		assert.Contains(t, got, want)
	}
}

// The new rows carry the facts the row prints and a route that exists.
func TestActionCenter_RedesignItems_MetaAndLinks(t *testing.T) {
	s := newStub()
	seedRedesign(s)
	c := caller()
	c.IsAdmin = true
	res, err := newUC(s).GetActionCenter(c, 50, 0)
	require.NoError(t, err)
	by := map[ItemType]ActionItem{}
	for _, it := range res.Data {
		by[it.Type] = it
	}
	v := by[ItemTypeVulnerabilitySLA]
	assert.Equal(t, "CVE-2026-21873", v.Meta["cve_id"])
	assert.Equal(t, "fw-edge-01", v.Meta["asset_name"])
	assert.Equal(t, "true", v.Meta["kev"])
	assert.Equal(t, "open", v.Meta["status"])
	assert.Contains(t, v.DeepLink, "/vulnerabilities?drawer=vulnerability&entity=")

	vd := by[ItemTypeVendorFollowUp]
	assert.Equal(t, "PayLink Afrique", vd.Title)
	assert.Contains(t, vd.DeepLink, "/vendors/")
	assert.Contains(t, vd.DeepLink, "/assessments/")

	rv := by[ItemTypeMitigationReview]
	assert.Equal(t, "REVIEW", rv.Meta["status"])
	assert.Contains(t, rv.DeepLink, "/risks/mitigations/")
}

// A role keeps its own perimeter: an auditor never triggers the vulnerability,
// vendor or review scans.
func TestActionCenter_RedesignCategories_Unauthorized(t *testing.T) {
	s := newStub()
	seedRedesign(s)
	s.role = domain.BusinessRoleAuditor
	res, err := newUC(s).GetActionCenter(caller(), 50, 0)
	require.NoError(t, err)
	assert.False(t, s.called["vulns"])
	assert.False(t, s.called["vendors"])
	assert.False(t, s.called["reviews"])
	assert.NotContains(t, types(res.Data), ItemTypeVulnerabilitySLA)
}

// A security analyst gets the vulnerability deadlines; nothing new for a
// member with no role and no admin right.
func TestActionCenter_RedesignCategories_ByRole(t *testing.T) {
	s := newStub()
	seedRedesign(s)
	s.role = domain.BusinessRoleSecurityAnalyst
	res, err := newUC(s).GetActionCenter(caller(), 50, 0)
	require.NoError(t, err)
	assert.Contains(t, types(res.Data), ItemTypeVulnerabilitySLA)

	s2 := newStub()
	seedRedesign(s2)
	res2, err := newUC(s2).GetActionCenter(caller(), 50, 0)
	require.NoError(t, err)
	assert.Empty(t, res2.Data, "no role, not admin: only own approvals, none seeded")
}
