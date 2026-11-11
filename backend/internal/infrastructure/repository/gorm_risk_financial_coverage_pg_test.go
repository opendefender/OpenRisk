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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #1033 — the financial screen counts quantified risks with the SQL FILTER of
// CountFinancialCoverage; the reports count them with domain.Risk.HasLossFigures.
// Both must give the same number on every combination of loss inputs, or a
// report and the screen disagree on whether the exposure is quantified. Runs
// against a migrated database inside a transaction that is rolled back.
func TestCountFinancialCoverage_MatchesHasLossFigures_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	v := func(f float64) *float64 { return &f }
	inputs := []func(r *domain.Risk){
		func(r *domain.Risk) {},
		func(r *domain.Risk) { r.ARO = v(0.5) },
		func(r *domain.Risk) { r.RemediationCostXAF = v(1_000_000) },
		func(r *domain.Risk) { r.SLEXAF = v(20_000_000); r.ARO = v(0.5) },
		func(r *domain.Risk) { r.SLEXAF = v(20_000_000) },
		func(r *domain.Risk) { r.FinesXAF = v(8_000_000) },
		func(r *domain.Risk) { r.DataLossCostXAF = v(1_000) },
		func(r *domain.Risk) { r.OtherDirectCostXAF = v(1_000) },
		func(r *domain.Risk) { r.DowntimeHours = v(4) },
		func(r *domain.Risk) { r.DowntimeHours = v(4); r.HourlyDowntimeCostXAF = v(100_000) },
		func(r *domain.Risk) { r.SLEXAF = v(0) },
	}

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		repo := NewGormRiskRepository(tx)
		tenant, other := uuid.New(), uuid.New()

		want := 0
		for i, set := range inputs {
			r := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "pg-1033-" + uuid.NewString()[:8],
				Probability: 0.5, Impact: 2, Score: 1, Criticality: "medium"}
			r.SetState(domain.StateDraft)
			set(r)
			if r.HasLossFigures() {
				want++
			}
			require.NoError(t, tx.Create(r).Error, "input %d", i)
		}
		// A quantified risk of another tenant must not move this tenant's count.
		foreign := &domain.Risk{ID: uuid.New(), TenantID: other, Title: "pg-1033-foreign",
			Probability: 0.5, Impact: 2, Score: 1, Criticality: "medium", SLEXAF: v(1)}
		foreign.SetState(domain.StateDraft)
		require.NoError(t, tx.Create(foreign).Error)

		total, quantified, err := repo.CountFinancialCoverage(ctx, tenant)
		require.NoError(t, err)
		assert.Equal(t, len(inputs), total)
		assert.Equal(t, want, quantified, "SQL FILTER and domain.Risk.HasLossFigures disagree")

		// And the Go rule over the rows as the reports load them.
		listed, err := repo.ListRisksForFinancial(ctx, tenant)
		require.NoError(t, err)
		goCount := 0
		for i := range listed {
			if listed[i].HasLossFigures() {
				goCount++
			}
		}
		assert.Equal(t, quantified, goCount)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
