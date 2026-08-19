// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDashboardSeverity_MatchesScoreEngineBands pins /stats severity bucketing to
// the Score Engine bands (score = P x I x AC): >=7 critical, >=4 high, >=2
// medium, <2 low — the same thresholds the register's `criticality` column is
// derived from. The previous 20/15/10 thresholds assumed a 1-5 x 1-5 scale and
// reported 0 critical / 0 high on a register the register itself showed as
// critical, contradicting /analytics/executive (audit-2026 #243).
func TestDashboardSeverity_MatchesScoreEngineBands(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`CREATE TABLE risks (id TEXT PRIMARY KEY, tenant_id TEXT, score REAL, deleted_at DATETIME);`).Error)

	tenant := uuid.New()
	other := uuid.New()

	// One risk per band plus the exact boundary values, in this tenant.
	scores := []float64{
		10.0, // critical
		7.0,  // critical (>= 7 boundary)
		6.999, // high  (< 7)
		4.0,  // high  (>= 4 boundary)
		3.999, // medium (< 4)
		2.0,  // medium (>= 2 boundary)
		1.999, // low   (< 2)
		0.1,  // low
	}
	for _, s := range scores {
		require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`,
			uuid.NewString(), tenant.String(), s).Error)
	}
	// A high-scoring risk in ANOTHER tenant must not be counted.
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, score) VALUES (?,?,?)`,
		uuid.NewString(), other.String(), 30.0).Error)

	var agg scalarAggregates
	require.NoError(t, db.Raw(`
		SELECT
			COUNT(*)                                              AS total,
			COUNT(*) FILTER (WHERE score >= 4.0)                  AS high,
			COUNT(*) FILTER (WHERE score >= 7.0)                  AS critical,
			COUNT(*) FILTER (WHERE score >= 4.0 AND score < 7.0)  AS high_band,
			COUNT(*) FILTER (WHERE score >= 2.0 AND score < 4.0)  AS medium,
			COUNT(*) FILTER (WHERE score < 2.0)                   AS low
		FROM risks
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenant.String()).Scan(&agg).Error)

	require.Equal(t, int64(8), agg.Total, "only this tenant's risks")
	require.Equal(t, int64(2), agg.Critical, "score 10 and 7.0")
	require.Equal(t, int64(2), agg.HighBand, "score 6.999 and 4.0")
	require.Equal(t, int64(2), agg.Medium, "score 3.999 and 2.0")
	require.Equal(t, int64(2), agg.Low, "score 1.999 and 0.1")
	require.Equal(t, int64(4), agg.High, "high-or-above KPI = high + critical")

	// The bands must partition the register fully: no risk is uncounted or
	// double-counted, so the dashboard total always reconciles.
	require.Equal(t, agg.Total, agg.Critical+agg.HighBand+agg.Medium+agg.Low)
}
