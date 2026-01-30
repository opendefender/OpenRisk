package handlers

import (
	"os"
	"testing"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDB initializes a test database connection
func TestDB(t testing.T) gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:/openrisk_test"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	return db
}

// SetupTestDB runs migrations and returns a database connection
func SetupTestDB(t testing.T) gorm.DB {
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
func CleanupTestDB(t testing.T, db gorm.DB) {
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
func InitTestDB(t testing.T) {
	db := SetupTestDB(t)
	database.DB = db
}
