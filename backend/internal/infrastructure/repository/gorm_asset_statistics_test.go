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

	"github.com/opendefender/openrisk/internal/domain"
)

// seedAsset writes one asset with an explicit creation time, so the period tests
// are about the boundary logic and not about when the suite happened to run.
func seedAsset(t *testing.T, repo *GormAssetRepository, tenantID uuid.UUID, name, typ, category, source string, crit domain.AssetCriticality, created time.Time) *domain.Asset {
	t.Helper()
	a := &domain.Asset{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        name,
		Type:        typ,
		Category:    domain.AssetCategory(category),
		Source:      source,
		Criticality: crit,
		CreatedAt:   created,
		UpdatedAt:   created,
	}
	require.NoError(t, repo.Create(context.Background(), a))
	// GORM stamps CreatedAt itself on insert when the field is zero and honours
	// it when set — but the autoUpdateTime tag rewrites UpdatedAt regardless, so
	// pin created_at explicitly rather than trusting the round-trip.
	require.NoError(t, repo.db.Exec(`UPDATE assets SET created_at = ? WHERE id = ?`, created, a.ID).Error)
	return a
}

func sumInt64(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

// The invariants the dashboard displays side by side. If a breakdown does not add
// up to the total beside it, the user cannot tell whether they are looking at a
// bug or a business rule — so these are asserted, not assumed.
func TestAssetStatistics_BreakdownsReconcileWithTotal(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	seedAsset(t, repo, tenant, "web-01", "Server", "server", "MANUAL", domain.CriticalityCritical, now.AddDate(0, 0, -1))
	seedAsset(t, repo, tenant, "web-02", "Server", "server", "SCANNER", domain.CriticalityHigh, now.AddDate(0, 0, -2))
	seedAsset(t, repo, tenant, "db-01", "Database", "database", "MANUAL", domain.CriticalityCritical, now.AddDate(0, 0, -40))
	seedAsset(t, repo, tenant, "laptop-7", "Laptop", "workstation", "MANUAL", domain.CriticalityLow, now.AddDate(0, 0, -100))
	// The two rows that make the invariants non-trivial: no category, no type.
	seedAsset(t, repo, tenant, "legacy-import", "", "", "MANUAL", domain.CriticalityMedium, now.AddDate(0, 0, -200))

	stats, err := repo.Statistics(ctx, tenant, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, int64(5), stats.Total)
	assert.Equal(t, stats.Total, sumInt64(stats.ByCriticality), "Σ by_criticality must equal total")
	assert.Equal(t, stats.Total, sumInt64(stats.ByCategory)+stats.Uncategorised, "Σ by_category + uncategorised must equal total")
	assert.Equal(t, stats.Total, sumInt64(stats.ByType)+stats.Untyped, "Σ by_type + untyped must equal total")
	assert.Equal(t, stats.Total, sumInt64(stats.BySource), "Σ by_source must equal total")

	assert.Equal(t, int64(1), stats.Uncategorised)
	assert.Equal(t, int64(1), stats.Untyped)
	assert.Equal(t, int64(2), stats.ByCriticality[string(domain.CriticalityCritical)])
	assert.Equal(t, int64(2), stats.ByType["Server"])
	assert.Equal(t, int64(1), stats.BySource["SCANNER"])
	assert.Equal(t, int64(3), stats.DistinctTypes, "blank types are not a type")
}

// All four bands are always present. A zero is a fact; an absent key makes the
// client decide what it meant.
func TestAssetStatistics_EmptyInventoryStatesEveryBand(t *testing.T) {
	repo := setupAssetRepo(t)
	stats, err := repo.Statistics(context.Background(), uuid.New(), nil, nil)
	require.NoError(t, err)

	assert.Equal(t, int64(0), stats.Total)
	require.Len(t, stats.ByCriticality, 4)
	for _, band := range []domain.AssetCriticality{
		domain.CriticalityCritical, domain.CriticalityHigh,
		domain.CriticalityMedium, domain.CriticalityLow,
	} {
		assert.Contains(t, stats.ByCriticality, string(band))
	}
	assert.Empty(t, stats.ByCategory)
	assert.Equal(t, int64(0), stats.DistinctTypes)
}

// RULE #2. The whole point of the aggregate is that it never sees another
// tenant's rows, and an aggregate is exactly where that is hardest to notice —
// one wrong number, no row to trace it back to.
func TestAssetStatistics_IsTenantScoped(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	seedAsset(t, repo, a, "a-1", "Server", "server", "MANUAL", domain.CriticalityCritical, now)
	seedAsset(t, repo, a, "a-2", "Server", "server", "MANUAL", domain.CriticalityLow, now)
	seedAsset(t, repo, b, "b-1", "Database", "database", "SCANNER", domain.CriticalityCritical, now)
	seedAsset(t, repo, b, "b-2", "Database", "database", "SCANNER", domain.CriticalityCritical, now)
	seedAsset(t, repo, b, "b-3", "Database", "database", "SCANNER", domain.CriticalityHigh, now)

	statsA, err := repo.Statistics(ctx, a, nil, nil)
	require.NoError(t, err)
	statsB, err := repo.Statistics(ctx, b, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, int64(2), statsA.Total)
	assert.Equal(t, int64(3), statsB.Total)
	assert.NotContains(t, statsA.ByType, "Database", "tenant A must not see tenant B's types")
	assert.NotContains(t, statsB.ByType, "Server", "tenant B must not see tenant A's types")
	assert.NotContains(t, statsA.BySource, "SCANNER")
}

// A deleted asset is not part of the estate. Counting it is how a dashboard total
// climbs above the inventory the user can actually see — invisible until someone
// deletes something.
func TestAssetStatistics_ExcludesSoftDeleted(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	seedAsset(t, repo, tenant, "keep", "Server", "server", "MANUAL", domain.CriticalityHigh, now)
	gone := seedAsset(t, repo, tenant, "gone", "Server", "server", "MANUAL", domain.CriticalityCritical, now)
	require.NoError(t, repo.Delete(ctx, gone.ID, tenant))

	stats, err := repo.Statistics(ctx, tenant, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.Total)
	assert.Equal(t, int64(0), stats.ByCriticality[string(domain.CriticalityCritical)])
	assert.Equal(t, int64(1), stats.ByType["Server"])
}

// The period narrows exactly one field. Everything else is a point-in-time stock:
// "how many critical assets do we have" does not become a different question when
// a date range is picked.
func TestAssetStatistics_PeriodNarrowsOnlyAddedInPeriod(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	seedAsset(t, repo, tenant, "old", "Server", "server", "MANUAL", domain.CriticalityCritical, now.AddDate(0, 0, -60))
	seedAsset(t, repo, tenant, "recent", "Server", "server", "MANUAL", domain.CriticalityHigh, now.AddDate(0, 0, -3))
	seedAsset(t, repo, tenant, "today", "Laptop", "workstation", "MANUAL", domain.CriticalityLow, now)

	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 1)
	stats, err := repo.Statistics(ctx, tenant, &from, &to)
	require.NoError(t, err)

	assert.Equal(t, int64(3), stats.Total, "the stock is never narrowed by the window")
	assert.Equal(t, int64(1), stats.ByCriticality[string(domain.CriticalityCritical)], "the 60-day-old critical asset still exists")
	assert.Equal(t, int64(2), stats.AddedInPeriod, "only the flow counter is period-scoped")
}

// [from, to): from inclusive, to exclusive. Consecutive windows tile, and a row
// on the boundary belongs to exactly one of them.
func TestAssetStatistics_PeriodBoundsAreHalfOpen(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()

	aug1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	sep1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seedAsset(t, repo, tenant, "on-from", "Server", "server", "MANUAL", domain.CriticalityLow, aug1)
	seedAsset(t, repo, tenant, "on-to", "Server", "server", "MANUAL", domain.CriticalityLow, sep1)

	august, err := repo.Statistics(ctx, tenant, &aug1, &sep1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), august.AddedInPeriod, "from is inclusive, to is exclusive")

	oct1 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	september, err := repo.Statistics(ctx, tenant, &sep1, &oct1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), september.AddedInPeriod, "the next window owns the shared boundary")
}

// A tenant with a long tail of free-text labels must not produce a
// thousand-key object — and the client must be able to say "+N more" rather than
// implying the list it received is complete.
func TestAssetStatistics_CapsFreeTextTypesAndReportsTheTruncation(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

	distinct := domain.AssetTypeCap + 5
	for i := 0; i < distinct; i++ {
		seedAsset(t, repo, tenant, "a"+string(rune('a'+i)), "type-"+string(rune('a'+i)), "server", "MANUAL", domain.CriticalityLow, now)
	}

	stats, err := repo.Statistics(ctx, tenant, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(distinct), stats.Total)
	assert.Len(t, stats.ByType, domain.AssetTypeCap)
	assert.Equal(t, int64(5), stats.TypesTruncated)
	assert.Equal(t, int64(distinct), stats.DistinctTypes, "the KPI reports the true count, not the capped one")
}

// Determinism: the same inventory must always produce the same breakdown, or a
// chart's colours shuffle between refreshes for no reason.
func TestAssetStatistics_TypeOrderIsStable(t *testing.T) {
	repo := setupAssetRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	for i := 0; i < domain.AssetTypeCap+3; i++ {
		seedAsset(t, repo, tenant, "n"+string(rune('a'+i)), "t"+string(rune('a'+i)), "server", "MANUAL", domain.CriticalityLow, now)
	}

	first, err := repo.Statistics(ctx, tenant, nil, nil)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		again, err := repo.Statistics(ctx, tenant, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, first.ByType, again.ByType)
	}
}

// Fail closed. Counting every tenant's assets because the context was not
// resolved is the one outcome worse than an error.
func TestAssetStatistics_RefusesWithoutTenant(t *testing.T) {
	repo := setupAssetRepo(t)
	_, err := repo.Statistics(context.Background(), uuid.Nil, nil, nil)
	require.Error(t, err)
}
