// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReportJobRepo(t *testing.T) *GormReportJobRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Mirrors the AutoMigrate shape. Written out rather than AutoMigrated so a
	// drift between this DDL and domain.ReportJob shows up as a test failure
	// here instead of as a surprise in Postgres.
	require.NoError(t, db.Exec(`
		CREATE TABLE report_jobs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'queued',
			params TEXT,
			title TEXT,
			filename TEXT,
			content_type TEXT,
			artifact BLOB,
			size_bytes INTEGER,
			error TEXT,
			requested_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			completed_at DATETIME
		)`).Error)

	return NewGormReportJobRepository(db)
}

func seedJob(t *testing.T, r *GormReportJobRepository, tenantID uuid.UUID) *domain.ReportJob {
	t.Helper()
	job := &domain.ReportJob{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Kind:        domain.ReportKindComplianceFramework,
		Status:      domain.ReportJobSucceeded,
		Title:       "ISO/IEC 27001 2022",
		Filename:    "r.pdf",
		ContentType: "application/pdf",
		Artifact:    []byte("%PDF-1.4"),
		SizeBytes:   8,
		RequestedBy: uuid.New(),
	}
	require.NoError(t, r.Create(context.Background(), job))
	return job
}

// RULE #2. A report is a rendered snapshot of a tenant's control posture, so a
// readable job id from another tenant would hand over the whole framework —
// controls, statuses, evidence counts — in one PDF.
func TestReportJob_GetByID_CrossTenantIsNotFound(t *testing.T) {
	repo := setupReportJobRepo(t)
	owner, intruder := uuid.New(), uuid.New()
	job := seedJob(t, repo, owner)

	got, err := repo.GetByID(context.Background(), owner, job.ID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, got.ID)

	_, err = repo.GetByID(context.Background(), intruder, job.ID)
	require.Error(t, err, "another tenant read a report job by id")
	assert.True(t, errors.Is(err, domain.ErrNotFound),
		"cross-tenant read must be indistinguishable from a missing job, got %v", err)
}

func TestReportJob_List_ScopedToTenant(t *testing.T) {
	repo := setupReportJobRepo(t)
	a, b := uuid.New(), uuid.New()
	seedJob(t, repo, a)
	seedJob(t, repo, a)
	seedJob(t, repo, b)

	got, err := repo.List(context.Background(), a, 25)
	require.NoError(t, err)
	assert.Len(t, got, 2, "another tenant's jobs leaked into the list")
	for _, j := range got {
		assert.Equal(t, a, j.TenantID)
	}
}

// The list feeds a status table; selecting every stored PDF to render it would
// move megabytes to describe kilobytes.
func TestReportJob_List_OmitsTheArtifact(t *testing.T) {
	repo := setupReportJobRepo(t)
	tenant := uuid.New()
	seedJob(t, repo, tenant)

	got, err := repo.List(context.Background(), tenant, 25)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Artifact, "List selected the artifact bytes")
	assert.Equal(t, 8, got[0].SizeBytes, "the size must still be reported")
}

// A mis-set TenantID must not let an update reach another tenant's row.
func TestReportJob_Update_CannotCrossTenants(t *testing.T) {
	repo := setupReportJobRepo(t)
	owner, intruder := uuid.New(), uuid.New()
	job := seedJob(t, repo, owner)

	forged := *job
	forged.TenantID = intruder
	forged.Title = "overwritten"
	err := repo.Update(context.Background(), &forged)
	require.Error(t, err, "an update crossed tenants")

	still, err := repo.GetByID(context.Background(), owner, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "ISO/IEC 27001 2022", still.Title, "the owner's row was modified")
}

func TestReportJob_Create_RequiresTenant(t *testing.T) {
	repo := setupReportJobRepo(t)
	err := repo.Create(context.Background(), &domain.ReportJob{
		ID: uuid.New(), Kind: domain.ReportKindComplianceFramework,
	})
	require.Error(t, err, "a tenant-less job was persisted")
}

// The artifact must survive the round trip: re-downloading has to return the
// document that was generated, not a fresh render.
func TestReportJob_ArtifactRoundTrips(t *testing.T) {
	repo := setupReportJobRepo(t)
	tenant := uuid.New()
	job := seedJob(t, repo, tenant)

	got, err := repo.GetByID(context.Background(), tenant, job.ID)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-1.4"), got.Artifact)
}
