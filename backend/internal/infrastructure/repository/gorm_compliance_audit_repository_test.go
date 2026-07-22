// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditRepo(t *testing.T) *GormComplianceAuditRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_audits (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			title TEXT NOT NULL,
			framework_id TEXT,
			type TEXT NOT NULL DEFAULT 'internal',
			status TEXT NOT NULL DEFAULT 'planned',
			auditor TEXT,
			scope TEXT,
			summary TEXT,
			compliance_score REAL,
			scheduled_start DATETIME,
			scheduled_end DATETIME,
			completed_at DATETIME,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)

	require.NoError(t, db.Exec(`
		CREATE TABLE remediation_plans (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			control_id TEXT,
			framework_id TEXT,
			audit_id TEXT,
			priority TEXT NOT NULL DEFAULT 'medium',
			status TEXT NOT NULL DEFAULT 'open',
			assigned_to TEXT,
			due_date DATETIME,
			completed_at DATETIME,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
	`).Error)

	return NewGormComplianceAuditRepository(db)
}

func TestAuditRepo_CreateGet_TenantIsolation(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	a := &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenantA, Title: "ISO 27001 internal audit", Type: domain.AuditTypeInternal, Status: domain.AuditStatusPlanned}
	require.NoError(t, repo.CreateAudit(ctx, a))

	// Same tenant → found.
	got, err := repo.GetAuditByID(ctx, a.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "ISO 27001 internal audit", got.Title)

	// Other tenant → nil (never 403, never leak).
	other, err := repo.GetAuditByID(ctx, a.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, other)
}

func TestAuditRepo_CreateRequiresTenant(t *testing.T) {
	repo := setupAuditRepo(t)
	err := repo.CreateAudit(context.Background(), &domain.ComplianceAudit{ID: uuid.New(), Title: "x"})
	assert.Error(t, err)
}

func TestAuditRepo_List_ScopedToTenant(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	require.NoError(t, repo.CreateAudit(ctx, &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenantA, Title: "A1"}))
	require.NoError(t, repo.CreateAudit(ctx, &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenantA, Title: "A2"}))
	require.NoError(t, repo.CreateAudit(ctx, &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenantB, Title: "B1"}))

	list, err := repo.ListAudits(ctx, tenantA)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestAuditRepo_Update_CrossTenantRefused(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	a := &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenantA, Title: "orig", Status: domain.AuditStatusPlanned}
	require.NoError(t, repo.CreateAudit(ctx, a))

	// Forge the same ID but another tenant → no row matched → ErrNotFound.
	forged := &domain.ComplianceAudit{ID: a.ID, TenantID: tenantB, Title: "hijacked", Status: domain.AuditStatusCompleted}
	err := repo.UpdateAudit(ctx, forged)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Original untouched.
	got, _ := repo.GetAuditByID(ctx, a.ID, tenantA)
	require.NotNil(t, got)
	assert.Equal(t, "orig", got.Title)
}

func TestAuditRepo_Delete(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	a := &domain.ComplianceAudit{ID: uuid.New(), TenantID: tenant, Title: "gone"}
	require.NoError(t, repo.CreateAudit(ctx, a))
	require.NoError(t, repo.DeleteAudit(ctx, a.ID, tenant))
	// Deleting again → not found.
	assert.ErrorIs(t, repo.DeleteAudit(ctx, a.ID, tenant), domain.ErrNotFound)
}

func TestRemediationRepo_CreateListFilter_TenantIsolation(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	ctrl := uuid.New()

	p1 := &domain.RemediationPlan{ID: uuid.New(), TenantID: tenantA, Title: "Patch web-01", ControlID: &ctrl, Priority: domain.RemediationPriorityHigh, Status: domain.RemediationStatusOpen}
	require.NoError(t, repo.CreateRemediation(ctx, p1))
	require.NoError(t, repo.CreateRemediation(ctx, &domain.RemediationPlan{ID: uuid.New(), TenantID: tenantA, Title: "Other", Priority: domain.RemediationPriorityLow, Status: domain.RemediationStatusInProgress}))
	require.NoError(t, repo.CreateRemediation(ctx, &domain.RemediationPlan{ID: uuid.New(), TenantID: tenantB, Title: "B plan", Priority: domain.RemediationPriorityLow, Status: domain.RemediationStatusOpen}))

	// Tenant scoping.
	all, err := repo.ListRemediations(ctx, tenantA, domain.RemediationFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Filter by control id.
	byCtrl, err := repo.ListRemediations(ctx, tenantA, domain.RemediationFilter{ControlID: &ctrl})
	require.NoError(t, err)
	require.Len(t, byCtrl, 1)
	assert.Equal(t, "Patch web-01", byCtrl[0].Title)

	// Filter by status.
	open, err := repo.ListRemediations(ctx, tenantA, domain.RemediationFilter{Status: domain.RemediationStatusOpen})
	require.NoError(t, err)
	assert.Len(t, open, 1)

	// Cross-tenant get → nil.
	got, err := repo.GetRemediationByID(ctx, p1.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRemediationRepo_Update_CrossTenantRefused(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	p := &domain.RemediationPlan{ID: uuid.New(), TenantID: tenantA, Title: "orig", Priority: domain.RemediationPriorityMedium, Status: domain.RemediationStatusOpen}
	require.NoError(t, repo.CreateRemediation(ctx, p))

	forged := &domain.RemediationPlan{ID: p.ID, TenantID: tenantB, Title: "hijacked", Priority: domain.RemediationPriorityCritical, Status: domain.RemediationStatusCompleted}
	assert.ErrorIs(t, repo.UpdateRemediation(ctx, forged), domain.ErrNotFound)
}
