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

func setupReportRepo(t *testing.T) *GormReportRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Report{}, &domain.ReportComment{}))
	return NewGormReportRepository(db)
}

func seedReport(t *testing.T, r *GormReportRepository, tenant uuid.UUID, state domain.ReportRunState) *domain.Report {
	t.Helper()
	rep := &domain.Report{
		TenantID: tenant, Type: domain.ReportTypeComplianceByFramework,
		Format: domain.ReportFormatPDF, Locale: domain.ReportLocaleFR,
		RunState: state, Lifecycle: domain.ReportLifecycleDraft, Version: 1,
		Title: "Rapport",
	}
	require.NoError(t, r.Create(context.Background(), rep))
	return rep
}

func TestReportRepo_TenantIsolation(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	rep := seedReport(t, repo, tenantA, domain.ReportRunSucceeded)

	// A report is a document about a tenant's posture; another tenant reading it
	// would be a disclosure of the whole register at once.
	_, err := repo.GetByID(ctx, tenantB, rep.ID)
	assert.Error(t, err, "another tenant must not read this report")

	got, err := repo.GetByID(ctx, tenantA, rep.ID)
	require.NoError(t, err)
	assert.Equal(t, "Rapport", got.Title)

	assert.Error(t, repo.Delete(ctx, tenantB, rep.ID), "cross-tenant delete must fail")
	_, err = repo.GetByID(ctx, tenantA, rep.ID)
	assert.NoError(t, err, "the report must survive a cross-tenant delete attempt")

	items, total, err := repo.List(ctx, tenantB, domain.ReportFilter{})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Zero(t, total)
}

// The claim must be exclusive. Two workers taking the same row would render one
// request twice, producing two artifacts and two hashes for one report.
func TestReportRepo_ClaimQueued_IsExclusive(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	rep := seedReport(t, repo, tenant, domain.ReportRunQueued)

	first, err := repo.ClaimQueued(ctx)
	require.NoError(t, err)
	require.NotNil(t, first, "the queued report should be claimable")
	assert.Equal(t, rep.ID, first.ID)
	assert.Equal(t, domain.ReportRunRunning, first.RunState,
		"claiming must move it out of the queue in the same statement")

	second, err := repo.ClaimQueued(ctx)
	require.NoError(t, err)
	assert.Nil(t, second, "a second worker must not claim the same report")
}

func TestReportRepo_ClaimQueued_EmptyQueue(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	seedReport(t, repo, uuid.New(), domain.ReportRunSucceeded)

	claimed, err := repo.ClaimQueued(ctx)
	require.NoError(t, err)
	assert.Nil(t, claimed, "nothing queued means nothing to claim, not an error")
}

// Oldest first: a report queued five minutes ago must not wait behind one queued
// now, or a busy tenant starves everyone else.
func TestReportRepo_ClaimQueued_TakesOldestFirst(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	older := seedReport(t, repo, tenant, domain.ReportRunQueued)
	// Force a distinct, later creation time — sqlite's resolution can collapse
	// two writes into the same instant.
	newer := seedReport(t, repo, tenant, domain.ReportRunQueued)
	require.NoError(t, repo.db.Model(&domain.Report{}).Where("id = ?", newer.ID).
		UpdateColumn("created_at", older.CreatedAt.Add(60_000_000_000)).Error) // +1 minute

	claimed, err := repo.ClaimQueued(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, older.ID, claimed.ID, "the oldest queued report should go first")
}

// The listing must not drag every document out of the database to render a table
// of titles.
func TestReportRepo_ListOmitsTheArtifact(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	rep := seedReport(t, repo, tenant, domain.ReportRunSucceeded)
	rep.Artifact = []byte("a large pretend document")
	rep.SizeBytes = len(rep.Artifact)
	rep.ContentHash = domain.ComputeContentHash(rep.Artifact)
	require.NoError(t, repo.Update(ctx, rep))

	items, _, err := repo.List(ctx, tenant, domain.ReportFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Empty(t, items[0].Artifact, "the listing must not carry document bytes")
	assert.Equal(t, len(rep.Artifact), items[0].SizeBytes, "but it should still report the size")
	assert.NotEmpty(t, items[0].ContentHash, "and the hash, so a list can show it")

	// The single read does carry them.
	one, err := repo.GetByID(ctx, tenant, rep.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, one.Artifact)
}

func TestReportRepo_LineageWalksBothWays(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	v1 := seedReport(t, repo, tenant, domain.ReportRunSucceeded)

	v2 := &domain.Report{
		TenantID: tenant, Type: v1.Type, Format: v1.Format, Locale: v1.Locale,
		RunState: domain.ReportRunSucceeded, Lifecycle: domain.ReportLifecycleDraft,
		Version: 2, Supersedes: &v1.ID, Title: "Rapport v2",
	}
	require.NoError(t, repo.Create(ctx, v2))

	v3 := &domain.Report{
		TenantID: tenant, Type: v1.Type, Format: v1.Format, Locale: v1.Locale,
		RunState: domain.ReportRunSucceeded, Lifecycle: domain.ReportLifecycleDraft,
		Version: 3, Supersedes: &v2.ID, Title: "Rapport v3",
	}
	require.NoError(t, repo.Create(ctx, v3))

	// Asked from the middle of the chain, the whole lineage must come back —
	// otherwise "version history" depends on which version you opened.
	chain, err := repo.Lineage(ctx, tenant, v2.ID)
	require.NoError(t, err)
	require.Len(t, chain, 3)
	assert.Equal(t, 3, chain[0].Version, "newest first")
	assert.Equal(t, 1, chain[2].Version)
}

func TestReportRepo_DeleteTakesItsComments(t *testing.T) {
	repo := setupReportRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	rep := seedReport(t, repo, tenant, domain.ReportRunSucceeded)
	require.NoError(t, repo.AddComment(ctx, &domain.ReportComment{
		TenantID: tenant, ReportID: rep.ID, AuthorID: uuid.New(), Body: "à revoir",
	}))

	comments, err := repo.ListComments(ctx, tenant, rep.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)

	require.NoError(t, repo.Delete(ctx, tenant, rep.ID))
	comments, err = repo.ListComments(ctx, tenant, rep.ID)
	require.NoError(t, err)
	assert.Empty(t, comments, "a review remark on a report that no longer exists is litter, not history")
}
