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

// #792 — the risk row carries a score computed from its asset links, so the
// row and the links must be written together. Runs against a migrated
// database; everything happens in an outer transaction that is rolled back.
func TestGormRiskAssetStore_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		store := NewGormRiskAssetStore(tx)
		tenant, other := uuid.New(), uuid.New()

		asset := func(tid uuid.UUID, c domain.AssetCriticality) *domain.Asset {
			a := &domain.Asset{ID: uuid.New(), TenantID: tid, Name: "a-" + uuid.NewString()[:8], Type: "server", Criticality: c}
			require.NoError(t, tx.Create(a).Error)
			return a
		}
		links := func(riskID uuid.UUID) []uuid.UUID {
			var ids []uuid.UUID
			// risk_assets has no tenant_id; riskID is a risk this test created under tenant.
			require.NoError(t, tx.Table("risk_assets").Where("risk_id = ?", riskID).Order("asset_id").Pluck("asset_id", &ids).Error)
			return ids
		}
		riskCount := func(id uuid.UUID) int64 {
			var n int64
			require.NoError(t, tx.Model(&domain.Risk{}).Where("id = ? AND tenant_id = ?", id, tenant).Count(&n).Error)
			return n
		}

		crit, low, high := asset(tenant, domain.CriticalityCritical), asset(tenant, domain.CriticalityLow), asset(tenant, domain.CriticalityHigh)
		foreign := asset(other, domain.CriticalityCritical)

		// FindByIDs is tenant-scoped: the other tenant's asset is absent.
		found, err := store.FindByIDs(ctx, tenant, []uuid.UUID{crit.ID, foreign.ID})
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, crit.ID, found[0].ID)

		// Create writes the row and the links.
		r := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "pg-792", Probability: 0.5, Impact: 2, Score: 3}
		r.SetState(domain.StateDraft)
		require.NoError(t, store.SaveWithAssets(ctx, r, []*domain.Asset{crit}, true))
		assert.Equal(t, int64(1), riskCount(r.ID))
		assert.Equal(t, []uuid.UUID{crit.ID}, links(r.ID))
		var stored float64
		require.NoError(t, tx.Table("risks").Where("id = ? AND tenant_id = ?", r.ID, tenant).Pluck("score", &stored).Error)
		assert.InDelta(t, 3.0, stored, 1e-9)

		// Save replaces: the critical asset is unlinked, low and high are linked.
		r.Score = 0.5 * 2 * 1.5
		require.NoError(t, store.SaveWithAssets(ctx, r, []*domain.Asset{low, high}, false))
		got := links(r.ID)
		assert.ElementsMatch(t, []uuid.UUID{low.ID, high.ID}, got)

		// Another tenant's asset is refused before anything is written.
		r2 := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "pg-792-foreign", Probability: 0.5, Impact: 2}
		r2.SetState(domain.StateDraft)
		require.Error(t, store.SaveWithAssets(ctx, r2, []*domain.Asset{foreign}, true))
		assert.Zero(t, riskCount(r2.ID))

		// A link write Postgres refuses rolls the risk row back with it.
		for _, sql := range []string{
			`CREATE FUNCTION pg_temp.refuse_link() RETURNS trigger LANGUAGE plpgsql AS
			   $$ BEGIN RAISE EXCEPTION 'link refused'; END $$`,
			`CREATE TRIGGER refuse_link_792 BEFORE INSERT ON risk_assets
			   FOR EACH ROW EXECUTE FUNCTION pg_temp.refuse_link()`,
		} {
			require.NoError(t, tx.Exec(sql).Error, sql)
		}
		r3 := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "pg-792-atomic", Probability: 0.5, Impact: 2, Score: 3}
		r3.SetState(domain.StateDraft)
		require.Error(t, store.SaveWithAssets(ctx, r3, []*domain.Asset{crit}, true))
		assert.Zero(t, riskCount(r3.ID), "the risk row must not survive a failed link")

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
