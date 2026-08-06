// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

// DashboardStats is the tenant's headline posture. Every field is a server-side
// aggregate over the whole register, never a count over a page of results: the
// dashboard is paginated and a client-side .filter().length silently reports the
// current page as if it were the tenant.
type DashboardStats struct {
	TotalRisks      int64            `json:"total_risks"`
	GlobalRiskScore int              `json:"global_risk_score"` // 100 = safe, 0 = danger
	HighRisks       int64            `json:"high_risks"`
	MitigatedRisks  int64            `json:"mitigated_risks"`
	InProgressRisks int64            `json:"in_progress_risks"`
	QuantifiedRisks int64            `json:"quantified_risks"` // risks carrying financial drivers
	RisksBySeverity map[string]int64 `json:"risks_by_severity"`
	RiskMatrix      []MatrixCell     `json:"risk_matrix"`
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
	AvgScore   float64
}

// GetDashboardStats returns the tenant's posture in two aggregate queries.
//
// Both are grouped/filtered in SQL rather than by loading the register into memory
// and looping: a tenant with a large register otherwise pays a full table scan and
// a full deserialisation for four numbers.
//
// Status matching is case-insensitive because the codebase carries two RiskStatus
// vocabularies ("mitigated" and legacy "MITIGATED"); matching only one of them is
// how mitigated risks came to be undercounted.
func GetDashboardStats(c *fiber.Ctx) error {
	// Multi-tenancy (RULE #2). Fail closed: without a resolved tenant we must NOT
	// fall back to querying every tenant's risks.
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}
	tenantID := ctx.OrganizationID

	var agg scalarAggregates
	err := database.DB.Raw(`
		SELECT
			COUNT(*)                                                        AS total,
			COUNT(*) FILTER (WHERE score >= 15.0)                           AS high,
			COUNT(*) FILTER (WHERE LOWER(status) = 'mitigated')             AS mitigated,
			COUNT(*) FILTER (WHERE LOWER(status) IN ('in_progress','active')) AS in_progress,
			COUNT(*) FILTER (WHERE
				COALESCE(slexaf, 0) > 0
				OR COALESCE(data_loss_cost_xaf, 0) > 0
				OR COALESCE(fines_xaf, 0) > 0
				OR COALESCE(other_direct_cost_xaf, 0) > 0
				OR (COALESCE(downtime_hours, 0) > 0 AND COALESCE(hourly_downtime_cost_xaf, 0) > 0)
			)                                                               AS quantified,
			COUNT(*) FILTER (WHERE score >= 20.0)                           AS critical,
			COUNT(*) FILTER (WHERE score >= 15.0 AND score < 20.0)          AS high_band,
			COUNT(*) FILTER (WHERE score >= 10.0 AND score < 15.0)          AS medium,
			COUNT(*) FILTER (WHERE score < 10.0)                            AS low,
			COALESCE(AVG(score), 0)                                         AS avg_score
		FROM risks
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID).Scan(&agg).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to aggregate risk statistics"})
	}

	stats := DashboardStats{
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
		RiskMatrix: []MatrixCell{},
	}

	// Global security score: 100 = safe. A register with no risks scores 100, which
	// is the honest reading of "nothing recorded" for a score whose only input is
	// the register itself.
	stats.GlobalRiskScore = 100
	if agg.Total > 0 {
		securityScore := 100 - int(agg.AvgScore*4)
		if securityScore < 0 {
			securityScore = 0
		}
		stats.GlobalRiskScore = securityScore
	}

	// 5x5 heatmap. probability [0,1] -> band 1-5, impact [0,10] -> band 1-5. A risk
	// at exactly 0 still lands in band 1 rather than vanishing from the matrix.
	var cells []MatrixCell
	err = database.DB.Raw(`
		SELECT
			LEAST(5, GREATEST(1, CEIL(probability * 5)))::int AS probability,
			LEAST(5, GREATEST(1, CEIL(impact / 2.0)))::int    AS impact,
			COUNT(*)                                          AS count
		FROM risks
		WHERE tenant_id = ? AND deleted_at IS NULL
		GROUP BY 1, 2
	`, tenantID).Scan(&cells).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to aggregate risk matrix"})
	}
	if cells != nil {
		stats.RiskMatrix = cells
	}

	return c.JSON(stats)
}
