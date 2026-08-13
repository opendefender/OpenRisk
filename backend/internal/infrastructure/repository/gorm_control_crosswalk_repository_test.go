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

func setupMappingRepo(t *testing.T) *GormControlCrosswalkRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	// Schema from the model. The crosswalk model carries no Postgres-only default
	// any more, precisely so this can be derived rather than hand-written and left
	// to drift.
	require.NoError(t, db.AutoMigrate(&domain.ControlCrosswalk{}))
	return NewGormControlCrosswalkRepository(db)
}

func TestControlMappingRepo_CreateAndExistsBothDirections(t *testing.T) {
	repo := setupMappingRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	a, b := uuid.New(), uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.ControlCrosswalk{
		ID: uuid.New(), TenantID: tenant, SourceControlID: a, TargetControlID: b, Coverage: domain.CoverageFull,
	}))

	// Exists must be symmetric: A→B and B→A are the same mapping.
	fwd, err := repo.Exists(ctx, tenant, a, b)
	require.NoError(t, err)
	assert.True(t, fwd)
	rev, err := repo.Exists(ctx, tenant, b, a)
	require.NoError(t, err)
	assert.True(t, rev, "existence check must be direction-independent")

	// A different pair does not exist.
	none, err := repo.Exists(ctx, tenant, a, uuid.New())
	require.NoError(t, err)
	assert.False(t, none)
}

func TestControlMappingRepo_ListAllAndByControl(t *testing.T) {
	repo := setupMappingRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.ControlCrosswalk{ID: uuid.New(), TenantID: tenant, SourceControlID: a, TargetControlID: b}))
	require.NoError(t, repo.Create(ctx, &domain.ControlCrosswalk{ID: uuid.New(), TenantID: tenant, SourceControlID: b, TargetControlID: c}))

	all, err := repo.List(ctx, tenant, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Scoped to control a → only the a↔b mapping.
	scoped, err := repo.List(ctx, tenant, &a)
	require.NoError(t, err)
	assert.Len(t, scoped, 1)

	// Scoped to b → both (b is source of one, target of the other).
	scopedB, err := repo.List(ctx, tenant, &b)
	require.NoError(t, err)
	assert.Len(t, scopedB, 2)
}

func TestControlMappingRepo_TenantIsolationAndDelete(t *testing.T) {
	repo := setupMappingRepo(t)
	ctx := context.Background()
	t1, t2 := uuid.New(), uuid.New()
	a, b := uuid.New(), uuid.New()

	m := &domain.ControlCrosswalk{ID: uuid.New(), TenantID: t1, SourceControlID: a, TargetControlID: b}
	require.NoError(t, repo.Create(ctx, m))

	// Another tenant sees nothing.
	other, err := repo.List(ctx, t2, nil)
	require.NoError(t, err)
	assert.Empty(t, other)

	// Another tenant cannot delete it (reads back as not found).
	err = repo.Delete(ctx, m.ID, t2)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Owner deletes it.
	require.NoError(t, repo.Delete(ctx, m.ID, t1))
	mine, err := repo.List(ctx, t1, nil)
	require.NoError(t, err)
	assert.Empty(t, mine)
}
