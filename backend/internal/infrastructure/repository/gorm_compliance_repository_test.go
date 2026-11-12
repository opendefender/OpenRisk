// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupComplianceRepo creates an in-memory SQLite DB with compliance tables
// and returns a ready-to-use GormComplianceRepository.
// setupComplianceRepoWithEvidence returns both halves of the register: the
// compliance repository under test and the evidence library it now counts from.
func setupComplianceRepoWithEvidence(t *testing.T) (*GormComplianceRepository, *GormEvidenceRepository) {
	t.Helper()
	repo := setupComplianceRepo(t)
	return repo, NewGormEvidenceRepository(repo.db)
}

func setupComplianceRepo(t *testing.T) *GormComplianceRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		// Required for errors.Is(err, gorm.ErrDuplicatedKey) to work — see
		// gorm_compliance_repository.go's CreateFramework/CreateControl.
		TranslateError: true,
	})
	require.NoError(t, err)

	// compliance_frameworks (global)
	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_frameworks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			catalog_key TEXT NOT NULL DEFAULT '',
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX idx_compliance_frameworks_tenant_name_version
			ON compliance_frameworks(tenant_id, name, version) WHERE deleted_at IS NULL;
	`).Error)

	// compliance_controls (tenant-scoped)
	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_controls (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			framework_id TEXT NOT NULL,
			reference_code TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT,
			source_reference TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'not_implemented',
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX idx_compliance_controls_tenant_fw_ref
			ON compliance_controls(tenant_id, framework_id, reference_code)
			WHERE deleted_at IS NULL AND reference_code != '';
	`).Error)

	// The evidence library, from the models rather than hand-written DDL.
	// GetControlByID counts current proof from these tables, so every compliance
	// fixture needs them — control_evidences is gone, nothing reads it any more.
	require.NoError(t, db.AutoMigrate(&domain.Evidence{}, &domain.EvidenceControlLink{}))

	return NewGormComplianceRepository(db)
}

// =============================================================================
// Framework Tests (tenant-scoped)
// =============================================================================

func TestCreateAndGetFramework(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()

	fw := &domain.ComplianceFramework{
		ID:          uuid.New(),
		TenantID:    tenantA,
		Name:        "ISO 27001",
		Version:     "2022",
		Description: "Information security management",
	}
	require.NoError(t, repo.CreateFramework(ctx, fw))

	got, err := repo.GetFrameworkByID(ctx, fw.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "ISO 27001", got.Name)
	assert.Equal(t, "2022", got.Version)

	// Another tenant must not see it.
	other, err := repo.GetFrameworkByID(ctx, fw.ID, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, other, "a framework must not be visible to another tenant")
}

func TestCreateFramework_RequiresTenantID(t *testing.T) {
	repo := setupComplianceRepo(t)
	err := repo.CreateFramework(context.Background(), &domain.ComplianceFramework{ID: uuid.New(), Name: "X"})
	require.Error(t, err)
}

func TestGetFrameworkByID_NotFound(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	got, err := repo.GetFrameworkByID(ctx, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got, "Non-existent framework should return nil, nil")
}

func TestListFrameworks(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	fws := []*domain.ComplianceFramework{
		{ID: uuid.New(), TenantID: tenantA, Name: "SOC 2", Version: "2023"},
		{ID: uuid.New(), TenantID: tenantA, Name: "ISO 27001", Version: "2022"},
		{ID: uuid.New(), TenantID: tenantA, Name: "NIST CSF", Version: "2.0"},
		{ID: uuid.New(), TenantID: tenantB, Name: "COBAC", Version: "2016"}, // other tenant's
	}
	for _, fw := range fws {
		require.NoError(t, repo.CreateFramework(ctx, fw))
	}

	got, err := repo.ListFrameworks(ctx, tenantA)
	require.NoError(t, err)
	require.Len(t, got, 3, "must only return tenantA's frameworks, not tenantB's")
	// Ordered by name ASC
	assert.Equal(t, "ISO 27001", got[0].Name)
	assert.Equal(t, "NIST CSF", got[1].Name)
	assert.Equal(t, "SOC 2", got[2].Name)
}

// =============================================================================
// Control Tests — Cross-tenant isolation
// =============================================================================

func TestCreateControl_RequiresTenantID(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	control := &domain.ComplianceControl{
		ID:   uuid.New(),
		Name: "Access Control Policy",
		// TenantID intentionally omitted (zero value)
	}
	err := repo.CreateControl(ctx, control)
	require.Error(t, err, "Creating a control without tenant_id must fail")
	assert.Contains(t, err.Error(), "tenant_id is required")
}

func TestGetControlByID_CrossTenantReturnsNil(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	fwID := uuid.New()

	// Create a framework first
	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: fwID, TenantID: uuid.New(), Name: "ISO 27001", Version: "2022",
	}))

	// Create a control belonging to tenantA
	controlID := uuid.New()
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID:          controlID,
		TenantID:    tenantA,
		FrameworkID: fwID,
		Name:        "A.5.1.1 Policies for information security",
		Status:      domain.ControlStatusNotImplemented,
	}))

	// TenantA can access it
	got, err := repo.GetControlByID(ctx, controlID, tenantA, time.Now())
	require.NoError(t, err)
	require.NotNil(t, got, "TenantA must see its own control")
	assert.Equal(t, "A.5.1.1 Policies for information security", got.Name)

	// TenantB CANNOT access it (cross-tenant → nil, nil = 404)
	got, err = repo.GetControlByID(ctx, controlID, tenantB, time.Now())
	require.NoError(t, err)
	assert.Nil(t, got, "Cross-tenant access must return nil (404), not an error or data leak")
}

func TestListControlsByFramework_TenantIsolation(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	fwID := uuid.New()

	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: fwID, TenantID: uuid.New(), Name: "SOC 2", Version: "2023",
	}))

	// Create controls for tenantA
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantA, FrameworkID: fwID,
		Name: "CC1.1", Status: domain.ControlStatusImplemented,
	}))
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantA, FrameworkID: fwID,
		Name: "CC1.2", Status: domain.ControlStatusInProgress,
	}))

	// Create a control for tenantB
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantB, FrameworkID: fwID,
		Name: "CC1.1-B", Status: domain.ControlStatusNotImplemented,
	}))

	// TenantA sees only its 2 controls
	controlsA, err := repo.ListControlsByFramework(ctx, tenantA, fwID)
	require.NoError(t, err)
	assert.Len(t, controlsA, 2)

	// TenantB sees only its 1 control
	controlsB, err := repo.ListControlsByFramework(ctx, tenantB, fwID)
	require.NoError(t, err)
	assert.Len(t, controlsB, 1)
	assert.Equal(t, "CC1.1-B", controlsB[0].Name)
}

func TestDeleteControl_CrossTenantReturnsError(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	fwID := uuid.New()

	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: fwID, TenantID: uuid.New(), Name: "DORA", Version: "2025",
	}))

	controlID := uuid.New()
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: controlID, TenantID: tenantA, FrameworkID: fwID,
		Name: "ICT Risk Management", Status: domain.ControlStatusInProgress,
	}))

	// TenantB trying to delete tenantA's control → "not found"
	err := repo.DeleteControl(ctx, controlID, tenantB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify the control still exists for tenantA
	got, err := repo.GetControlByID(ctx, controlID, tenantA, time.Now())
	require.NoError(t, err)
	require.NotNil(t, got, "Control must still exist after cross-tenant delete attempt")
}

func TestUpdateControl_CrossTenantFails(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	fwID := uuid.New()

	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: fwID, TenantID: uuid.New(), Name: "COBAC", Version: "2024",
	}))

	controlID := uuid.New()
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: controlID, TenantID: tenantA, FrameworkID: fwID,
		Name: "Original Name", Status: domain.ControlStatusNotImplemented,
	}))

	// TenantB tries to update tenantA's control
	err := repo.UpdateControl(ctx, &domain.ComplianceControl{
		ID:       controlID,
		TenantID: tenantB, // Wrong tenant
		Name:     "Hijacked Name",
		Status:   domain.ControlStatusImplemented,
	})
	require.Error(t, err, "Cross-tenant update must fail")

	// Verify original data is untouched
	got, err := repo.GetControlByID(ctx, controlID, tenantA, time.Now())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Original Name", got.Name)
}

// =============================================================================
// Evidence counts
//
// Storing and reading evidence moved to the library (migration 0052), and so did
// its isolation tests — see gorm_evidence_repository_test.go. What is still this
// repository's job, and still tested here, is answering "how much CURRENT proof
// does each control of this framework have", because that number gates whether a
// control may be declared implemented.
// =============================================================================

// attach files one artifact in the library and links it to a control.
func attach(t *testing.T, r *GormEvidenceRepository, tenant, control uuid.UUID, validUntil *time.Time) {
	t.Helper()
	ctx := context.Background()
	ev := &domain.Evidence{
		TenantID: tenant, Title: "proof", CollectedAt: time.Now().Add(-48 * time.Hour),
		Review: domain.EvidenceReviewAccepted, ValidUntil: validUntil,
	}
	require.NoError(t, r.Create(ctx, ev))
	require.NoError(t, r.Link(ctx, &domain.EvidenceControlLink{
		TenantID: tenant, EvidenceID: ev.ID, ControlID: control,
	}))
}

func TestCountEvidencesByFramework_ScopedByTenantAndFramework(t *testing.T) {
	repo, evRepo := setupComplianceRepoWithEvidence(t)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	frameworkID := uuid.New()
	otherFramework := uuid.New()

	// Each tenant imports its OWN control rows (per-tenant instances), so control
	// IDs never overlap across tenants — that's why the count query scopes by
	// BOTH the evidence tenant and the control tenant (defense in depth).
	ctrl1 := uuid.New()  // tenantA, frameworkID — will have 2 evidences
	ctrl2 := uuid.New()  // tenantA, frameworkID — will have 1 evidence
	ctrl3 := uuid.New()  // tenantA, otherFramework — must NOT be counted
	ctrl1B := uuid.New() // tenantB, frameworkID — tenantB's own control

	for _, c := range []struct {
		id, tenant, fw uuid.UUID
	}{
		{ctrl1, tenantA, frameworkID},
		{ctrl2, tenantA, frameworkID},
		{ctrl3, tenantA, otherFramework},
		{ctrl1B, tenantB, frameworkID},
	} {
		require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
			ID: c.id, TenantID: c.tenant, FrameworkID: c.fw, ReferenceCode: c.id.String()[:8], Name: "n",
		}))
	}

	// tenantA evidence: 2 on ctrl1, 1 on ctrl2, 1 on ctrl3 (other framework)
	for _, cid := range []uuid.UUID{ctrl1, ctrl1, ctrl2, ctrl3} {
		attach(t, evRepo, tenantA, cid, nil)
	}
	// A stray artifact carrying tenantB's tenant_id but linked to tenantA's
	// control must never inflate tenantA's counts.
	attach(t, evRepo, tenantB, ctrl1, nil)
	// tenantB's legitimate evidence on its own control.
	attach(t, evRepo, tenantB, ctrl1B, nil)
	// And proof that EXPIRED does not count, however many copies of it exist:
	// this is the tightening the library brought, and the reason a control whose
	// certificate lapsed can no longer be declared implemented.
	expired := time.Now().Add(-24 * time.Hour)
	attach(t, evRepo, tenantA, ctrl2, &expired)

	counts, err := repo.CountEvidencesByFramework(ctx, tenantA, frameworkID, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 2, counts[ctrl1], "ctrl1 must count only tenantA's 2 evidences, not the stray tenantB one")
	assert.Equal(t, 1, counts[ctrl2])
	assert.Equal(t, 0, counts[ctrl3], "control from another framework must be absent")
	assert.Len(t, counts, 2, "only tenantA controls with evidence in this framework appear")

	// tenantB sees only its own control's single evidence.
	countsB, err := repo.CountEvidencesByFramework(ctx, tenantB, frameworkID, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, countsB[ctrl1B])
	assert.Len(t, countsB, 1)
}

func TestCreateFramework_DuplicateNameVersion_Conflict(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()

	// Uniqueness is per-tenant: same (name, version) inside the SAME tenant conflicts.
	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: uuid.New(), TenantID: tenantA, Name: "ISO 27001", Version: "2022",
	}))

	err := repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: uuid.New(), TenantID: tenantA, Name: "ISO 27001", Version: "2022",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)

	// The SAME (name, version) under a DIFFERENT tenant must NOT conflict.
	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: uuid.New(), TenantID: uuid.New(), Name: "ISO 27001", Version: "2022",
	}))
}

func TestCreateControl_DuplicateReferenceCode_Conflict(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()

	tenantA := uuid.New()
	fwID := uuid.New()
	require.NoError(t, repo.CreateFramework(ctx, &domain.ComplianceFramework{
		ID: fwID, TenantID: uuid.New(), Name: "ISO 27001", Version: "2022",
	}))
	require.NoError(t, repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantA, FrameworkID: fwID,
		ReferenceCode: "A.5.1.1", Name: "Policies for information security",
	}))

	err := repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantA, FrameworkID: fwID,
		ReferenceCode: "A.5.1.1", Name: "Duplicate reference code",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)

	// A different tenant reusing the same reference_code under the same
	// framework must succeed — the unique index is scoped by tenant_id too.
	tenantB := uuid.New()
	err = repo.CreateControl(ctx, &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantB, FrameworkID: fwID,
		ReferenceCode: "A.5.1.1", Name: "Same code, different tenant",
	})
	require.NoError(t, err)
}

// =============================================================================
// #419 — evidence validity is judged at the caller's instant, never time.Now()
//
// The compliance repository used to read the wall clock inside both evidence
// predicates, while the evidence service judges the same valid_until with its
// injected clock (domain.Evidence.Covers). In production both are the wall
// clock, so nothing showed; the moment anything set a clock, the two answered
// different questions about the same artifact. Same class of defect as #409.
//
// The expiry instant is fixed in the past on purpose: against the old code,
// every "before expiry" case reads zero, so these tests cannot pass by luck.
// They never depend on the wall clock, so they cannot expire either.
// =============================================================================

var evidenceExpiry419 = time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)

// evidenceBoundaryCases pins the predicate at the expiry instant itself:
// `valid_until > asOf`, so proof whose valid_until IS asOf has lapsed, exactly
// as domain.Evidence.Covers decides. A drift to >= fails "exactly at expiry".
var evidenceBoundaryCases = []struct {
	name string
	asOf time.Time
	want int
}{
	{"a day before expiry it covers", evidenceExpiry419.Add(-24 * time.Hour), 1},
	{"a second before expiry it covers", evidenceExpiry419.Add(-time.Second), 1},
	{"exactly at expiry it has lapsed", evidenceExpiry419, 0},
	{"a day after expiry it has lapsed", evidenceExpiry419.Add(24 * time.Hour), 0},
}

func TestGetControlByID_JudgesEvidenceExpiryAgainstTheGivenInstant(t *testing.T) {
	repo, evRepo := setupComplianceRepoWithEvidence(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	ctrl := &domain.ComplianceControl{ID: uuid.New(), TenantID: tenantA, FrameworkID: uuid.New(), ReferenceCode: "A.5.28", Name: "Collection of evidence"}
	require.NoError(t, repo.CreateControl(ctx, ctrl))
	expiry := evidenceExpiry419
	attach(t, evRepo, tenantA, ctrl.ID, &expiry)

	for _, tc := range evidenceBoundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.GetControlByID(ctx, ctrl.ID, tenantA, tc.asOf)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tc.want, got.EvidenceCount,
				"EvidenceCount at %s — the count is not using the instant it was given", tc.asOf.Format(time.RFC3339))
		})
	}

	// Still tenant-scoped at any instant: tenant B never reads tenant A's control.
	other, err := repo.GetControlByID(ctx, ctrl.ID, tenantB, evidenceExpiry419.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Nil(t, other, "tenant B must not read tenant A's control")
}

func TestCountEvidencesByFramework_JudgesEvidenceExpiryAgainstTheGivenInstant(t *testing.T) {
	repo, evRepo := setupComplianceRepoWithEvidence(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	framework := uuid.New()

	ctrl := &domain.ComplianceControl{ID: uuid.New(), TenantID: tenantA, FrameworkID: framework, ReferenceCode: "A.5.28", Name: "Collection of evidence"}
	require.NoError(t, repo.CreateControl(ctx, ctrl))
	expiry := evidenceExpiry419
	attach(t, evRepo, tenantA, ctrl.ID, &expiry)

	for _, tc := range evidenceBoundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			counts, err := repo.CountEvidencesByFramework(ctx, tenantA, framework, tc.asOf)
			require.NoError(t, err)
			assert.Equal(t, tc.want, counts[ctrl.ID],
				"count at %s — the count is not using the instant it was given", tc.asOf.Format(time.RFC3339))
		})
	}

	countsB, err := repo.CountEvidencesByFramework(ctx, tenantB, framework, evidenceExpiry419.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, countsB, "tenant B must not see tenant A's evidence counts")
}
