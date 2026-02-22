package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/opendefender/openrisk/backend/internal/domain"
	"github.com/opendefender/openrisk/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type IntegrationTestSuite struct {
	suite.Suite
	db             *gorm.DB
	cacheService   *services.CacheService
	queryOptimizer *services.QueryOptimizer
	ctx            context.Context
}

// Setup database connection for integration tests
func (suite *IntegrationTestSuite) SetupSuite() {
	// Use test database
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("TEST_DB_HOST"),
		os.Getenv("TEST_DB_PORT"),
		os.Getenv("TEST_DB_USER"),
		os.Getenv("TEST_DB_PASSWORD"),
		os.Getenv("TEST_DB_NAME"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	suite.NoError(err, "Failed to connect to test database")
	suite.db = db

	suite.ctx = context.Background()
}

// Clean up after each test
func (suite *IntegrationTestSuite) TearDownTest() {
	// Clean test data
	suite.db.Exec("TRUNCATE TABLE risks CASCADE")
	suite.db.Exec("TRUNCATE TABLE mitigations CASCADE")
	suite.db.Exec("TRUNCATE TABLE sub_actions CASCADE")
	suite.db.Exec("TRUNCATE TABLE assets CASCADE")
	suite.db.Exec("TRUNCATE TABLE custom_fields CASCADE")
}

// Test Risk CRUD Operations
func (suite *IntegrationTestSuite) TestRiskCRUD() {
	// Create
	risk := &domain.Risk{
		Title:       "Test Risk",
		Description: "Integration test risk",
		Status:      "open",
		Score:       75,
		Impact:      "high",
		Probability: "medium",
		Priority:    1,
	}

	result := suite.db.Create(risk)
	suite.NoError(result.Error)
	suite.NotZero(risk.ID)

	// Read
	var retrieved domain.Risk
	suite.db.First(&retrieved, risk.ID)
	assert.Equal(suite.T(), risk.Title, retrieved.Title)

	// Update
	risk.Status = "mitigated"
	suite.db.Save(risk)

	var updated domain.Risk
	suite.db.First(&updated, risk.ID)
	assert.Equal(suite.T(), "mitigated", updated.Status)

	// Delete
	suite.db.Delete(risk)

	var deleted domain.Risk
	result = suite.db.First(&deleted, risk.ID)
	assert.Equal(suite.T(), sql.ErrNoRows, result.Error)
}

// Test Mitigation CRUD Operations
func (suite *IntegrationTestSuite) TestMitigationCRUD() {
	// Create risk first
	risk := &domain.Risk{
		Title:       "Test Risk for Mitigation",
		Status:      "open",
		Score:       50,
		Impact:      "medium",
		Probability: "medium",
	}
	suite.db.Create(risk)

	// Create mitigation
	mitigation := &domain.Mitigation{
		RiskID:      risk.ID,
		Title:       "Test Mitigation",
		Status:      "pending",
		DueDate:     time.Now().AddDate(0, 0, 30),
		Owner:       "Test User",
		Description: "Test mitigation description",
	}

	result := suite.db.Create(mitigation)
	suite.NoError(result.Error)
	suite.NotZero(mitigation.ID)

	// Verify relationship
	var retrieved domain.Mitigation
	suite.db.Preload("Risk").First(&retrieved, mitigation.ID)
	assert.NotNil(suite.T(), retrieved.Risk)
	assert.Equal(suite.T(), risk.ID, retrieved.RiskID)
}

// Test Asset Relationships
func (suite *IntegrationTestSuite) TestAssetRelationships() {
	// Create risk
	risk := &domain.Risk{
		Title:       "Test Risk with Assets",
		Status:      "open",
		Score:       60,
		Impact:      "high",
		Probability: "high",
	}
	suite.db.Create(risk)

	// Create asset
	asset := &domain.Asset{
		Name:        "Test Asset",
		AssetType:   "system",
		Description: "Test asset for risk",
		Location:    "Test Location",
	}
	suite.db.Create(asset)

	// Create association
	suite.db.Model(risk).Association("Assets").Append(asset)

	// Verify association
	var riskWithAssets domain.Risk
	suite.db.Preload("Assets").First(&riskWithAssets, risk.ID)
	assert.Len(suite.T(), riskWithAssets.Assets, 1)
	assert.Equal(suite.T(), asset.ID, riskWithAssets.Assets[0].ID)
}

// Test Bulk Operations
func (suite *IntegrationTestSuite) TestBulkOperations() {
	// Create multiple risks
	risks := []domain.Risk{
		{Title: "Risk 1", Status: "open", Score: 50, Impact: "medium", Probability: "medium"},
		{Title: "Risk 2", Status: "open", Score: 75, Impact: "high", Probability: "high"},
		{Title: "Risk 3", Status: "open", Score: 25, Impact: "low", Probability: "low"},
	}

	for _, risk := range risks {
		suite.db.Create(&risk)
	}

	// Bulk update
	result := suite.db.Model(&domain.Risk{}).
		Where("status = ?", "open").
		Update("status", "in_review").
		RowsAffected

	assert.Equal(suite.T(), int64(3), result)

	// Verify update
	var count int64
	suite.db.Model(&domain.Risk{}).Where("status = ?", "in_review").Count(&count)
	assert.Equal(suite.T(), int64(3), count)
}

// Test Query Performance
func (suite *IntegrationTestSuite) TestQueryPerformance() {
	// Create test data
	for i := 0; i < 100; i++ {
		risk := domain.Risk{
			Title:       fmt.Sprintf("Risk %d", i),
			Status:      "open",
			Score:       int32(i % 100),
			Impact:      "medium",
			Probability: "medium",
		}
		suite.db.Create(&risk)
	}

	// Measure query time
	start := time.Now()

	var risks []domain.Risk
	suite.db.Where("status = ?", "open").Limit(20).Find(&risks)

	duration := time.Since(start)

	// Assert query completes within reasonable time (< 100ms for indexed query)
	assert.Less(suite.T(), duration, 100*time.Millisecond,
		fmt.Sprintf("Query took %v, expected < 100ms", duration))
	assert.Len(suite.T(), risks, 20)
}

// Test Concurrent Operations
func (suite *IntegrationTestSuite) TestConcurrentOperations() {
	done := make(chan bool, 10)

	// Spawn 10 concurrent goroutines creating risks
	for i := 0; i < 10; i++ {
		go func(index int) {
			risk := domain.Risk{
				Title:       fmt.Sprintf("Concurrent Risk %d", index),
				Status:      "open",
				Score:       int32(index * 10),
				Impact:      "medium",
				Probability: "medium",
			}
			result := suite.db.Create(&risk)
			suite.NoError(result.Error)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all were created
	var count int64
	suite.db.Model(&domain.Risk{}).Count(&count)
	assert.Greater(suite.T(), count, int64(0))
}

// Test Transaction Rollback
func (suite *IntegrationTestSuite) TestTransactionRollback() {
	tx := suite.db.BeginTx(suite.ctx, &sql.TxOptions{})

	risk := domain.Risk{
		Title:       "Transaction Test Risk",
		Status:      "open",
		Score:       50,
		Impact:      "medium",
		Probability: "medium",
	}
	tx.Create(&risk)

	// Rollback transaction
	tx.Rollback()

	// Verify risk wasn't created
	var retrieved domain.Risk
	result := suite.db.First(&retrieved, risk.ID)
	assert.Equal(suite.T(), sql.ErrNoRows, result.Error)
}

// Test Custom Fields Storage
func (suite *IntegrationTestSuite) TestCustomFieldsStorage() {
	// Create custom field
	field := domain.CustomField{
		Name:        "severity_category",
		FieldType:   "choice",
		Description: "Custom severity category",
		IsRequired:  true,
		Options:     []string{"Critical", "High", "Medium", "Low"},
		Scope:       "risk",
	}

	result := suite.db.Create(&field)
	suite.NoError(result.Error)

	// Retrieve and verify
	var retrieved domain.CustomField
	suite.db.First(&retrieved, field.ID)
	assert.Equal(suite.T(), field.Name, retrieved.Name)
	assert.Len(suite.T(), retrieved.Options, 4)
}

// Test Audit Log Creation
func (suite *IntegrationTestSuite) TestAuditLogCreation() {
	auditLog := domain.AuditLog{
		UserID:        1,
		Action:        "CREATE_RISK",
		ResourceType:  "Risk",
		ResourceID:    "123",
		ChangedFields: map[string]interface{}{"title": "New Risk"},
		IPAddress:     "127.0.0.1",
		UserAgent:     "Mozilla/5.0",
	}

	result := suite.db.Create(&auditLog)
	suite.NoError(result.Error)

	// Verify
	var retrieved domain.AuditLog
	suite.db.First(&retrieved, auditLog.ID)
	assert.Equal(suite.T(), "CREATE_RISK", retrieved.Action)
}

// Run all integration tests
func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
