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
	ent "github.com/opendefender/openrisk/pkg/entitlements"
	"github.com/stretchr/testify/require"
)

// The framework count is what D-076's cap reads. RequireCapacity fails OPEN on
// a count error, so a broken count is no cap at all: the count gets its own
// test, against a real schema, rather than trusting the middleware's fake.

func TestCountFrameworks_Success(t *testing.T) {
	db := setupComplianceRepo(t).db
	repo := NewGormEntitlementRepository(db)
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()

	for _, fw := range []domain.ComplianceFramework{
		{TenantID: tenant, Name: "ISO 27001", Version: "2022"},
		{TenantID: tenant, Name: "NIST CSF", Version: "2.0"},
		{TenantID: other, Name: "ISO 27001", Version: "2022"},
		{TenantID: other, Name: "COBAC", Version: "R-2024"},
		{TenantID: other, Name: "ANTIC", Version: "2023"},
	} {
		fw.ID = uuid.New()
		require.NoError(t, db.Create(&fw).Error)
	}

	n, err := repo.Count(ctx, tenant, ent.LimitFrameworks)
	require.NoError(t, err)
	require.Equal(t, 2, n, "another tenant's frameworks must not count")
}

func TestCountFrameworks_NotFound(t *testing.T) {
	// A tenant with no framework counts zero, and a deleted framework frees its
	// slot: the next create succeeds.
	db := setupComplianceRepo(t).db
	repo := NewGormEntitlementRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	n, err := repo.Count(ctx, tenant, ent.LimitFrameworks)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	fw := domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "ISO 27001", Version: "2022"}
	require.NoError(t, db.Create(&fw).Error)
	n, err = repo.Count(ctx, tenant, ent.LimitFrameworks)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.False(t, ent.WithinLimit(ent.PlanFree, ent.LimitFrameworks, n), "Free is full at one")

	require.NoError(t, db.Delete(&fw).Error) // soft delete
	n, err = repo.Count(ctx, tenant, ent.LimitFrameworks)
	require.NoError(t, err)
	require.Equal(t, 0, n, "a soft-deleted framework is not active")
	require.True(t, ent.WithinLimit(ent.PlanFree, ent.LimitFrameworks, n))
}

func TestCountFrameworks_Unauthorized(t *testing.T) {
	// A count for the nil tenant reads nobody's frameworks, never everybody's.
	db := setupComplianceRepo(t).db
	repo := NewGormEntitlementRepository(db)
	fw := domain.ComplianceFramework{ID: uuid.New(), TenantID: uuid.New(), Name: "ISO 27001", Version: "2022"}
	require.NoError(t, db.Create(&fw).Error)

	n, err := repo.Count(context.Background(), uuid.Nil, ent.LimitFrameworks)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}
