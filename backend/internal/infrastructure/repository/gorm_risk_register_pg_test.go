// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #913 — register views, summary counts and the score-trend batch, against real
// Postgres: the trend query is array_agg ... FILTER and the views use
// LOWER/COALESCE/NULLS LAST, none of which a sqlite run proves. Everything runs
// in one transaction that is rolled back.
func TestRiskRegister_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Only two tables matter here; on a bare database their foreign keys
		// point at tables this test does not create.
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		// A migrated database already has these; a bare one gets them for the
		// duration of the transaction.
		require.NoError(t, tx.AutoMigrate(&domain.Risk{}, &domain.RiskHistory{}))

		ctx := context.Background()
		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		repo := NewGormRiskRepository(tx)
		repo.clock = func() time.Time { return now }

		tenant, other := uuid.New(), uuid.New()
		me, someoneElse := uuid.New(), uuid.New()
		day := func(n int) *time.Time { d := now.AddDate(0, 0, n); return &d }

		mk := func(tid uuid.UUID, title string, score float64, crit domain.CriticalityLevel, status domain.RiskStatus, review *time.Time, owner *uuid.UUID) *domain.Risk {
			r := &domain.Risk{
				ID: uuid.New(), TenantID: tid, Title: title, Probability: 0.5, Impact: 4,
				Score: score, Criticality: crit, Status: status, NextReviewAt: review, CreatedBy: someoneElse,
			}
			if owner != nil {
				r.OwnerID = owner
			}
			require.NoError(t, tx.Create(r).Error)
			// Create's AfterSave wrote a snapshot dated "now" (the real clock);
			// the test dates its own snapshots.
			require.NoError(t, tx.Exec("DELETE FROM risk_histories WHERE risk_id = ?", r.ID).Error)
			return r
		}
		snap := func(r *domain.Risk, score float64, at time.Time) {
			require.NoError(t, tx.Create(&domain.RiskHistory{ID: uuid.New(), RiskID: r.ID, Score: score, ChangeType: "UPDATE", CreatedAt: at}).Error)
		}

		hot := mk(tenant, "hot-open", 8, domain.CriticalityCriticalNew, domain.RiskOpen, day(10), &me)
		overdue := mk(tenant, "high-overdue", 5, domain.CriticalityHighNew, domain.RiskInProgress, day(-5), nil)
		accepted := mk(tenant, "medium-accepted", 3, domain.CriticalityMediumNew, domain.RiskAccepted, day(45), nil)
		closed := mk(tenant, "critical-closed", 9, domain.CriticalityCriticalNew, domain.RiskClosed, day(5), nil)
		noReview := mk(tenant, "low-no-review", 1, domain.CriticalityLowNew, domain.RiskOpen, nil, nil)
		legacy := mk(tenant, "legacy-upper", 8, domain.CriticalityLevel("CRITICAL"), domain.RiskOpen, nil, nil)
		foreign := mk(other, "other-tenant-critical", 9, domain.CriticalityCriticalNew, domain.RiskOpen, day(1), &me)

		t.Run("views_list_and_count_agree", func(t *testing.T) {
			counts, err := repo.ViewCounts(ctx, tenant, me)
			require.NoError(t, err)

			wantIDs := map[domain.RiskView][]uuid.UUID{
				domain.RiskViewAll:       {hot.ID, overdue.ID, accepted.ID, closed.ID, noReview.ID, legacy.ID},
				domain.RiskViewMine:      {hot.ID},
				domain.RiskViewHot:       {hot.ID, overdue.ID, legacy.ID},
				domain.RiskViewReviewDue: {hot.ID, overdue.ID},
				domain.RiskViewAccepted:  {accepted.ID},
			}
			for view, want := range wantIDs {
				q := domain.NewRiskQuery()
				q.Limit = 100
				q.View = view
				if view == domain.RiskViewMine {
					q.InvolvedUser = &me
				}
				res, err := repo.List(ctx, tenant, q)
				require.NoError(t, err, view)
				got := make([]uuid.UUID, 0, len(res.Data))
				for _, r := range res.Data {
					got = append(got, r.ID)
				}
				assert.ElementsMatch(t, want, got, "list of view %s", view)
				assert.Equal(t, int64(len(want)), counts[view], "count of view %s must equal its list", view)
			}
		})

		t.Run("mine_without_a_user_matches_nothing", func(t *testing.T) {
			q := domain.NewRiskQuery()
			q.View = domain.RiskViewMine
			res, err := repo.List(ctx, tenant, q)
			require.NoError(t, err)
			assert.Empty(t, res.Data, "no authenticated user must fail closed, not list the register")

			counts, err := repo.ViewCounts(ctx, tenant, uuid.Nil)
			require.NoError(t, err)
			assert.Zero(t, counts[domain.RiskViewMine])
		})

		t.Run("criticality_counts_follow_view_and_search", func(t *testing.T) {
			all, err := repo.CriticalityCounts(ctx, tenant, me, domain.RiskViewAll, "")
			require.NoError(t, err)
			assert.Equal(t, map[domain.CriticalityLevel]int64{
				domain.CriticalityCriticalNew: 3, // hot, closed, and the legacy upper-case row
				domain.CriticalityHighNew:     1,
				domain.CriticalityMediumNew:   1,
				domain.CriticalityLowNew:      1,
			}, all)

			inHot, err := repo.CriticalityCounts(ctx, tenant, me, domain.RiskViewHot, "")
			require.NoError(t, err)
			assert.Equal(t, int64(2), inHot[domain.CriticalityCriticalNew], "closed is not hot")
			assert.Equal(t, int64(1), inHot[domain.CriticalityHighNew])
			assert.Zero(t, inHot[domain.CriticalityLowNew])

			searched, err := repo.CriticalityCounts(ctx, tenant, me, domain.RiskViewAll, "overdue")
			require.NoError(t, err)
			assert.Equal(t, int64(1), searched[domain.CriticalityHighNew])
			assert.Zero(t, searched[domain.CriticalityCriticalNew])
		})

		t.Run("active_excludes_closed", func(t *testing.T) {
			n, err := repo.CountActive(ctx, tenant)
			require.NoError(t, err)
			assert.Equal(t, int64(5), n)

			inputs, err := repo.ActiveExposureInputs(ctx, tenant)
			require.NoError(t, err)
			assert.Len(t, inputs, 5)
			for _, in := range inputs {
				assert.NotEmpty(t, in.Criticality, "criticality must be read for the reference ALE")
			}
		})

		t.Run("next_review_at_sorts_nulls_last_both_ways", func(t *testing.T) {
			for _, dir := range []string{"asc", "desc"} {
				q := domain.NewRiskQuery()
				q.Limit = 100
				q.SortBy, q.SortOrder = "next_review_at", dir
				res, err := repo.List(ctx, tenant, q)
				require.NoError(t, err)
				require.Len(t, res.Data, 6)
				assert.Nil(t, res.Data[4].NextReviewAt, "%s: blanks last", dir)
				assert.Nil(t, res.Data[5].NextReviewAt, "%s: blanks last", dir)
				if dir == "asc" {
					assert.Equal(t, overdue.ID, res.Data[0].ID)
				} else {
					assert.Equal(t, accepted.ID, res.Data[0].ID)
				}
			}
		})

		t.Run("trend_baseline_and_tenant_scope", func(t *testing.T) {
			cutoff := domain.TrendBaselineCutoff(now)

			// At-cutoff wins over older and newer snapshots.
			snap(hot, 2, now.AddDate(0, 0, -90))
			snap(hot, 6, cutoff) // exactly at the cutoff counts as "at or before"
			snap(hot, 7.5, now.AddDate(0, 0, -3))
			// Only recent snapshots: the oldest is the baseline.
			snap(overdue, 4, now.AddDate(0, 0, -20))
			snap(overdue, 4.4, now.AddDate(0, 0, -2))
			// No snapshot at all for accepted.
			// A foreign risk's snapshot must be unreachable by id.
			snap(foreign, 1, now.AddDate(0, 0, -50))

			ids := []uuid.UUID{hot.ID, overdue.ID, accepted.ID, foreign.ID}
			got, err := repo.TrendSnapshots(ctx, tenant, ids, cutoff)
			require.NoError(t, err)

			require.Contains(t, got, hot.ID)
			require.NotNil(t, got[hot.ID].AtCutoff)
			assert.InDelta(t, 6.0, *got[hot.ID].AtCutoff, 1e-9)
			assert.InDelta(t, 2.0, got[hot.ID].Oldest, 1e-9)
			assert.InDelta(t, 6.0, got[hot.ID].Baseline(), 1e-9)

			require.Contains(t, got, overdue.ID)
			assert.Nil(t, got[overdue.ID].AtCutoff)
			assert.InDelta(t, 4.0, got[overdue.ID].Baseline(), 1e-9)

			assert.NotContains(t, got, accepted.ID, "no snapshot, no entry")
			assert.NotContains(t, got, foreign.ID, "another tenant's risk id must read as nothing")

			// The other tenant sees its own risk, and not ours.
			theirs, err := repo.TrendSnapshots(ctx, other, ids, cutoff)
			require.NoError(t, err)
			assert.Contains(t, theirs, foreign.ID)
			assert.NotContains(t, theirs, hot.ID)

			_, err = repo.TrendSnapshots(ctx, uuid.Nil, ids, cutoff)
			require.ErrorIs(t, err, domain.ErrUnauthorized)
		})

		t.Run("summary_is_tenant_scoped", func(t *testing.T) {
			counts, err := repo.ViewCounts(ctx, other, me)
			require.NoError(t, err)
			assert.Equal(t, int64(1), counts[domain.RiskViewAll])
			assert.Equal(t, int64(1), counts[domain.RiskViewMine])

			_, err = repo.ViewCounts(ctx, uuid.Nil, me)
			require.ErrorIs(t, err, domain.ErrUnauthorized)
			_, err = repo.CriticalityCounts(ctx, uuid.Nil, me, domain.RiskViewAll, "")
			require.ErrorIs(t, err, domain.ErrUnauthorized)
			_, err = repo.CountActive(ctx, uuid.Nil)
			require.ErrorIs(t, err, domain.ErrUnauthorized)
			_, err = repo.ActiveExposureInputs(ctx, uuid.Nil)
			require.ErrorIs(t, err, domain.ErrUnauthorized)
		})

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
