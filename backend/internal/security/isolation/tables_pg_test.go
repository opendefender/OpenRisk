// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package isolation

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlexception"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// TestTenantTables_AreTheTenantOwnedTables keeps the list and the row types in
// step: a seventh table of the chain, or a second exception table, cannot be added
// without being listed. The chain is the six tables of migration 0086; the seventh
// entry is control_exceptions (migration 0087), which is not a chain table, and the
// eighth is control_status_evaluations (migration 0089, #555), the engine's cache.
// The last three are the compliance catalogue (#1490), named from their models.
func TestTenantTables_AreTheTenantOwnedTables(t *testing.T) {
	want := append(controlchain.Tables(), controlexception.TableControlExceptions, controlstatus.TableControlStatusEvaluations,
		domain.ComplianceFramework{}.TableName(), domain.ComplianceControl{}.TableName(), domain.ControlCrosswalk{}.TableName())
	got := append([]string(nil), TenantTables...)
	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got)
	assert.Len(t, got, 11)
}

// TestTenantTables_Postgres asserts, on a migrated PostgreSQL database, that
// every listed table has `tenant_id uuid NOT NULL` and an index that leads with
// it. Skipped without DATABASE_URL.
func TestTenantTables_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := testdb.Open(t, postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var present bool
	require.NoError(t, db.Raw(`SELECT to_regclass('controls') IS NOT NULL`).Scan(&present).Error)
	if !present {
		t.Skip("controls missing: point DATABASE_URL at a migrated database")
	}

	for _, table := range TenantTables {
		t.Run(table, func(t *testing.T) {
			var col struct {
				DataType   string
				IsNullable string
			}
			// tenant: information_schema describes the table's own shape, it reads no row.
			require.NoError(t, db.Raw(`SELECT data_type, is_nullable FROM information_schema.columns
				WHERE table_schema = current_schema() AND table_name = ? AND column_name = 'tenant_id'`, table).Scan(&col).Error)
			assert.Equal(t, "uuid", col.DataType, "%s.tenant_id", table)
			assert.Equal(t, "NO", col.IsNullable, "%s.tenant_id must be NOT NULL", table)

			var leading int64
			// tenant: pg_index describes the table's own indexes, it reads no row.
			require.NoError(t, db.Raw(`SELECT count(*) FROM pg_index i
				JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = i.indkey[0]
				WHERE i.indrelid = to_regclass(?) AND a.attname = 'tenant_id'`, table).Scan(&leading).Error)
			assert.GreaterOrEqual(t, leading, int64(1), "%s needs an index that leads with tenant_id", table)
		})
	}
}
