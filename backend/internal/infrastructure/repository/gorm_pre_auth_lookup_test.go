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

// #808: GetOAuthProviderByEmail is the other pre-session OAuth lookup.
func TestOAuthProviderRepository_GetByEmailRunsUnderTheGuard(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE user_oauth_providers (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "user_oauth_providers", &domain.OAuthProvider{}))
	repo := NewGormOAuthProviderRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.CreateOAuthProvider(ctx, &domain.OAuthProvider{ID: uuid.New(), TenantID: uuid.New(), UserID: uuid.New(), Provider: "github", ProviderUserID: "s", Email: "a@example.com"}))

	got, err := repo.GetOAuthProviderByEmail(ctx, "a@example.com", "github")
	require.NoError(t, err)
	require.NotNil(t, got)
	none, err := repo.GetOAuthProviderByEmail(ctx, "b@example.com", "github")
	require.NoError(t, err)
	assert.Nil(t, none)
}
