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

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #808: the password-reset path looks a token up by hash before any tenant is
// known. password_reset_tokens has no tenant column, so the guard must let the
// whole flow through; this runs it under testdb so staging cannot break on it.
func TestPasswordResetRepository_PreAuthFlowRunsUnderTheGuard(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE password_reset_tokens (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "password_reset_tokens", &domain.PasswordResetToken{}))
	repo := NewGormPasswordResetRepository(db)
	ctx := context.Background()

	hash := "h" + uuid.NewString()
	hash = (hash + hash)[:64]
	userID := uuid.New()
	tok := &domain.PasswordResetToken{ID: uuid.New(), EmailHash: "e", UserID: &userID, TokenHash: &hash, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, repo.Create(ctx, tok))

	got, err := repo.FindByTokenHash(ctx, hash)
	require.NoError(t, err)
	require.NotNil(t, got)
	miss, err := repo.FindByTokenHash(ctx, "nope")
	require.NoError(t, err)
	assert.Nil(t, miss)

	claimed, err := repo.ClaimToken(ctx, got.ID)
	require.NoError(t, err)
	assert.True(t, claimed)
	again, err := repo.ClaimToken(ctx, got.ID)
	require.NoError(t, err)
	assert.False(t, again, "a token is claimed exactly once")
}

// #808: MarkCompleted carries the organisation, so a request id paired with the
// wrong organisation completes nothing.
func TestOrgDeletionRepository_MarkCompleted_CarriesTheOrganisation(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE org_deletion_requests (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "org_deletion_requests", &domain.OrgDeletionRequest{}))
	repo := NewGormOrgDeletionRepository(db)
	ctx := context.Background()

	orgA, orgB := uuid.New(), uuid.New()
	req := &domain.OrgDeletionRequest{ID: uuid.New(), OrganizationID: orgA, RequestedBy: uuid.New(), Status: domain.DeletionPending, ScheduledPurgeAt: time.Now().Add(-time.Hour)}
	require.NoError(t, db.Create(req).Error)

	require.NoError(t, repo.MarkCompleted(ctx, orgB, req.ID, time.Now()))
	due, err := repo.ListDue(ctx, time.Now())
	require.NoError(t, err)
	assert.Len(t, due, 1, "another organisation must not complete this request")

	require.NoError(t, repo.MarkCompleted(ctx, orgA, req.ID, time.Now()))
	due, err = repo.ListDue(ctx, time.Now())
	require.NoError(t, err)
	assert.Empty(t, due)
}

// #808: the OAuth sign-in resolves a link by provider subject before any tenant
// is known, and TouchLogin carries the link's tenant.
func TestOAuthLinkRepository_PreAuthLookupAndTouchRunUnderTheGuard(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE user_oauth_providers (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "user_oauth_providers", &domain.OAuthProvider{}))
	repo := NewGormOAuthLinkRepository(db)
	ctx := context.Background()

	tenant, user := uuid.New(), uuid.New()
	link := &domain.OAuthProvider{ID: uuid.New(), TenantID: tenant, UserID: user, Provider: "google", ProviderUserID: "sub-1"}
	require.NoError(t, repo.Create(ctx, link))

	got, err := repo.FindByProviderSubject(ctx, "google", "sub-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	links, err := repo.ListByUser(ctx, user)
	require.NoError(t, err)
	assert.Len(t, links, 1)

	require.NoError(t, repo.TouchLogin(ctx, uuid.New(), link.ID, time.Now()))
	after, err := repo.FindByProviderSubject(ctx, "google", "sub-1")
	require.NoError(t, err)
	assert.Nil(t, after.LastLoginAt, "another tenant must not touch this link")
	require.NoError(t, repo.TouchLogin(ctx, tenant, link.ID, time.Now()))
	after, err = repo.FindByProviderSubject(ctx, "google", "sub-1")
	require.NoError(t, err)
	assert.NotNil(t, after.LastLoginAt)
}

// #808: re-ingesting a finding that already exists updates it through its
// tenant, and the same finding of another tenant is left alone.
func TestVulnerabilityRepo_UpsertReIngest_WritesThroughTheTenant(t *testing.T) {
	repo, db := setupVulnStatsRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	mk := func(tenant uuid.UUID, title string) *domain.Vulnerability {
		return &domain.Vulnerability{ID: uuid.New(), TenantID: tenant, Source: "scanner", ExternalID: "ext-1", CVEID: "CVE-2026-0001", Title: title, Severity: domain.VulnSeverity("high"), Status: domain.VulnStatusOpen}
	}
	created, err := repo.Upsert(ctx, mk(a, "first"))
	require.NoError(t, err)
	require.True(t, created)
	_, err = repo.Upsert(ctx, mk(b, "tenant b"))
	require.NoError(t, err)

	created, err = repo.Upsert(ctx, mk(a, "refreshed"))
	require.NoError(t, err)
	assert.False(t, created, "the second ingest updates the existing finding")

	var gotA, gotB domain.Vulnerability
	require.NoError(t, db.Where("tenant_id = ?", a).First(&gotA).Error)
	require.NoError(t, db.Where("tenant_id = ?", b).First(&gotB).Error)
	assert.Equal(t, "refreshed", gotA.Title)
	assert.Equal(t, "tenant b", gotB.Title)

	gotA.Title = "via update"
	require.NoError(t, repo.Update(ctx, &gotA))
	foreign := gotA
	foreign.TenantID = b
	assert.Error(t, repo.Update(ctx, &foreign), "a row of tenant a must not be written under tenant b")
}

// #808: an organisation role is rewritten through its organisation.
func TestOrganizationRoleRepository_Update_WritesThroughTheOrganisation(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE organization_roles (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "organization_roles", &domain.OrganizationRole{}))
	repo := NewGormOrganizationRoleRepository(db)
	ctx := context.Background()

	org := uuid.New()
	role := &domain.OrganizationRole{ID: uuid.New(), OrganizationID: org, Name: "auditor", IsActive: true, Permissions: []byte("[]")}
	require.NoError(t, repo.Create(ctx, role))

	role.Name = "auditor (renamed)"
	require.NoError(t, repo.Update(ctx, role))
	got, err := repo.GetByOrganizationID(ctx, org)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "auditor (renamed)", got[0].Name)

	foreign := *role
	foreign.OrganizationID = uuid.New()
	foreign.Name = "stolen"
	assert.Error(t, repo.Update(ctx, &foreign), "another organisation must not rewrite this role")
	got, err = repo.GetByOrganizationID(ctx, org)
	require.NoError(t, err)
	assert.Equal(t, "auditor (renamed)", got[0].Name)
}
