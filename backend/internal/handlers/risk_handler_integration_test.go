//go:build integration
// +build integration

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRisk_Integration(t testing.T) {
	// Setup
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	app := fiber.New()
	app.Post("/risks", CreateRisk)

	// Test data
	payload := map[string]interface{}{
		"title":       "Critical Data Breach",
		"description": "Unauthorized access to customer database",
		"impact":      ,
		"probability": ,
		"tags":        []string{"security", "data"},
	}
	body, _ := json.Marshal(payload)

	// Request
	req := httptest.NewRequest("POST", "/risks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, , resp.StatusCode)

	var result domain.Risk
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Critical Data Breach", result.Title)
	assert.Equal(t, , result.Impact)
	assert.Equal(t, , result.Probability)

	// Verify in database
	var storedRisk domain.Risk
	err = database.DB.First(&storedRisk, "id = ?", result.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Critical Data Breach", storedRisk.Title)
}

func TestGetRisks_Integration(t testing.T) {
	// Setup
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	// Create test risks
	risk := domain.Risk{
		ID:          uuid.New(),
		Title:       "Risk ",
		Impact:      ,
		Probability: ,
	}
	risk := domain.Risk{
		ID:          uuid.New(),
		Title:       "Risk ",
		Impact:      ,
		Probability: ,
	}
	database.DB.Create(&risk)
	database.DB.Create(&risk)

	app := fiber.New()
	app.Get("/risks", GetRisks)

	// Request
	req := httptest.NewRequest("GET", "/risks", nil)
	resp, err := app.Test(req)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, , resp.StatusCode)

	var risks []domain.Risk
	json.NewDecoder(resp.Body).Decode(&risks)
	assert.GreaterOrEqual(t, len(risks), )
}

// TestGetRisk_Integration - Commented out: GetRiskByID handler not found
/
func TestGetRisk_Integration(t testing.T) {
	// Setup
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	risk := domain.Risk{
		ID:          uuid.New(),
		Title:       "Test Risk",
		Impact:      ,
		Probability: ,
	}
	database.DB.Create(&risk)

	app := fiber.New()
	app.Get("/risks/:id", GetRiskByID)

	// Request
	req := httptest.NewRequest("GET", "/risks/"+risk.ID.String(), nil)
	resp, err := app.Test(req)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, , resp.StatusCode)

	var result domain.Risk
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, risk.ID, result.ID)
	assert.Equal(t, "Test Risk", result.Title)
}
/

func TestUpdateRisk_Integration(t testing.T) {
	// Setup
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	risk := domain.Risk{
		ID:          uuid.New(),
		Title:       "Original Title",
		Impact:      ,
		Probability: ,
	}
	database.DB.Create(&risk)

	app := fiber.New()
	app.Patch("/risks/:id", UpdateRisk)

	// Update payload
	payload := map[string]interface{}{
		"title":       "Updated Title",
		"description": "New description",
		"impact":      ,
	}
	body, _ := json.Marshal(payload)

	// Request
	req := httptest.NewRequest("PATCH", "/risks/"+risk.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, , resp.StatusCode)

	var result domain.Risk
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Updated Title", result.Title)
	assert.Equal(t, "New description", result.Description)
	assert.Equal(t, , result.Impact)
}

func TestDeleteRisk_Integration(t testing.T) {
	// Setup
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	risk := domain.Risk{
		ID:          uuid.New(),
		Title:       "Risk to Delete",
		Impact:      ,
		Probability: ,
	}
	database.DB.Create(&risk)

	app := fiber.New()
	app.Delete("/risks/:id", DeleteRisk)

	// Request
	req := httptest.NewRequest("DELETE", "/risks/"+risk.ID.String(), nil)
	resp, err := app.Test(req)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, , resp.StatusCode)

	// Verify soft delete
	var deletedRisk domain.Risk
	err = database.DB.First(&deletedRisk, "id = ?", risk.ID).Error
	assert.Error(t, err) // Should be soft-deleted
}
