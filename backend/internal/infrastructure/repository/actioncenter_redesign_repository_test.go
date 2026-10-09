// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// #902: the three new action-center sources, each read for one tenant only.

func TestActionCenterRepo_VulnerabilitiesDueBy(t *testing.T) {
	db := newActionCenterTestDB(t)
	repo := NewActionCenterRepository(db)
	now := acNow()
	add := func(tenant uuid.UUID, title string, sev domain.VulnSeverity, seen time.Time, status domain.VulnStatus) {
		v := domain.Vulnerability{ID: uuid.New(), TenantID: tenant, Title: title, Severity: sev,
			FirstSeen: seen, LastSeen: seen, Status: status, Source: "manual"}
		require.NoError(t, db.Create(&v).Error)
	}
	add(tA, "overdue critical", domain.VulnSeverityCritical, now.AddDate(0, 0, -10), domain.VulnStatusOpen)
	add(tA, "due in 4 days", domain.VulnSeverityHigh, now.AddDate(0, 0, -10), domain.VulnStatusTriaged)
	add(tA, "due in a month", domain.VulnSeverityLow, now.AddDate(0, 0, -20), domain.VulnStatusOpen)
	add(tA, "fixed", domain.VulnSeverityCritical, now.AddDate(0, 0, -10), domain.VulnStatusRemediated)
	add(tB, "other tenant", domain.VulnSeverityCritical, now.AddDate(0, 0, -10), domain.VulnStatusOpen)

	rows, err := repo.VulnerabilitiesDueBy(tA, now.AddDate(0, 0, 7), 50)
	require.NoError(t, err)
	titles := []string{}
	for _, r := range rows {
		titles = append(titles, r.Title)
	}
	assert.Equal(t, []string{"overdue critical", "due in 4 days"}, titles, "deadline order, open work only, this tenant only")

	_, err = repo.VulnerabilitiesDueBy(uuid.Nil, now, 50)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestActionCenterRepo_VendorAssessmentsDueBy(t *testing.T) {
	db := newActionCenterTestDB(t)
	repo := NewActionCenterRepository(db)
	now := acNow()
	vendorA := domain.Asset{ID: uuid.New(), TenantID: tA, Name: "PayLink Afrique"}
	vendorB := domain.Asset{ID: uuid.New(), TenantID: tB, Name: "Not yours"}
	require.NoError(t, db.Create(&vendorA).Error)
	require.NoError(t, db.Create(&vendorB).Error)
	add := func(tenant, vendor uuid.UUID, status domain.VendorAssessmentStatus, due time.Time) {
		a := domain.VendorAssessment{ID: uuid.New(), TenantID: tenant, VendorAssetID: vendor, TemplateID: uuid.New(),
			TemplateVersion: 1, Status: status, OwnerUserID: uuid.New(), ContactEmail: "x@y.z", ContactLanguage: "fr",
			DueAt: due, SentBy: uuid.New(), SentAt: now.AddDate(0, 0, -9)}
		require.NoError(t, db.Create(&a).Error)
	}
	add(tA, vendorA.ID, domain.VendorAssessmentSent, now.AddDate(0, 0, -2))
	add(tA, vendorA.ID, domain.VendorAssessmentSubmitted, now.AddDate(0, 0, -2))
	add(tA, vendorA.ID, domain.VendorAssessmentInProgress, now.AddDate(0, 0, 30))
	// A row of tenant A pointing at tenant B's vendor must not surface B's name.
	add(tA, vendorB.ID, domain.VendorAssessmentSent, now.AddDate(0, 0, -1))
	add(tB, vendorB.ID, domain.VendorAssessmentSent, now.AddDate(0, 0, -1))

	rows, err := repo.VendorAssessmentsDueBy(tA, now.AddDate(0, 0, 7), 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "PayLink Afrique", rows[0].VendorName)
}

func TestActionCenterRepo_MitigationsInReview(t *testing.T) {
	db := newActionCenterTestDB(t)
	repo := NewActionCenterRepository(db)
	add := func(tenant uuid.UUID, title string, st domain.MitigationStatus) {
		m := domain.Mitigation{ID: uuid.New(), TenantID: tenant, RiskID: uuid.New(), Title: title, Status: st, CreatedBy: uuid.New()}
		require.NoError(t, db.Create(&m).Error)
	}
	add(tA, "review me", domain.MitigationReview)
	add(tA, "doing", domain.MitigationInProgress)
	add(tB, "theirs", domain.MitigationReview)

	rows, err := repo.MitigationsInReview(tA, 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "review me", rows[0].Title)
}
