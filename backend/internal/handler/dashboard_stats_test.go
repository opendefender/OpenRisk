// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain/timeframe"
)

// The clock every test resolves its window against, so assertions about day
// boundaries are about the boundary logic and not about when the suite ran.
var statsNow = time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)

// newStatsDB builds the columns ComputeDashboardStats actually reads. It is
// deliberately the minimum: a wider hand-written DDL is a wider surface to drift
// away from the model, which is how TestRiskCRUDFlow spent months red.
func newStatsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			score REAL DEFAULT 0,
			probability REAL DEFAULT 0,
			impact REAL DEFAULT 0,
			status TEXT,
			slexaf REAL,
			data_loss_cost_xaf REAL,
			fines_xaf REAL,
			other_direct_cost_xaf REAL,
			downtime_hours REAL,
			hourly_downtime_cost_xaf REAL,
			created_at DATETIME,
			deleted_at DATETIME
		);`).Error)
	return db
}

type seed struct {
	score       float64
	probability float64
	impact      float64
	status      string
	created     time.Time
}

func insertRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID, s seed) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO risks (id, tenant_id, score, probability, impact, status, created_at) VALUES (?,?,?,?,?,?,?)`,
		uuid.NewString(), tenant.String(), s.score, s.probability, s.impact, s.status, s.created).Error)
}

func windowFor(t *testing.T, preset string) timeframe.Window {
	t.Helper()
	w, err := timeframe.Parse(preset, "", "", statsNow)
	require.NoError(t, err)
	return w
}

// Pins /stats severity bucketing to the Score Engine bands (score = P x I x AC):
// >=7 critical, >=4 high, >=2 medium, <2 low — the same thresholds the register's
// criticality column is derived from. The previous 20/15/10 thresholds assumed a
// 1-5 x 1-5 scale and reported 0 critical / 0 high on a register the register
// itself showed as critical, contradicting /analytics/executive (audit-2026 #243).
//
// This calls ComputeDashboardStats. The test it replaces re-typed the handler's
// SQL into its own body and asserted against the copy, which passes just as
// happily when the handler drifts away from it.
func TestDashboardStats_SeverityMatchesScoreEngineBands(t *testing.T) {
	db := newStatsDB(t)
	tenant, other := uuid.New(), uuid.New()

	for _, score := range []float64{
		10.0,  // critical
		7.0,   // critical (>= 7 boundary)
		6.999, // high     (< 7)
		4.0,   // high     (>= 4 boundary)
		3.999, // medium   (< 4)
		2.0,   // medium   (>= 2 boundary)
		1.999, // low      (< 2)
		0.1,   // low
	} {
		insertRisk(t, db, tenant, seed{score: score, created: statsNow})
	}
	// A high-scoring risk in ANOTHER tenant must not be counted.
	insertRisk(t, db, other, seed{score: 30.0, created: statsNow})

	stats, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "all"))
	require.NoError(t, err)

	sev := stats.RisksBySeverity
	assert.Equal(t, int64(8), stats.TotalRisks, "only this tenant's risks")
	assert.Equal(t, int64(2), sev["CRITICAL"], "score 10 and 7.0")
	assert.Equal(t, int64(2), sev["HIGH"], "score 6.999 and 4.0")
	assert.Equal(t, int64(2), sev["MEDIUM"], "score 3.999 and 2.0")
	assert.Equal(t, int64(2), sev["LOW"], "score 1.999 and 0.1")
	assert.Equal(t, int64(4), stats.HighRisks, "high-or-above KPI = high + critical")

	// The bands must partition the register fully: no risk uncounted, none
	// double-counted, so the dashboard total always reconciles with the sum of
	// the breakdown printed beside it.
	assert.Equal(t, stats.TotalRisks, sev["CRITICAL"]+sev["HIGH"]+sev["MEDIUM"]+sev["LOW"])
}

// The heatmap's 25 cells must account for every risk, or the matrix and the
// total beside it disagree on the same screen.
func TestDashboardStats_MatrixCoversEveryRisk(t *testing.T) {
	db := newStatsDB(t)
	tenant := uuid.New()

	// Boundary values of both bandings, including the zeroes that used to fall
	// out of the matrix entirely.
	for _, s := range []seed{
		{probability: 0, impact: 0},      // band 1 x 1
		{probability: 0.2, impact: 2},    // band 1 x 1 (CEIL boundary)
		{probability: 0.21, impact: 2.1}, // band 2 x 2
		{probability: 0.6, impact: 6},    // band 3 x 3
		{probability: 1.0, impact: 10},   // band 5 x 5
	} {
		s.created = statsNow
		insertRisk(t, db, tenant, s)
	}

	stats, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "all"))
	require.NoError(t, err)

	var counted int64
	for _, cell := range stats.RiskMatrix {
		counted += cell.Count
		assert.GreaterOrEqual(t, cell.Probability, 1)
		assert.LessOrEqual(t, cell.Probability, 5)
		assert.GreaterOrEqual(t, cell.Impact, 1)
		assert.LessOrEqual(t, cell.Impact, 5)
	}
	assert.Equal(t, stats.TotalRisks, counted, "every risk lands in exactly one cell")
}

// The stock counters answer "what exists", and picking a date range does not
// change that question. This is the invariant that keeps the dashboard
// reconcilable with the register it links to.
func TestDashboardStats_PeriodNarrowsOnlyTheFlow(t *testing.T) {
	db := newStatsDB(t)
	tenant := uuid.New()

	insertRisk(t, db, tenant, seed{score: 9, created: statsNow.AddDate(0, 0, -100)})
	insertRisk(t, db, tenant, seed{score: 5, created: statsNow.AddDate(0, 0, -3)})
	insertRisk(t, db, tenant, seed{score: 1, created: statsNow})

	all, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "all"))
	require.NoError(t, err)
	week, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "7d"))
	require.NoError(t, err)

	assert.Equal(t, int64(3), all.TotalRisks)
	assert.Equal(t, int64(3), week.TotalRisks, "the stock is identical under any window")
	assert.Equal(t, all.RisksBySeverity, week.RisksBySeverity, "so is the severity breakdown")
	assert.Equal(t, int64(3), all.OpenedInPeriod, "an unbounded window opened everything")
	assert.Equal(t, int64(2), week.OpenedInPeriod, "the 100-day-old risk was not opened this week")
	assert.Equal(t, []string{"opened_in_period", "risk_trend"}, week.PeriodAppliesTo,
		"the response must name what the period touched")
}

// The trend's cumulative line must start from the risks that already existed, not
// from zero — a chart that restarts at zero for a two-year-old tenant reads as
// data loss.
func TestDashboardStats_TrendCumulativeIncludesPreWindowBaseline(t *testing.T) {
	db := newStatsDB(t)
	tenant := uuid.New()

	for i := 0; i < 4; i++ {
		insertRisk(t, db, tenant, seed{score: 8, created: statsNow.AddDate(0, 0, -200)})
	}
	insertRisk(t, db, tenant, seed{score: 3, created: statsNow.AddDate(0, 0, -2)})
	insertRisk(t, db, tenant, seed{score: 3, created: statsNow})

	stats, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "7d"))
	require.NoError(t, err)

	points := stats.RiskTrend.Points
	require.Len(t, points, 7, "7d must produce seven daily buckets")
	assert.Equal(t, "day", stats.RiskTrend.Granularity)
	assert.Equal(t, int64(4), points[0].CumulativeTotal, "the baseline is carried in")
	assert.Equal(t, int64(6), points[len(points)-1].CumulativeTotal, "and ends at the live total")
	assert.Equal(t, stats.TotalRisks, points[len(points)-1].CumulativeTotal,
		"the last point of the cumulative line IS the headline total")

	var opened int64
	for _, p := range points {
		opened += p.Opened
		// All four bands are always stated, so a zero is a fact and not a
		// missing key the client has to interpret.
		assert.Len(t, p.OpenedByBand, 4)
	}
	assert.Equal(t, stats.OpenedInPeriod, opened, "Σ opened over the series == opened_in_period")
}

// Buckets are half-open and tile: a risk created at the instant a bucket starts
// belongs to that bucket and to no other.
func TestDashboardStats_TrendBucketsTileWithoutGapOrOverlap(t *testing.T) {
	db := newStatsDB(t)
	tenant := uuid.New()

	// Midnight exactly, on each of three consecutive days inside the window.
	for d := 3; d >= 1; d-- {
		day := time.Date(2026, 8, 21-d, 0, 0, 0, 0, time.UTC)
		insertRisk(t, db, tenant, seed{score: 5, created: day})
	}

	stats, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "7d"))
	require.NoError(t, err)

	byBucket := map[string]int64{}
	for _, p := range stats.RiskTrend.Points {
		byBucket[p.Bucket] = p.Opened
	}
	assert.Equal(t, int64(1), byBucket["2026-08-18"])
	assert.Equal(t, int64(1), byBucket["2026-08-19"])
	assert.Equal(t, int64(1), byBucket["2026-08-20"])
	assert.Equal(t, int64(0), byBucket["2026-08-21"], "today is present and honestly empty")
}

// An unbounded window counts everything, but its SERIES is capped — and the
// response must report the bounds it actually used rather than the ones asked
// for, so "all time" never silently means "since last year".
func TestDashboardStats_UnboundedWindowReportsTheCappedTrendBounds(t *testing.T) {
	db := newStatsDB(t)
	tenant := uuid.New()
	insertRisk(t, db, tenant, seed{score: 5, created: statsNow.AddDate(-3, 0, 0)})
	insertRisk(t, db, tenant, seed{score: 5, created: statsNow})

	stats, err := ComputeDashboardStats(context.Background(), db, tenant, windowFor(t, "all"))
	require.NoError(t, err)

	assert.Equal(t, int64(2), stats.TotalRisks)
	assert.Equal(t, int64(2), stats.OpenedInPeriod, "the COUNT is genuinely all-time")
	assert.Equal(t, "week", stats.RiskTrend.Granularity)
	assert.Equal(t, "", stats.Period.From, "an unbounded window invents no start date")

	from, err := time.Parse(time.RFC3339, stats.RiskTrend.From)
	require.NoError(t, err)
	assert.True(t, from.After(statsNow.AddDate(-2, 0, 0)),
		"the series is capped and says so, even though the count is not")
	// The three-year-old risk is outside the plotted range but still in the
	// cumulative baseline, so the line does not begin below the truth.
	assert.Equal(t, int64(1), stats.RiskTrend.Points[0].CumulativeTotal)
}

// An empty tenant is an empty tenant. Buckets are still emitted, at zero: a chart
// with no x-axis is indistinguishable from a chart that failed to load.
func TestDashboardStats_EmptyTenantStillDrawsTheAxis(t *testing.T) {
	db := newStatsDB(t)
	stats, err := ComputeDashboardStats(context.Background(), db, uuid.New(), windowFor(t, "30d"))
	require.NoError(t, err)

	assert.Equal(t, int64(0), stats.TotalRisks)
	assert.Equal(t, int64(0), stats.OpenedInPeriod)
	assert.Empty(t, stats.RiskMatrix)
	assert.Len(t, stats.RiskTrend.Points, 30)
	for _, p := range stats.RiskTrend.Points {
		assert.Equal(t, int64(0), p.Opened)
		assert.Equal(t, int64(0), p.CumulativeTotal)
	}
}

// RULE #2, at the aggregation itself. An aggregate is where a leak is hardest to
// notice: one wrong number, and no row to trace it back to.
func TestDashboardStats_IsTenantScopedIncludingTheTrend(t *testing.T) {
	db := newStatsDB(t)
	a, b := uuid.New(), uuid.New()
	insertRisk(t, db, a, seed{score: 9, created: statsNow})
	for i := 0; i < 5; i++ {
		insertRisk(t, db, b, seed{score: 9, created: statsNow})
	}

	statsA, err := ComputeDashboardStats(context.Background(), db, a, windowFor(t, "7d"))
	require.NoError(t, err)
	statsB, err := ComputeDashboardStats(context.Background(), db, b, windowFor(t, "7d"))
	require.NoError(t, err)

	assert.Equal(t, int64(1), statsA.TotalRisks)
	assert.Equal(t, int64(5), statsB.TotalRisks)
	assert.Equal(t, int64(1), statsA.RiskTrend.Points[len(statsA.RiskTrend.Points)-1].CumulativeTotal)
	assert.Equal(t, int64(5), statsB.RiskTrend.Points[len(statsB.RiskTrend.Points)-1].CumulativeTotal)
}

// Fail closed. Aggregating every tenant's register because the context was not
// resolved is the one outcome worse than an error.
func TestDashboardStats_RefusesWithoutTenant(t *testing.T) {
	db := newStatsDB(t)
	_, err := ComputeDashboardStats(context.Background(), db, uuid.Nil, windowFor(t, "all"))
	require.Error(t, err)
}
