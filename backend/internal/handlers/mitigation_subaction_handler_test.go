package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestCreateMitigationSubAction_InvalidMitigationID tests invalid UUID
func TestCreateMitigationSubAction_InvalidMitigationID(t *testing.T) {
	app := fiber.New()
	app.Post("/mitigations/:id/subactions", CreateMitigationSubAction)

	payload := map[string]string{"title": "Test"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/mitigations/invalid-uuid/subactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Invalid mitigation ID", result["error"])
}

// TestCreateMitigationSubAction_EmptyTitle tests empty title validation
func TestCreateMitigationSubAction_EmptyTitle(t *testing.T) {
	app := fiber.New()
	app.Post("/mitigations/:id/subactions", CreateMitigationSubAction)

	payload := map[string]string{"title": ""}
	body, _ := json.Marshal(payload)

	testMitigationID := uuid.New().String()
	req := httptest.NewRequest("POST", "/mitigations/"+testMitigationID+"/subactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Invalid payload", result["error"])
}

// TestToggleMitigationSubAction_SubActionNotFound tests non-existent sub-action
func TestToggleMitigationSubAction_SubActionNotFound(t *testing.T) {
	app := fiber.New()
	app.Patch("/mitigations/:id/subactions/:subactionId/toggle", ToggleMitigationSubAction)

	fakeSubID := uuid.New().String()
	testMitID := uuid.New().String()
	req := httptest.NewRequest("PATCH", "/mitigations/"+testMitID+"/subactions/"+fakeSubID+"/toggle", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Sub-action not found", result["error"])
}

// TestToggleMitigationSubAction_OwnershipMismatch tests sub-action from different mitigation
func TestToggleMitigationSubAction_OwnershipMismatch(t *testing.T) {
	app := fiber.New()
	app.Patch("/mitigations/:id/subactions/:subactionId/toggle", ToggleMitigationSubAction)

	// Create two mitigations
	mit1 := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Mitigation 1",
		Status: domain.MitigationPlanned,
	}
	mit2 := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Mitigation 2",
		Status: domain.MitigationPlanned,
	}
	database.DB.Create(&mit1)
	database.DB.Create(&mit2)
	defer database.DB.Delete(&mit1)
	defer database.DB.Delete(&mit2)

	// Create sub-action for mit1
	subaction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mit1.ID,
		Title:        "Test Sub-Action",
		Completed:    false,
	}
	database.DB.Create(&subaction)
	defer database.DB.Delete(&subaction)

	// Try to toggle with mit2 ID (mismatch)
	req := httptest.NewRequest("PATCH", "/mitigations/"+mit2.ID.String()+"/subactions/"+subaction.ID.String()+"/toggle", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Sub-action not found for given mitigation", result["error"])
}

// TestDeleteMitigationSubAction_Success tests successful deletion
func TestDeleteMitigationSubAction_Success(t *testing.T) {
	app := fiber.New()
	app.Delete("/mitigations/:id/subactions/:subactionId", DeleteMitigationSubAction)

	// Create test mitigation and sub-action
	mitigation := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Test Mitigation",
		Status: domain.MitigationPlanned,
	}
	database.DB.Create(&mitigation)
	defer database.DB.Delete(&mitigation)

	subaction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mitigation.ID,
		Title:        "Test Sub-Action",
		Completed:    false,
	}
	database.DB.Create(&subaction)

	// Delete request
	req := httptest.NewRequest("DELETE", "/mitigations/"+mitigation.ID.String()+"/subactions/"+subaction.ID.String(), nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)

	// Verify deletion
	var result domain.MitigationSubAction
	err = database.DB.First(&result, "id = ?", subaction.ID).Error
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestDeleteMitigationSubAction_SubActionNotFound tests deletion of non-existent sub-action
func TestDeleteMitigationSubAction_SubActionNotFound(t *testing.T) {
	app := fiber.New()
	app.Delete("/mitigations/:id/subactions/:subactionId", DeleteMitigationSubAction)

	mitigation := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Test Mitigation",
		Status: domain.MitigationPlanned,
	}
	database.DB.Create(&mitigation)
	defer database.DB.Delete(&mitigation)

	fakeSubID := uuid.New().String()
	req := httptest.NewRequest("DELETE", "/mitigations/"+mitigation.ID.String()+"/subactions/"+fakeSubID, nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Sub-action not found", result["error"])
}

// TestDeleteMitigationSubAction_OwnershipMismatch tests delete sub-action from different mitigation
func TestDeleteMitigationSubAction_OwnershipMismatch(t *testing.T) {
	app := fiber.New()
	app.Delete("/mitigations/:id/subactions/:subactionId", DeleteMitigationSubAction)

	// Create two mitigations
	mit1 := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Mitigation 1",
		Status: domain.MitigationPlanned,
	}
	mit2 := domain.Mitigation{
		ID:     uuid.New(),
		RiskID: uuid.New(),
		Title:  "Mitigation 2",
		Status: domain.MitigationPlanned,
	}
	database.DB.Create(&mit1)
	database.DB.Create(&mit2)
	defer database.DB.Delete(&mit1)
	defer database.DB.Delete(&mit2)

	// Create sub-action for mit1
	subaction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mit1.ID,
		Title:        "Test Sub-Action",
		Completed:    false,
	}
	database.DB.Create(&subaction)
	defer database.DB.Delete(&subaction)

	// Try to delete with mit2 ID (mismatch)
	req := httptest.NewRequest("DELETE", "/mitigations/"+mit2.ID.String()+"/subactions/"+subaction.ID.String(), nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "Sub-action not found for given mitigation", result["error"])

	// Verify sub-action still exists
	var stillExists domain.MitigationSubAction
	err = database.DB.First(&stillExists, "id = ?", subaction.ID).Error
	assert.NoError(t, err)
}
