// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertRiskAt(t *testing.T, db *gorm.DB, tenant uuid.UUID, score float64, at time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO risks (id, tenant_id, status, level, severity, framework, score, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		uuid.NewString(), tenant.String(), "open", "high", "high", "ISO27001", score, at, at,
	).Error)
}

// An empty register has no history: no point may carry an average score of
// 0.0, and the series must be reported as insufficient (#756).
func TestRiskTrends_EmptyTenant_InsufficientData(t *testing.T) {
	db := setupAnalyticsDB(t)
	tenant := uuid.New()
	ctx := context.Background()

	points, err := NewAnalyticsService(db).GetRiskTrends(ctx, tenant, 30)
	require.NoError(t, err)
	require.Len(t, points, 30)
	for _, p := range points {
		require.Nil(t, p.AvgScore, "a day with no risk has no average, never 0.0")
	}
	require.Equal(t, TrendStatusInsufficientData, RiskTrendStatus(points))

	dash, err := NewDashboardDataService(db, nil).GetRiskTrends(ctx, tenant)
	require.NoError(t, err)
	for _, p := range dash {
		require.Nil(t, p.Score, "a day with no risk has no average, never 0.0")
	}
	require.Equal(t, TrendStatusInsufficientData, DashboardTrendStatus(dash))
}

// One risk created today is a single dated point, not a trend.
func TestRiskTrends_SinglePoint_InsufficientData(t *testing.T) {
	db := setupAnalyticsDB(t)
	tenant := uuid.New()
	insertRiskAt(t, db, tenant, 5, time.Now())

	points, err := NewAnalyticsService(db).GetRiskTrends(context.Background(), tenant, 30)
	require.NoError(t, err)
	require.Equal(t, TrendStatusInsufficientData, RiskTrendStatus(points))
	last := points[len(points)-1]
	require.NotNil(t, last.AvgScore)
	require.InDelta(t, 5.0, *last.AvgScore, 0.001)
}

// Three risks created on three different days: the running average is
// checked against a hand calculation.
//
//	day -3: {6}       → count 1, avg 6.0
//	day -2: {6, 9}    → count 2, avg 7.5
//	day -1: {6, 9}    → count 2, avg 7.5
//	today:  {6, 9, 3} → count 3, avg 6.0
func TestRiskTrends_History_ComputedValues(t *testing.T) {
	db := setupAnalyticsDB(t)
	tenant := uuid.New()
	now := time.Now()
	noon := func(daysAgo int) time.Time {
		d := now.AddDate(0, 0, -daysAgo)
		return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, d.Location())
	}
	insertRiskAt(t, db, tenant, 6, noon(3))
	insertRiskAt(t, db, tenant, 9, noon(2))
	insertRiskAt(t, db, tenant, 3, now)
	// Another tenant's history must not leak into the series.
	insertRiskAt(t, db, uuid.New(), 10, noon(3))

	points, err := NewAnalyticsService(db).GetRiskTrends(context.Background(), tenant, 5)
	require.NoError(t, err)
	require.Len(t, points, 5)
	require.Equal(t, TrendStatusOK, RiskTrendStatus(points))

	require.Equal(t, int64(0), points[0].Count) // day -4
	require.Nil(t, points[0].AvgScore)

	want := []struct {
		count int64
		avg   float64
	}{{1, 6.0}, {2, 7.5}, {2, 7.5}, {3, 6.0}}
	for i, w := range want {
		p := points[i+1]
		require.Equal(t, w.count, p.Count, "day %d", i+1)
		require.NotNil(t, p.AvgScore, "day %d", i+1)
		require.InDelta(t, w.avg, *p.AvgScore, 0.001, "day %d", i+1)
	}

	dash, err := NewDashboardDataService(db, nil).GetRiskTrends(context.Background(), tenant)
	require.NoError(t, err)
	require.Equal(t, TrendStatusOK, DashboardTrendStatus(dash))
	last := dash[len(dash)-1]
	require.Equal(t, int64(3), last.Count)
	require.NotNil(t, last.Score)
	require.InDelta(t, 6.0, *last.Score, 0.001)
}
