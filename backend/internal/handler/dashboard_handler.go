// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain/timeframe"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
)

// MatrixCell is one cell of the 5x5 probability x impact heatmap, counted in SQL.
// Probability and Impact are 1-5 display bands, not the stored values: the Score
// Engine works on probability [0,1] and impact [0,10], and the banding is done in
// the query so every consumer bands identically.
type MatrixCell struct {
	Probability int   `json:"probability"`
	Impact      int   `json:"impact"`
	Count       int64 `json:"count"`
}

// RiskTrendPoint is one bucket of the risk-trend series.
//
// Read the field names literally, because the widget that plots them says the
// same words. `Opened` is a FLOW — risks whose created_at falls inside this
// bucket. `CumulativeTotal` is a STOCK — every risk that existed at the bucket's
// end. They answer different questions and the chart labels them separately.
//
// OpenedByBand splits the flow using each risk's CURRENT criticality band, not
// the band it carried on the day it was opened. The register does not version
// criticality per day — risk_histories records score changes as they happen, not
// a daily snapshot — so a per-day band would have to be reconstructed and would
// be wrong for every risk created before that log existed. The contract says
// "current band" and the UI repeats it; inventing a per-day band would be a
// number nobody could reconcile against anything.
type RiskTrendPoint struct {
	// Bucket is the bucket's inclusive start, as a UTC calendar date.
	Bucket string `json:"bucket"`
	// Opened counts risks created inside [bucket, next bucket).
	Opened int64 `json:"opened"`
	// OpenedByBand splits Opened by the risk's current band. The four keys are
	// always present, so a zero is stated rather than left for the client to
	// infer from a missing key.
	OpenedByBand map[string]int64 `json:"opened_by_band"`
	// CumulativeTotal is every non-deleted risk that existed at the end of this
	// bucket, including those created before the window began.
	CumulativeTotal int64 `json:"cumulative_total"`
}

// RiskTrend is the series plus the bounds it was actually computed over.
//
// From/To are echoed because they are NOT always the window that was requested:
// an unbounded window counts everything but its series is capped (see
// timeframe.TrendCap), and a chart labelled "all time" that silently starts a
// year ago is the kind of quiet substitution this wave removes.
type RiskTrend struct {
	From        string           `json:"from"`
	To          string           `json:"to"`
	Granularity string           `json:"granularity"`
	Points      []RiskTrendPoint `json:"points"`
}

// DashboardStats is the tenant's headline posture. Every field is a server-side
// aggregate over the whole register, never a count over a page of results: the
// register is paginated and a client-side .filter().length silently reports the
// current page as if it were the tenant.
//
// NOTE ON WHAT IS *NOT* HERE. This type used to carry `global_risk_score`,
// computed inline as `100 - avg(risks.score) * 4`. That was a second security
// score, on the same 0-100 scale as the canonical model served by GET /score,
// derived from the register alone — no controls, no vulnerabilities, no
// incidents, and no accounting for a source that could not be read. Two personas
// of the same product therefore showed two different "security scores" for the
// same tenant, and they disagreed by more the more compliance data a tenant had.
// It is removed rather than deprecated: a competing formula left in a payload is
// a competing formula that the next widget picks up because it was already there.
type DashboardStats struct {
	// Period is the window this response was computed for, echoed back so the
	// numbers can be reconciled against anything at all.
	Period timeframe.Resolved `json:"period"`
	// PeriodAppliesTo names the fields the window actually narrowed. Everything
	// else is a point-in-time stock, counted in full — see the comment on
	// OpenedInPeriod.
	PeriodAppliesTo []string `json:"period_applies_to"`

	TotalRisks      int64            `json:"total_risks"`
	HighRisks       int64            `json:"high_risks"`
	MitigatedRisks  int64            `json:"mitigated_risks"`
	InProgressRisks int64            `json:"in_progress_risks"`
	QuantifiedRisks int64            `json:"quantified_risks"` // risks carrying financial drivers
	RisksBySeverity map[string]int64 `json:"risks_by_severity"`
	RiskMatrix      []MatrixCell     `json:"risk_matrix"`

	// OpenedInPeriod is the one period-scoped counter: risks created inside the
	// window. The stock counters above are deliberately NOT narrowed by it.
	//
	// "How many critical risks do we have" does not become a different question
	// when someone picks a date range. Narrowing it by created_at would answer
	// "how many did we OPEN last month", print it under a label saying
	// otherwise, and put the dashboard permanently at odds with the register it
	// links to — which is precisely the reconciliation this wave has to hold.
	OpenedInPeriod int64 `json:"opened_in_period"`

	RiskTrend RiskTrend `json:"risk_trend"`

	GeneratedAt string `json:"generated_at"`
}

// scalarAggregates is the destination of the single-row counters query.
type scalarAggregates struct {
	Total      int64
	High       int64
	Mitigated  int64
	InProgress int64
	Quantified int64
	Critical   int64
	HighBand   int64
	Medium     int64
	Low        int64
}

// GetDashboardStats returns the tenant's posture over the requested window.
//
// Multi-tenancy (RULE #2): fail closed. Without a resolved tenant we must NOT
// fall back to querying every tenant's risks.
func GetDashboardStats(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}
	window, err := parsePeriod(c)
	if err != nil {
		return writePeriodError(c, err)
	}

	stats, err := ComputeDashboardStats(c.UserContext(), database.DB, ctx.OrganizationID, window)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

// ComputeDashboardStats is the whole aggregation, separated from the HTTP layer
// so the tests exercise THIS code rather than a copy of its SQL.
//
// That is not tidiness. The previous test re-typed the handler's query into its
// own body and asserted against the copy, which passes just as happily when the
// handler's query drifts away from it — the same failure mode as the hand-written
// sqlite DDL that has bitten this repository twice.
func ComputeDashboardStats(ctx context.Context, db *gorm.DB, tenantID uuid.UUID, window timeframe.Window) (*DashboardStats, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant is required")
	}
	tx := db.WithContext(ctx)

	// --- stock counters, one row -------------------------------------------
	//
	// Grouped/filtered in SQL rather than by loading the register into memory and
	// looping: a large register otherwise pays a full scan AND a full
	// deserialisation for four numbers.
	//
	// Status matching is case-insensitive because the codebase carries two
	// RiskStatus vocabularies ("mitigated" and legacy "MITIGATED"); matching only
	// one of them is how mitigated risks came to be undercounted.
	var agg scalarAggregates
	err := tx.Raw(`
		SELECT
			COUNT(*)                                                        AS total,
			COUNT(*) FILTER (WHERE score >= 4.0)                            AS high,
			COUNT(*) FILTER (WHERE LOWER(status) = 'mitigated')             AS mitigated,
			COUNT(*) FILTER (WHERE LOWER(status) IN ('in_progress','active')) AS in_progress,
			COUNT(*) FILTER (WHERE
				COALESCE(slexaf, 0) > 0
				OR COALESCE(data_loss_cost_xaf, 0) > 0
				OR COALESCE(fines_xaf, 0) > 0
				OR COALESCE(other_direct_cost_xaf, 0) > 0
				OR (COALESCE(downtime_hours, 0) > 0 AND COALESCE(hourly_downtime_cost_xaf, 0) > 0)
			)                                                               AS quantified,
			-- Severity bands MUST match the Score Engine (score = P x I x AC),
			-- the same thresholds the register's criticality column is derived
			-- from: >=7 critical, >=4 high, >=2 medium, <2 low. The previous
			-- 20/15/10 thresholds assumed a 1-5 x 1-5 scale and reported 0
			-- critical / 0 high on a register the register itself showed as
			-- critical. Score is always set and the bands partition fully, so
			-- critical+high+medium+low = total.
			COUNT(*) FILTER (WHERE score >= 7.0)                            AS critical,
			COUNT(*) FILTER (WHERE score >= 4.0 AND score < 7.0)           AS high_band,
			COUNT(*) FILTER (WHERE score >= 2.0 AND score < 4.0)           AS medium,
			COUNT(*) FILTER (WHERE score < 2.0)                            AS low
		FROM risks
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID).Scan(&agg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate risk statistics: %w", err)
	}

	stats := &DashboardStats{
		Period: window.Resolved(),
		// The two period-scoped fields, named so the client can render the
		// period control as applying to those and no others.
		PeriodAppliesTo: []string{"opened_in_period", "risk_trend"},
		TotalRisks:      agg.Total,
		HighRisks:       agg.High,
		MitigatedRisks:  agg.Mitigated,
		InProgressRisks: agg.InProgress,
		QuantifiedRisks: agg.Quantified,
		RisksBySeverity: map[string]int64{
			"CRITICAL": agg.Critical,
			"HIGH":     agg.HighBand,
			"MEDIUM":   agg.Medium,
			"LOW":      agg.Low,
		},
		RiskMatrix:  []MatrixCell{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// --- 5x5 heatmap --------------------------------------------------------
	// probability [0,1] -> band 1-5, impact [0,10] -> band 1-5. A risk at exactly
	// 0 still lands in band 1 rather than vanishing from the matrix.
	// The CASE ladders are exactly CEIL(probability*5) and CEIL(impact/2.0),
	// clamped to [1,5] — written out rather than as LEAST/GREATEST/CEIL/::int
	// because those are Postgres-only and a matrix query the sqlite suite cannot
	// run is a matrix query that ships unverified.
	var cells []MatrixCell
	err = tx.Raw(`
		SELECT
			CASE
				WHEN probability > 0.8 THEN 5
				WHEN probability > 0.6 THEN 4
				WHEN probability > 0.4 THEN 3
				WHEN probability > 0.2 THEN 2
				ELSE 1
			END AS probability,
			CASE
				WHEN impact > 8 THEN 5
				WHEN impact > 6 THEN 4
				WHEN impact > 4 THEN 3
				WHEN impact > 2 THEN 2
				ELSE 1
			END AS impact,
			COUNT(*) AS count
		FROM risks
		WHERE tenant_id = ? AND deleted_at IS NULL
		GROUP BY 1, 2
	`, tenantID).Scan(&cells).Error
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate risk matrix: %w", err)
	}
	if cells != nil {
		stats.RiskMatrix = cells
	}

	// --- the period-scoped flow --------------------------------------------
	trend, opened, err := riskTrend(tx, tenantID, window)
	if err != nil {
		return nil, err
	}
	stats.RiskTrend = trend
	stats.OpenedInPeriod = opened
	return stats, nil
}

// bandOfScore maps a score to the Score Engine's band. One function, used by the
// trend, so the series cannot band differently from the counters above it.
func bandOfScore(score float64) string {
	switch {
	case score >= 7.0:
		return "CRITICAL"
	case score >= 4.0:
		return "HIGH"
	case score >= 2.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// riskTrend builds the series and the period-scoped opened count.
//
// The bucketing is done in Go over a projected two-column scan rather than with
// date_trunc/to_char in SQL. Two reasons, in this order:
//
//  1. Correctness under test. Those functions are Postgres-only, so a SQL-side
//     bucketing could not be exercised by the sqlite suite and would ship
//     unverified — which is exactly how MonthlyRiskTrend's to_char query got to
//     production without a test.
//  2. Cost. The projection is (timestamp, float) per risk in ONE indexed scan of
//     the same table the counters above already scan, with no join and no
//     preload. It is bounded by the register, which the register screen pages
//     precisely because it can be large — so if this ever becomes the bottleneck
//     the fix is a materialised daily rollup, not a per-bucket query.
func riskTrend(tx *gorm.DB, tenantID uuid.UUID, window timeframe.Window) (RiskTrend, int64, error) {
	from, to, granularity := window.TrendBounds()

	type row struct {
		CreatedAt time.Time
		Score     float64
	}
	var rows []row
	// Everything created before `to`: rows before `from` do not appear as their
	// own bucket but DO count towards the cumulative stock, which is the whole
	// point of a cumulative line. Filtering them out here is how a trend chart
	// comes to start at zero for a tenant that has had risks for two years.
	if err := tx.Table("risks").
		Select("created_at, score").
		Where("tenant_id = ? AND deleted_at IS NULL AND created_at < ?", tenantID, to).
		Scan(&rows).Error; err != nil {
		return RiskTrend{}, 0, fmt.Errorf("failed to read risk trend: %w", err)
	}

	buckets := bucketStarts(from, to, granularity)
	trend := RiskTrend{
		From:        from.Format(time.RFC3339),
		To:          to.Format(time.RFC3339),
		Granularity: granularity,
		Points:      make([]RiskTrendPoint, 0, len(buckets)),
	}

	openedPerBucket := make([]int64, len(buckets))
	bandPerBucket := make([]map[string]int64, len(buckets))
	for i := range bandPerBucket {
		bandPerBucket[i] = map[string]int64{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}
	}
	// Everything created strictly before the first bucket: the baseline the
	// cumulative line starts from.
	var baseline int64
	var openedInWindow int64

	for _, r := range rows {
		created := r.CreatedAt.UTC()
		if created.Before(from) {
			baseline++
			continue
		}
		i := bucketIndex(buckets, created)
		if i < 0 {
			continue // created >= to; excluded by the query, kept for safety
		}
		openedPerBucket[i]++
		bandPerBucket[i][bandOfScore(r.Score)]++
	}

	cumulative := baseline
	for i, b := range buckets {
		cumulative += openedPerBucket[i]
		openedInWindow += openedPerBucket[i]
		trend.Points = append(trend.Points, RiskTrendPoint{
			Bucket:          b.Format("2006-01-02"),
			Opened:          openedPerBucket[i],
			OpenedByBand:    bandPerBucket[i],
			CumulativeTotal: cumulative,
		})
	}

	// OpenedInPeriod follows the REQUESTED window, not the capped trend bounds:
	// on an unbounded window it is every risk ever opened, which is what the
	// label says.
	opened := openedInWindow
	if window.IsAll() {
		opened = baseline + openedInWindow
	}
	return trend, opened, nil
}

// bucketStarts returns the inclusive start of every bucket in [from, to).
func bucketStarts(from, to time.Time, granularity string) []time.Time {
	step := 1
	if granularity == "week" {
		step = 7
	}
	start := truncateUTCDay(from)
	if granularity == "week" {
		// Anchor weeks on Monday so a bucket label is a real week, and two
		// requests a day apart do not shift every point sideways.
		offset := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -offset)
	}
	out := []time.Time{}
	for b := start; b.Before(to); b = b.AddDate(0, 0, step) {
		out = append(out, b)
	}
	return out
}

// bucketIndex finds the bucket owning an instant, using the half-open rule:
// a point belongs to the last bucket whose start is <= it.
func bucketIndex(buckets []time.Time, t time.Time) int {
	if len(buckets) == 0 || t.Before(buckets[0]) {
		return -1
	}
	i := sort.Search(len(buckets), func(i int) bool { return buckets[i].After(t) })
	return i - 1
}

func truncateUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
