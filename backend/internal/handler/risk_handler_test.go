// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/crq"
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
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	RiskID      uuid.UUID
	Score       float64
	Impact      int
	Probability int
	Status      string
	ChangedBy   string
	ChangeType  string
	CreatedAt   time.Time
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
	// Hand-written (AutoMigrate can't build this model on sqlite: gen_random_uuid()
	// default + pq array columns). Kept in sync with domain.Risk — every column the
	// repository INSERT/RETURNING touches must exist, else the insert 400s. This
	// had drifted (missing tenant_id + ~15 newer columns), which was the real cause
	// of the long-standing TestRiskCRUDFlow failure.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS risks (
		id TEXT PRIMARY KEY,
		tenant_id TEXT,
		organization_id TEXT,
		name TEXT,
		title TEXT NOT NULL,
		description TEXT,
		probability REAL,
		impact REAL,
		score REAL,
		criticality TEXT,
		impact_legacy INTEGER,
		probability_legacy INTEGER,
		status TEXT,
		level TEXT,
		lifecycle_phase TEXT,
		created_by TEXT,
		assigned_to TEXT,
		reviewer_id TEXT,
		owner TEXT,
		asset_id TEXT,
		treatment_plan TEXT,
		residual_risk REAL,
		last_mitigated_at DATETIME,
		slexaf REAL,
		aro REAL,
		review_interval_days INTEGER,
		next_review_at DATETIME,
		last_reviewed_at DATETIME,
		source TEXT,
		source_cve_id TEXT,
		external_id TEXT,
		custom_fields TEXT,
		tags TEXT,
		frameworks TEXT,
		control_ids TEXT,
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
		applicationrisk.NewMarkRiskReviewedUseCase(riskRepo),
		applicationrisk.NewTransitionPhaseUseCase(riskRepo),
		nil,
		crq.NewQuantifier(0, crq.Reference{}),
	)
	api.Post("/risks", handler.CreateRisk)
	api.Get("/risks/:id", handler.GetRisk)
	api.Patch("/risks/:id", handler.UpdateRisk)
	api.Post("/risks/:id/transition", handler.TransitionPhase)
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
		// Probability is on the Score Engine's 0.0–1.0 scale (validated min=0,max=1);
		// the old 1–5 value here (4) always failed validation → the pre-existing 400.
		"probability": 0.4,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	defer resp.Body.Close()
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
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("expected 200 got %d", getResp.StatusCode)
	}

	// 3. Update risk
	updatePayload := map[string]interface{}{"title": "Updated", "impact": 5}
	ub, _ := json.Marshal(updatePayload)
	upReq := httptest.NewRequest(http.MethodPatch, "/api/v1/risks/"+created.ID.String(), bytes.NewReader(ub))
	upReq.Header.Set("Content-Type", "application/json")
	upResp, _ := app.Test(upReq)
	defer upResp.Body.Close()
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
	defer delResp.Body.Close()
	if delResp.StatusCode != 204 {
		t.Fatalf("expected 204 on delete got %d", delResp.StatusCode)
	}
}

func TestCreateValidationFail(t *testing.T) {
	app := setupAppWithDB(t)

	// Missing required title
	payload := map[string]interface{}{"impact": 3, "probability": 0.4}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 got %d", resp.StatusCode)
	}
}
