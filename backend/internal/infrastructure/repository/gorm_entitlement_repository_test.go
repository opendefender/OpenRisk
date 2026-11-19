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

	n, err := repo.Count(ctx, tenant, ent.LimitFrameworks, time.Now())
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

	n, err := repo.Count(ctx, tenant, ent.LimitFrameworks, time.Now())
	require.NoError(t, err)
	require.Equal(t, 0, n)

	fw := domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "ISO 27001", Version: "2022"}
	require.NoError(t, db.Create(&fw).Error)
	n, err = repo.Count(ctx, tenant, ent.LimitFrameworks, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.False(t, ent.WithinLimit(ent.PlanFree, ent.LimitFrameworks, n), "Free is full at one")

	require.NoError(t, db.Delete(&fw).Error) // soft delete
	n, err = repo.Count(ctx, tenant, ent.LimitFrameworks, time.Now())
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

	n, err := repo.Count(context.Background(), uuid.Nil, ent.LimitFrameworks, time.Now())
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

// #419 — "current" usage is judged at the caller's instant, never time.Now().
//
// The entitlements service resolves plan and trial with its injected clock. A
// seat count that read the wall clock let one request see a trial as live and
// a pending invitation as expired, or the reverse. The expiry is fixed in the
// past on purpose: against the old code the "before expiry" cases read zero.

func TestCountUsers_JudgesInvitationExpiryAgainstTheGivenInstant(t *testing.T) {
	db := newMembershipTestDB(t)
	repo := NewGormEntitlementRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	expiry := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&domain.Invitation{
		ID: uuid.New(), OrganizationID: tenantA, Email: "seat@a.io", Role: domain.RoleUser,
		TokenHash: "419seat", Status: domain.InvitationPending, ExpiresAt: expiry,
		InvitedByID: uuid.New(), LastSentAt: expiry.Add(-7 * 24 * time.Hour), SendCount: 1,
	}).Error)

	for _, tc := range []struct {
		name string
		asOf time.Time
		want int
	}{
		{"a day before expiry it holds a seat", expiry.Add(-24 * time.Hour), 1},
		{"a second before expiry it holds a seat", expiry.Add(-time.Second), 1},
		{"exactly at expiry the seat is free", expiry, 0},
		{"a day after expiry the seat is free", expiry.Add(24 * time.Hour), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := repo.Count(ctx, tenantA, ent.LimitUsers, tc.asOf)
			require.NoError(t, err)
			require.Equal(t, tc.want, n, "seats at %s — the count is not using the instant it was given", tc.asOf.Format(time.RFC3339))
		})
	}

	n, err := repo.Count(ctx, tenantB, ent.LimitUsers, expiry.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, 0, n, "tenant B must not count tenant A's invitation")
}

func TestCountScansPerMonth_JudgesTheMonthAgainstTheGivenInstant(t *testing.T) {
	db := setupComplianceRepo(t).db
	require.NoError(t, db.Exec(`CREATE TABLE scan_jobs (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, created_at DATETIME, deleted_at DATETIME)`).Error)
	repo := NewGormEntitlementRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	monthStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{monthStart.Add(-time.Second), monthStart} {
		require.NoError(t, db.Table("scan_jobs").Create(map[string]any{
			"id": uuid.New(), "tenant_id": tenantA, "created_at": at,
		}).Error)
	}

	// The month opens AT its first instant, inclusive (`created_at >= since`):
	// a job started at 00:00:00 on the 1st is this month's, one a second
	// earlier is last month's.
	for _, tc := range []struct {
		name string
		asOf time.Time
		want int
	}{
		{"in March both jobs count", monthStart.Add(-time.Second), 2},
		{"at the first instant of April only April's job counts", monthStart, 1},
		{"later in April only April's job counts", monthStart.Add(15 * 24 * time.Hour), 1},
		{"in May neither counts", monthStart.AddDate(0, 1, 0), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := repo.Count(ctx, tenantA, ent.LimitScansPerMonth, tc.asOf)
			require.NoError(t, err)
			require.Equal(t, tc.want, n, "scans at %s — the month is not the one the instant falls in", tc.asOf.Format(time.RFC3339))
		})
	}

	n, err := repo.Count(ctx, tenantB, ent.LimitScansPerMonth, monthStart)
	require.NoError(t, err)
	require.Equal(t, 0, n, "tenant B must not count tenant A's scans")
}
