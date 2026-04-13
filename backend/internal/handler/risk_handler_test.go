package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	applicationrisk "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/domain"
)

// Test-only lightweight structs to avoid DB-specific defaults (used with sqlite in-memory)
type UserT struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"not null"`
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
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (UserT) TableName() string { return "users" }

type RiskT struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Title       string    `gorm:"size:255;not null"`
	Description string    `gorm:"type:text"`
	Impact      int
	Probability int
	Score       float64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (RiskT) TableName() string { return "risks" }

type MitigationT struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	RiskID    uuid.UUID
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (MitigationT) TableName() string { return "mitigations" }

type AssetT struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string
}

func (AssetT) TableName() string { return "assets" }

type RiskHistoryT struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	RiskID    uuid.UUID
	Score     float64
	Impact    int
	Probability int
	Status    string
	ChangedBy string
	ChangeType string
	CreatedAt time.Time
}

func (RiskHistoryT) TableName() string { return "risk_histories" }

func setupAppWithDB(t *testing.T) *fiber.App {
	// In-memory SQLite for fast tests
	dsn := "file:risk_handler_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	// migrate schema using test-only structs
	if err := db.AutoMigrate(&UserT{}, &MitigationT{}, &AssetT{}, &RiskHistoryT{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS risks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		impact INTEGER,
		probability INTEGER,
		score REAL,
		status TEXT,
		tags TEXT,
		owner TEXT,
		source TEXT,
		external_id TEXT,
		level TEXT,
		custom_fields TEXT,
		frameworks TEXT,
		organization_id TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	);`).Error; err != nil {
		t.Fatalf("create risks table failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS risk_assets (
		risk_id TEXT NOT NULL,
		asset_id TEXT NOT NULL
	);`).Error; err != nil {
		t.Fatalf("create risk_assets table failed: %v", err)
	}

	// replace global DB used by handlers
	database.DB = db

	app := fiber.New()
	testOrgID := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{
			UserID:         uuid.New(),
			OrganizationID: testOrgID,
		})
		return c.Next()
	})
	api := app.Group("/api/v1")
	riskRepo := repository.NewGormRiskRepository(db)
	handler := NewRiskHandler(
		applicationrisk.NewCreateRiskUseCase(riskRepo),
		applicationrisk.NewGetRiskUseCase(riskRepo),
		applicationrisk.NewListRisksUseCase(riskRepo),
		applicationrisk.NewUpdateRiskUseCase(riskRepo),
		applicationrisk.NewDeleteRiskUseCase(riskRepo),
	)
	api.Post("/risks", handler.CreateRisk)
	api.Get("/risks/:id", handler.GetRisk)
	api.Patch("/risks/:id", handler.UpdateRisk)
	api.Delete("/risks/:id", handler.DeleteRisk)

	return app
}

func TestRiskCRUDFlow(t *testing.T) {
	app := setupAppWithDB(t)

	// 1. Create risk
	payload := map[string]interface{}{
		"title":       "Test Risk",
		"description": "desc",
		"impact":      3,
		"probability": 4,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201 got %d", resp.StatusCode)
	}

	var created domain.Risk
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == uuid.Nil {
		t.Fatalf("expected created id, got nil")
	}

	// 2. Get risk
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/risks/"+created.ID.String(), nil)
	getResp, _ := app.Test(getReq)
	if getResp.StatusCode != 200 {
		t.Fatalf("expected 200 got %d", getResp.StatusCode)
	}

	// 3. Update risk
	updatePayload := map[string]interface{}{"title": "Updated", "impact": 5}
	ub, _ := json.Marshal(updatePayload)
	upReq := httptest.NewRequest(http.MethodPatch, "/api/v1/risks/"+created.ID.String(), bytes.NewReader(ub))
	upReq.Header.Set("Content-Type", "application/json")
	upResp, _ := app.Test(upReq)
	if upResp.StatusCode != 200 {
		t.Fatalf("expected 200 on update got %d", upResp.StatusCode)
	}

	var updated domain.Risk
	json.NewDecoder(upResp.Body).Decode(&updated)
	if updated.Title != "Updated" || updated.Impact != 5 {
		t.Fatalf("update did not apply: %+v", updated)
	}

	// 4. Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/risks/"+created.ID.String(), nil)
	delResp, _ := app.Test(delReq)
	if delResp.StatusCode != 204 {
		t.Fatalf("expected 204 on delete got %d", delResp.StatusCode)
	}
}

func TestCreateValidationFail(t *testing.T) {
	app := setupAppWithDB(t)

	// Missing required title
	payload := map[string]interface{}{"impact": 3, "probability": 4}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 got %d", resp.StatusCode)
	}
}
