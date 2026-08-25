// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// setupMFAPolicyRepo builds the schema with AutoMigrate rather than hand-written
// DDL. domain.MFAPolicy carries no Postgres-only column default precisely so
// this works — hand-written test DDL has drifted from the models twice in this
// codebase, and the drift is only ever discovered by a failure that looks like
// something else.
func setupMFAPolicyRepo(t *testing.T) *GormMFAPolicyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.MFAPolicy{}))
	return NewGormMFAPolicyRepository(db)
}

func TestMFAPolicyRepo_UnsavedTenantReadsAsNil(t *testing.T) {
	// nil, not a synthesised row: the settings screen has to be able to tell
	// "saved" from "never saved".
	repo := setupMFAPolicyRepo(t)
	got, err := repo.GetMFAPolicy(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMFAPolicyRepo_CreatesThenUpdatesInPlace(t *testing.T) {
	repo := setupMFAPolicyRepo(t)
	ctx := context.Background()
	tenant, actor := uuid.New(), uuid.New()

	p := &domain.MFAPolicy{TenantID: tenant, GraceDays: 3, UpdatedByID: &actor}
	require.NoError(t, repo.SaveMFAPolicy(ctx, p))
	require.NotEqual(t, uuid.Nil, p.ID)
	first := p.ID

	q := &domain.MFAPolicy{TenantID: tenant, GraceDays: 14}
	require.NoError(t, repo.SaveMFAPolicy(ctx, q))
	assert.Equal(t, first, q.ID, "one policy per tenant — a second save must not make a second row")

	got, err := repo.GetMFAPolicy(ctx, tenant)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 14, got.GraceDays)
}

func TestMFAPolicyRepo_IsTenantScoped(t *testing.T) {
	// RULE #2. Tenant A at 7 days, tenant B at 1 day: neither reads the other's,
	// and writing one leaves the other where it was.
	repo := setupMFAPolicyRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	require.NoError(t, repo.SaveMFAPolicy(ctx, &domain.MFAPolicy{TenantID: a, GraceDays: 7}))
	require.NoError(t, repo.SaveMFAPolicy(ctx, &domain.MFAPolicy{TenantID: b, GraceDays: 1}))

	gotA, err := repo.GetMFAPolicy(ctx, a)
	require.NoError(t, err)
	gotB, err := repo.GetMFAPolicy(ctx, b)
	require.NoError(t, err)
	require.NotNil(t, gotA)
	require.NotNil(t, gotB)
	assert.Equal(t, 7, gotA.GraceDays)
	assert.Equal(t, 1, gotB.GraceDays)
	assert.NotEqual(t, gotA.ID, gotB.ID)

	require.NoError(t, repo.SaveMFAPolicy(ctx, &domain.MFAPolicy{TenantID: a, GraceDays: 30}))
	gotB, err = repo.GetMFAPolicy(ctx, b)
	require.NoError(t, err)
	assert.Equal(t, 1, gotB.GraceDays, "tenant B's policy did not move")
}

func TestMFAPolicyRepo_ForgedIDCannotSteerAnotherTenantsRow(t *testing.T) {
	// The tenant is the identity here, not the id the client supplies. Passing
	// tenant A's policy id while claiming tenant B must create B's own row and
	// leave A untouched.
	repo := setupMFAPolicyRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	victim := &domain.MFAPolicy{TenantID: a, GraceDays: 7}
	require.NoError(t, repo.SaveMFAPolicy(ctx, victim))

	forged := &domain.MFAPolicy{ID: victim.ID, TenantID: b, GraceDays: 0}
	require.NoError(t, repo.SaveMFAPolicy(ctx, forged))

	gotA, err := repo.GetMFAPolicy(ctx, a)
	require.NoError(t, err)
	require.NotNil(t, gotA)
	assert.Equal(t, 7, gotA.GraceDays, "tenant A's window was not moved by a forged id")
	assert.NotEqual(t, victim.ID, forged.ID, "the supplied id is ignored; a fresh row is created")
}

func TestMFAPolicyRepo_RefusesOutOfRangeAndMissingTenant(t *testing.T) {
	repo := setupMFAPolicyRepo(t)
	ctx := context.Background()

	require.Error(t, repo.SaveMFAPolicy(ctx, &domain.MFAPolicy{TenantID: uuid.New(), GraceDays: 91}))
	require.Error(t, repo.SaveMFAPolicy(ctx, &domain.MFAPolicy{TenantID: uuid.Nil, GraceDays: 7}))
	require.Error(t, repo.SaveMFAPolicy(ctx, nil))

	_, err := repo.GetMFAPolicy(ctx, uuid.Nil)
	require.Error(t, err)
}
