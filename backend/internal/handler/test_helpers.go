// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"os"
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDB initializes a test database connection
func TestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatalf("DATABASE_URL environment variable not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	return db
}

// SetupTestDB runs migrations and returns a database connection
func SetupTestDB(t *testing.T) *gorm.DB {
	db := TestDB(t)

	// Run auto migrations
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Risk{},
		&domain.Mitigation{},
		&domain.Asset{},
		&domain.RiskHistory{},
		&domain.APIToken{},
	); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return db
}

// CleanupTestDB truncates all test data
func CleanupTestDB(t *testing.T, db *gorm.DB) {
	tables := []string{
		"api_tokens",
		"risk_assets",
		"mitigations",
		"mitigation_subactions",
		"risks",
		"risk_histories",
		"users",
	}

	for _, table := range tables {
		if err := db.Exec("TRUNCATE TABLE " + table + " CASCADE").Error; err != nil {
			t.Logf("Warning: Failed to truncate %s: %v", table, err)
		}
	}
}

// InitTestDB initializes database.DB singleton for tests
func InitTestDB(t *testing.T) {
	db := SetupTestDB(t)
	database.DB = db
}
