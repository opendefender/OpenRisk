// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

//go:build integration

package main

// #706 against real PostgreSQL: build the schema exactly as the server does at
// startup (schemaModels(), nothing else), then run the sub-action queries the
// transitions route depends on. SQLite accepts things PostgreSQL does not, so
// the JOIN-scoped repository queries are exercised here too.
//
// Runs in the CI "Backend Integration Tests" job (DATABASE_URL, -tags=integration).

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// startupSchemaDB opens DATABASE_URL on a throwaway PostgreSQL schema and builds
// it with schemaModels(), so nothing another test created can make a missing
// table look present.
func startupSchemaDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	schema := "schema_706_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, admin.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)).Error)
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema)).Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	// The options database.Connect uses (internal/infrastructure/database/
	// database.go). DisableForeignKeyConstraintWhenMigrating is load-bearing:
	// without it AutoMigrate cannot order risks and mitigations on an empty
	// database and fails before reaching the model under test.
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		TranslateError:                           true,
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // search_path is per connection
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Exec(fmt.Sprintf(`SET search_path TO %s, public`, schema)).Error)

	require.NoError(t, database.PrepareForAutoMigrate(db))
	require.NoError(t, db.AutoMigrate(schemaModels()...), "the startup schema must build")
	return db
}

func TestStartupSchema_Success(t *testing.T) {
	db := startupSchemaDB(t)

	assert.True(t, db.Migrator().HasTable(&domain.MitigationSubAction{}),
		"mitigation_subactions must exist on a startup-built schema")
	for _, m := range schemaModels() {
		assert.True(t, db.Migrator().HasTable(m), "table for %T", m)
	}

	tenant := uuid.New()
	plan := &domain.Mitigation{ID: uuid.New(), TenantID: tenant, RiskID: uuid.New(), Title: "Harden the edge", Status: domain.MitigationPlanned, CreatedBy: uuid.New()}
	require.NoError(t, db.Create(plan).Error)

	repo := repository.NewGormMitigationSubActionRepository(db)
	first := &domain.MitigationSubAction{ID: uuid.New(), MitigationID: plan.ID, Title: "Inventory", Completed: true}
	require.NoError(t, repo.Create(tenant.String(), first))
	second := &domain.MitigationSubAction{ID: uuid.New(), MitigationID: plan.ID, Title: "Patch", DependsOn: &first.ID, Order: 1}
	require.NoError(t, repo.Create(tenant.String(), second))

	list, err := repo.List(tenant.String(), plan.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	ok, err := repo.CanComplete(tenant.String(), second.ID)
	require.NoError(t, err)
	assert.True(t, ok)

	deps, err := repo.GetDependencies(tenant.String(), first.ID)
	require.NoError(t, err)
	assert.Len(t, deps, 1)

	cycle, err := repo.HasCycle(tenant.String(), first.ID, second.ID)
	require.NoError(t, err)
	assert.True(t, cycle)
}

func TestStartupSchema_NotFound(t *testing.T) {
	db := startupSchemaDB(t)
	repo := repository.NewGormMitigationSubActionRepository(db)
	_, err := repo.CanComplete(uuid.New().String(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestStartupSchema_Unauthorized(t *testing.T) {
	db := startupSchemaDB(t)
	repo := repository.NewGormMitigationSubActionRepository(db)

	tenantA, tenantB := uuid.New(), uuid.New()
	planB := &domain.Mitigation{ID: uuid.New(), TenantID: tenantB, RiskID: uuid.New(), Title: "B's plan", Status: domain.MitigationPlanned, CreatedBy: uuid.New()}
	require.NoError(t, db.Create(planB).Error)
	secret := &domain.MitigationSubAction{ID: uuid.New(), MitigationID: planB.ID, Title: "Tenant B secret step"}
	require.NoError(t, repo.Create(tenantB.String(), secret))

	assert.ErrorIs(t, repo.Create(tenantA.String(), &domain.MitigationSubAction{ID: uuid.New(), MitigationID: planB.ID, Title: "intruder"}), domain.ErrForbidden)
	_, err := repo.CanComplete(tenantA.String(), secret.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = repo.List(tenantA.String(), planB.ID)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}
