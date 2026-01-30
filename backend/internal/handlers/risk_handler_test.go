package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// Test-only lightweight structs to avoid DB-specific defaults (used with sqlite in-memory)
type UserT struct {
	ID           uuid.UUID gorm:"type:uuid;primaryKey"
	Email        string    gorm:"not null"
	Password     string
	FullName     string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Tags         string
	Owner        string
	Source       string
	ExternalID   string
	CustomFields string
	Frameworks   string
	DeletedAt    gorm.DeletedAt gorm:"index"
}

func (UserT) TableName() string { return "users" }

type RiskT struct {
	ID          uuid.UUID gorm:"type:uuid;primaryKey"
	Title       string    gorm:"size:;not null"
	Description string    gorm:"type:text"
	Impact      int
	Probability int
	Score       float
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt gorm:"index"
}

func (RiskT) TableName() string { return "risks" }

type MitigationT struct {
	ID        uuid.UUID gorm:"type:uuid;primaryKey"
	RiskID    uuid.UUID
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt gorm:"index"
}

func (MitigationT) TableName() string { return "mitigations" }

type AssetT struct {
	ID   uuid.UUID gorm:"type:uuid;primaryKey"
	Name string
}

func (AssetT) TableName() string { return "assets" }

type RiskHistoryT struct {
	ID        uuid.UUID gorm:"type:uuid;primaryKey"
	RiskID    uuid.UUID
	Score     float
	CreatedAt time.Time
}

func (RiskHistoryT) TableName() string { return "risk_histories" }

func setupAppWithDB(t testing.T) fiber.App {
	// In-memory SQLite for fast tests
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	// migrate schema using test-only structs
	if err := db.AutoMigrate(&UserT{}, &RiskT{}, &MitigationT{}, &AssetT{}, &RiskHistoryT{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	// replace global DB used by handlers
	database.DB = db

	app := fiber.New()
	api := app.Group("/api/v")
	api.Post("/risks", CreateRisk)
	api.Get("/risks/:id", GetRisk)
	api.Patch("/risks/:id", UpdateRisk)
	api.Delete("/risks/:id", DeleteRisk)

	return app
}

func TestRiskCRUDFlow(t testing.T) {
	app := setupAppWithDB(t)

	// . Create risk
	payload := map[string]interface{}{
		"title":       "Test Risk",
		"description": "desc",
		"impact":      ,
		"probability": ,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode !=  {
		t.Fatalf("expected  got %d", resp.StatusCode)
	}

	var created domain.Risk
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == uuid.Nil {
		t.Fatalf("expected created id, got nil")
	}

	// . Get risk
	getReq := httptest.NewRequest(http.MethodGet, "/api/v/risks/"+created.ID.String(), nil)
	getResp, _ := app.Test(getReq)
	if getResp.StatusCode !=  {
		t.Fatalf("expected  got %d", getResp.StatusCode)
	}

	// . Update risk
	updatePayload := map[string]interface{}{"title": "Updated", "impact": }
	ub, _ := json.Marshal(updatePayload)
	upReq := httptest.NewRequest(http.MethodPatch, "/api/v/risks/"+created.ID.String(), bytes.NewReader(ub))
	upReq.Header.Set("Content-Type", "application/json")
	upResp, _ := app.Test(upReq)
	if upResp.StatusCode !=  {
		t.Fatalf("expected  on update got %d", upResp.StatusCode)
	}

	var updated domain.Risk
	json.NewDecoder(upResp.Body).Decode(&updated)
	if updated.Title != "Updated" || updated.Impact !=  {
		t.Fatalf("update did not apply: %+v", updated)
	}

	// . Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v/risks/"+created.ID.String(), nil)
	delResp, _ := app.Test(delReq)
	if delResp.StatusCode !=  {
		t.Fatalf("expected  on delete got %d", delResp.StatusCode)
	}
}

func TestCreateValidationFail(t testing.T) {
	app := setupAppWithDB(t)

	// Missing required title
	payload := map[string]interface{}{"impact": , "probability": }
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode !=  {
		t.Fatalf("expected  got %d", resp.StatusCode)
	}
}
