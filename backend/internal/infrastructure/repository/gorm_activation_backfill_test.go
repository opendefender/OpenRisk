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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// Minimal tables reconciled against the models, for the reason
// internal/testsupport/sqliteschema documents: Risk, Asset, Mitigation and
// ComplianceFramework carry Postgres-only DDL (gen_random_uuid() defaults, jsonb
// and text[] columns) that GORM cannot emit for sqlite, so AutoMigrate on them
// fails outright here. ActivationEvent was written to avoid exactly that and
// migrates directly.
func setupBackfillRepo(t *testing.T) (*GormActivationRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.ActivationEvent{}))

	for _, ddl := range []string{
		`CREATE TABLE risks (id TEXT PRIMARY KEY)`,
		`CREATE TABLE compliance_frameworks (id TEXT PRIMARY KEY)`,
		`CREATE TABLE assets (id TEXT PRIMARY KEY)`,
		`CREATE TABLE mitigations (id TEXT PRIMARY KEY)`,
		`CREATE TABLE reports (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
		// domain.Risk has an AfterSave hook that appends here; carry the table
		// so seeding exercises the same write path production does.
		`CREATE TABLE risk_histories (id TEXT PRIMARY KEY)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	for _, m := range []struct {
		table string
		model any
	}{
		{"risks", &domain.Risk{}},
		{"compliance_frameworks", &domain.ComplianceFramework{}},
		{"assets", &domain.Asset{}},
		{"mitigations", &domain.Mitigation{}},
		{"reports", &domain.Report{}},
		{"organization_members", &domain.OrganizationMember{}},
		{"risk_histories", &domain.RiskHistory{}},
	} {
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}
	return NewGormActivationRepository(db), db
}

// seedConfiguredTenant gives a tenant one of every record the checklist derives
// from — the "bank that signed up last quarter" of the issue.
func seedConfiguredTenant(t *testing.T, db *gorm.DB, tenant uuid.UUID, base time.Time) {
	t.Helper()

	require.NoError(t, db.Create(&domain.Risk{
		ID: uuid.New(), TenantID: tenant, OrganizationID: tenant,
		Name: "Ransomware", Title: "Ransomware",
		CreatedAt: base.Add(1 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&domain.ComplianceFramework{
		ID: uuid.New(), TenantID: tenant, Name: "ISO 27001",
		CreatedAt: base.Add(2 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&domain.Asset{
		ID: uuid.New(), TenantID: tenant, OrganizationID: tenant, Name: "core-banking",
		CreatedAt: base.Add(3 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&domain.Mitigation{
		ID: uuid.New(), TenantID: tenant, RiskID: uuid.New(), Title: "Offline backups",
		CreatedAt: base.Add(4 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&domain.Report{
		ID: uuid.New(), TenantID: tenant, Title: "Q2 board report",
		RunState:  domain.ReportRunSucceeded,
		CreatedAt: base.Add(5 * time.Hour),
	}).Error)

	// Two members: the founder, then the teammate they invited. Only the second
	// proves "invited a teammate".
	require.NoError(t, db.Create(&domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: uuid.New(),
		Role: domain.RoleAdmin, CreatedAt: base,
	}).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: uuid.New(),
		Role: domain.RoleUser, CreatedAt: base.Add(6 * time.Hour),
	}).Error)
}

// Criterion 1: a tenant configured before activation existed sees the truth on
// first render — six steps complete, each dated from its own record, not from
// the boot that noticed them.
func TestBackfillDerived_ConfiguredTenantSeesCompletedSteps(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	tenant := uuid.New()
	base := time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)
	seedConfiguredTenant(t, db, tenant, base)

	require.NoError(t, repo.BackfillDerivedEvents(context.Background()))

	got, err := repo.FirstOccurrences(context.Background(), tenant)
	require.NoError(t, err)

	for key, want := range map[domain.ActivationEventKey]time.Time{
		domain.ActivationRiskCreated:       base.Add(1 * time.Hour),
		domain.ActivationFrameworkImported: base.Add(2 * time.Hour),
		domain.ActivationAssetConnected:    base.Add(3 * time.Hour),
		domain.ActivationMitigationCreated: base.Add(4 * time.Hour),
		domain.ActivationReportGenerated:   base.Add(5 * time.Hour),
		domain.ActivationMemberInvited:     base.Add(6 * time.Hour),
	} {
		at, ok := got[key]
		require.Truef(t, ok, "step %s must be seeded for a tenant that holds the record", key)
		assert.WithinDurationf(t, want, at, time.Second,
			"%s must be dated from the underlying record, not from the backfill run", key)
	}

	// profile is not derivable from any stored record and must stay untouched.
	_, hasProfile := got[domain.ActivationProfileCompleted]
	assert.False(t, hasProfile, "profile has no proving record and must not be invented")
}

// Criterion 2: reboots must not duplicate rows, and must not move a date.
func TestBackfillDerived_IsIdempotentAcrossReboots(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	tenant := uuid.New()
	base := time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)
	seedConfiguredTenant(t, db, tenant, base)
	ctx := context.Background()

	require.NoError(t, repo.BackfillDerivedEvents(ctx))
	first, err := repo.FirstOccurrences(ctx, tenant)
	require.NoError(t, err)

	var afterFirst int64
	require.NoError(t, db.Model(&domain.ActivationEvent{}).Count(&afterFirst).Error)

	require.NoError(t, repo.BackfillDerivedEvents(ctx))
	require.NoError(t, repo.BackfillDerivedEvents(ctx))

	var afterThird int64
	require.NoError(t, db.Model(&domain.ActivationEvent{}).Count(&afterThird).Error)
	assert.Equal(t, afterFirst, afterThird, "re-running the backfill must not append rows")

	second, err := repo.FirstOccurrences(ctx, tenant)
	require.NoError(t, err)
	require.Len(t, second, len(first))
	for key, at := range first {
		assert.WithinDuration(t, at, second[key], time.Second, "completed_at moved for %s", key)
	}
}

// Criterion 2, second half: a step already ticked by live use keeps ITS date.
// The backfill must never overwrite a real event with a derived one.
func TestBackfillDerived_DoesNotDisturbLiveEvents(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	tenant := uuid.New()
	base := time.Date(2026, 3, 2, 8, 0, 0, 0, time.UTC)
	seedConfiguredTenant(t, db, tenant, base)
	ctx := context.Background()

	// The tenant created a risk live, AFTER the record the backfill would find.
	live := base.Add(48 * time.Hour)
	recordActivation(t, repo, tenant, domain.ActivationRiskCreated, live)

	require.NoError(t, repo.BackfillDerivedEvents(ctx))

	var riskEvents int64
	require.NoError(t, db.Model(&domain.ActivationEvent{}).
		Where("tenant_id = ? AND event_key = ?", tenant, string(domain.ActivationRiskCreated)).
		Count(&riskEvents).Error)
	assert.EqualValues(t, 1, riskEvents, "a tenant with a live event must not gain a derived duplicate")

	got, err := repo.FirstOccurrences(ctx, tenant)
	require.NoError(t, err)
	assert.WithinDuration(t, live, got[domain.ActivationRiskCreated], time.Second)
}

// Criterion 3: no record, no tick. This is the property that keeps the panel
// worth reading — a step is complete only when something real backs it.
func TestBackfillDerived_NeverMarksAStepWithoutEvidence(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	tenant := uuid.New()
	ctx := context.Background()

	// An asset and nothing else. A lone founding member, no invitee. A report
	// that failed. A soft-deleted risk.
	base := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&domain.Asset{
		ID: uuid.New(), TenantID: tenant, OrganizationID: tenant, Name: "vpn",
		CreatedAt: base,
	}).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: uuid.New(),
		Role: domain.RoleAdmin, CreatedAt: base,
	}).Error)
	require.NoError(t, db.Create(&domain.Report{
		ID: uuid.New(), TenantID: tenant, Title: "never finished",
		RunState: domain.ReportRunFailed, CreatedAt: base,
	}).Error)
	deleted := &domain.Risk{
		ID: uuid.New(), TenantID: tenant, OrganizationID: tenant,
		Name: "gone", Title: "gone", CreatedAt: base,
	}
	require.NoError(t, db.Create(deleted).Error)
	require.NoError(t, db.Delete(deleted).Error)

	require.NoError(t, repo.BackfillDerivedEvents(ctx))

	got, err := repo.FirstOccurrences(ctx, tenant)
	require.NoError(t, err)

	assert.Contains(t, got, domain.ActivationAssetConnected, "the asset is real and must tick")
	for _, absent := range []domain.ActivationEventKey{
		domain.ActivationRiskCreated,       // only a soft-deleted risk
		domain.ActivationFrameworkImported, // nothing at all
		domain.ActivationMitigationCreated, // nothing at all
		domain.ActivationReportGenerated,   // the run failed
		domain.ActivationMemberInvited,     // the founder is not an invitee
	} {
		assert.NotContainsf(t, got, absent, "%s has no proving record and must stay incomplete", absent)
	}
}

// Criterion 4 — RULE #2. The backfill reads every tenant's tables at once, which
// is exactly the shape that leaks. Tenant B's records must never tick tenant A.
func TestBackfillDerived_TenantIsolation(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	quiet, busy := uuid.New(), uuid.New()
	base := time.Date(2026, 5, 5, 7, 0, 0, 0, time.UTC)
	ctx := context.Background()

	// `busy` has everything. `quiet` has one asset and nothing else.
	seedConfiguredTenant(t, db, busy, base)
	require.NoError(t, db.Create(&domain.Asset{
		ID: uuid.New(), TenantID: quiet, OrganizationID: quiet, Name: "laptop",
		CreatedAt: base.Add(9 * time.Hour),
	}).Error)

	require.NoError(t, repo.BackfillDerivedEvents(ctx))

	quietState, err := repo.FirstOccurrences(ctx, quiet)
	require.NoError(t, err)
	require.Len(t, quietState, 1, "the quiet tenant holds one record and must get exactly one step")
	assert.Contains(t, quietState, domain.ActivationAssetConnected)
	assert.WithinDuration(t, base.Add(9*time.Hour), quietState[domain.ActivationAssetConnected], time.Second)

	// And no event row on the quiet tenant may carry a busy tenant's date.
	var rows []domain.ActivationEvent
	require.NoError(t, db.Where("tenant_id = ?", quiet).Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, quiet, rows[0].TenantID)

	busyState, err := repo.FirstOccurrences(ctx, busy)
	require.NoError(t, err)
	assert.Len(t, busyState, 6, "the busy tenant keeps its own six steps")
}

// A tenant that never existed has no state — and asking for one must not return
// somebody else's. The read path's fail-closed behaviour, pinned here because
// the backfill is what puts rows in the table for it to leak.
func TestBackfillDerived_UnknownTenantHasNoState(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	base := time.Date(2026, 5, 5, 7, 0, 0, 0, time.UTC)
	seedConfiguredTenant(t, db, uuid.New(), base)
	ctx := context.Background()

	require.NoError(t, repo.BackfillDerivedEvents(ctx))

	got, err := repo.FirstOccurrences(ctx, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, got)

	nilTenant, err := repo.FirstOccurrences(ctx, uuid.Nil)
	require.NoError(t, err)
	assert.Empty(t, nilTenant, "no tenant context must mean no state, never everyone's state")
}

// An empty install must be a no-op rather than an error at boot.
func TestBackfillDerived_EmptyDatabaseIsANoOp(t *testing.T) {
	repo, db := setupBackfillRepo(t)
	require.NoError(t, repo.BackfillDerivedEvents(context.Background()))

	var count int64
	require.NoError(t, db.Model(&domain.ActivationEvent{}).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}
