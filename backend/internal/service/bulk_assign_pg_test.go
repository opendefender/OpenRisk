// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
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
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// #1213 — the bulk assign path against the real schema: the plans land with
// the job's tenant and author, through the repository's tenant-checked create,
// and a re-run adds nothing. Runs against a migrated database; everything
// happens in an outer transaction that is rolled back.
func TestBulkAssign_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		s := &BulkOperationService{db: tx, mitigations: repository.NewGormMitigationRepository(tx)}
		tenant, other := uuid.New(), uuid.New()
		author := uuid.New()

		risk := func(tid uuid.UUID) uuid.UUID {
			r := &domain.Risk{ID: uuid.New(), TenantID: tid, Title: "pg-1213-" + uuid.NewString()[:8], Probability: 0.5, Impact: 2, Score: 1}
			r.SetState(domain.StateDraft)
			require.NoError(t, tx.Create(r).Error)
			return r.ID
		}
		templateRisk, r2, r3 := risk(tenant), risk(tenant), risk(tenant)
		foreignRisk := risk(other)

		tpl := &domain.Mitigation{
			ID: uuid.New(), TenantID: tenant, RiskID: templateRisk,
			Title: "pg-1213 rotate credentials", Description: "Rotate every leaked key",
			Priority: domain.MitigationPriority("high"), Status: domain.MitigationPlanned,
			CreatedBy: author,
		}
		require.NoError(t, tx.Create(tpl).Error)

		run := func() *domain.BulkOperation {
			op := &domain.BulkOperation{
				ID: uuid.New(), OperationType: domain.BulkOperationTypeAssign,
				Status:     domain.BulkOperationStatusPending,
				UpdateData: map[string]interface{}{"mitigation_id": tpl.ID.String()},
				TenantID:   tenant, CreatedBy: author, CreatedAt: time.Now(),
			}
			require.NoError(t, tx.Create(op).Error)
			require.NoError(t, s.processBulkAssign(op))
			return op
		}

		type planRow struct {
			RiskID    uuid.UUID
			TenantID  uuid.UUID
			CreatedBy uuid.UUID
			OwnerID   *uuid.UUID
			Title     string
			Priority  string
		}
		plans := func() []planRow {
			var rows []planRow
			require.NoError(t, tx.Table("mitigations").
				Select("risk_id, tenant_id, created_by, owner_id, title, priority").
				Where("tenant_id = ? AND title = ? AND id <> ? AND deleted_at IS NULL", tenant, tpl.Title, tpl.ID).
				Scan(&rows).Error)
			return rows
		}

		first := run()
		assert.Equal(t, 3, first.ProcessedCount)
		assert.Equal(t, 0, first.ErrorCount)
		got := plans()
		require.Len(t, got, 2, "one copy per risk that does not already hold the plan")
		var risks []uuid.UUID
		for _, p := range got {
			assert.Equal(t, tenant, p.TenantID)
			assert.Equal(t, author, p.CreatedBy)
			require.NotNil(t, p.OwnerID)
			assert.Equal(t, author, *p.OwnerID)
			assert.Equal(t, "high", p.Priority)
			risks = append(risks, p.RiskID)
		}
		assert.ElementsMatch(t, []uuid.UUID{r2, r3}, risks)

		var zeroTenant int64
		require.NoError(t, tx.Table("mitigations").
			Where("tenant_id = ? AND risk_id IN ?", uuid.Nil, []uuid.UUID{templateRisk, r2, r3, foreignRisk}).
			Count(&zeroTenant).Error)
		assert.Zero(t, zeroTenant, "no plan is written without a tenant")

		var foreign int64
		require.NoError(t, tx.Table("mitigations").
			Where("tenant_id = ? AND risk_id = ?", other, foreignRisk).Count(&foreign).Error)
		assert.Zero(t, foreign, "the other tenant's risk is untouched")

		second := run()
		assert.Equal(t, 3, second.ProcessedCount)
		assert.Len(t, plans(), 2, "a re-run writes nothing new")

		// Read the counter column alone: scanning the whole row trips on the
		// jsonb maps, a separate defect outside #1213.
		var processed []int
		require.NoError(t, tx.Table("bulk_operations").
			Where("id = ? AND tenant_id = ?", second.ID, tenant).Pluck("processed_count", &processed).Error)
		assert.Equal(t, []int{3}, processed, "the counter reaches the row")

		// A template from another tenant is refused, nothing written.
		foreignTpl := &domain.Mitigation{
			ID: uuid.New(), TenantID: other, RiskID: foreignRisk, Title: "pg-1213 foreign",
			Status: domain.MitigationPlanned, CreatedBy: uuid.New(),
		}
		require.NoError(t, tx.Create(foreignTpl).Error)
		_, err := s.assignTemplate(tenant, foreignTpl.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
