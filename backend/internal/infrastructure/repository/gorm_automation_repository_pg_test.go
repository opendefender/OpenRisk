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
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// TestAutomationRuleCreate_KeepsADisabledRuleDisabled_Postgres pins the fix #783
// found live: the schema defaults enabled to true and GORM omits a false bool
// from the INSERT, so a draft or an adopted template was stored active and
// fired. Kept when the plan gates were reverted (#1159).
func TestAutomationRuleCreate_KeepsADisabledRuleDisabled_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE automation_rules (LIKE public.automation_rules INCLUDING ALL) ON COMMIT DROP`).Error)
		rules := NewGormAutomationRuleRepository(tx)
		ctx := context.Background()
		tenant := uuid.New()

		draft := &domain.AutomationRule{TenantID: tenant, Name: "draft", Trigger: domain.TriggerVulnerabilityDetected, Enabled: false}
		live := &domain.AutomationRule{TenantID: tenant, Name: "live", Trigger: domain.TriggerVulnerabilityDetected, Enabled: true}
		require.NoError(t, rules.Create(ctx, draft))
		require.NoError(t, rules.Create(ctx, live))

		got, err := rules.GetByID(ctx, draft.ID, tenant)
		require.NoError(t, err)
		require.False(t, got.Enabled, "a rule created disabled must be stored disabled")

		enabled, err := rules.ListEnabledByTrigger(ctx, tenant, domain.TriggerVulnerabilityDetected)
		require.NoError(t, err)
		require.Len(t, enabled, 1)
		require.Equal(t, live.ID, enabled[0].ID, "only the rule created enabled may fire")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
