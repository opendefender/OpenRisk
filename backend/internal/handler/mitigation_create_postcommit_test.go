// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newCreateMitigationApp wires POST /risks/:id/mitigations over an in-memory
// database holding the tables the write touches, and returns the id of a risk
// that belongs to the caller's tenant.
//
// failReadBack is the point of the #335 test. GetByIDWithSubActions — the
// read-back the handler performs after the write has committed — issues a
// Preload("Risk"). A query callback fails exactly that preload, which
// reproduces a post-commit read failure through the real code path rather than
// a mock. The ownership check before the write reads `risks` without a model,
// so the callback leaves it alone.
func newCreateMitigationApp(t *testing.T, failReadBack bool) (*fiber.App, *gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()

	dsn := "file:mitigation_postcommit_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE TABLE mitigations (
		id TEXT PRIMARY KEY, tenant_id TEXT, risk_id TEXT,
		title TEXT, description TEXT, status TEXT, priority TEXT,
		owner_id TEXT, assignee_id TEXT, reviewer_id TEXT,
		assigned_to TEXT, progress INTEGER DEFAULT 0,
		reminder_d7_sent_at DATETIME, reminder_d1_sent_at DATETIME, reminder_overdue_sent_at DATETIME,
		created_by TEXT, approved_by TEXT, approved_at DATETIME,
		source TEXT, auto_detected_at DATETIME, scanner_config_id TEXT,
		organization_id TEXT, assignee TEXT, cost INTEGER, mitigation_time INTEGER, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE mitigation_subactions (
		id TEXT PRIMARY KEY, mitigation_id TEXT, title TEXT, description TEXT,
		completed BOOLEAN DEFAULT 0, completed_at DATETIME, completed_by TEXT,
		completed_source TEXT, auto_detected_at DATETIME, depends_on TEXT,
		"order" INTEGER DEFAULT 0, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE risks (
		id TEXT PRIMARY KEY, tenant_id TEXT, title TEXT, description TEXT,
		status TEXT, score REAL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	if failReadBack {
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_risk_preload",
			func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "risks" {
					_ = tx.AddError(errors.New("simulated read-back failure"))
				}
			}))
	}

	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	tenantID := uuid.New()
	userID := uuid.New()
	riskID := seedRisk(t, db, tenantID)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: userID, OrganizationID: tenantID})
		return c.Next()
	})
	app.Post("/api/v1/risks/:id/mitigations", CreateMitigation)

	return app, db, tenantID, riskID
}

func seedRisk(t *testing.T, db *gorm.DB, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, title, status) VALUES (?, ?, ?, ?)`,
		id.String(), tenantID.String(), "Ransomware on the file server", "open").Error)
	return id
}

func postMitigation(t *testing.T, app *fiber.App, riskID string) (int, map[string]interface{}) {
	t.Helper()

	body, err := json.Marshal(map[string]interface{}{
		"title":    "Patch the vulnerable dependency",
		"priority": "high",
		"sub_actions": []map[string]interface{}{
			{"title": "Inventory affected hosts"},
			{"title": "Apply the patch"},
			{"title": "Verify the fix"},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks/"+riskID+"/mitigations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	decoded := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return res.StatusCode, decoded
}

// Criterion 1 and 5 — the ordinary path still answers 201 with the canonical plan.
func TestCreateMitigation_Success_ReturnsCreatedPlan(t *testing.T) {
	app, db, _, riskID := newCreateMitigationApp(t, false)

	status, body := postMitigation(t, app, riskID.String())

	require.Equal(t, http.StatusCreated, status)
	assert.NotEmpty(t, body["id"], "the response must carry the plan id")
	assert.NotNil(t, body["risk"], "the read-back resolves the caller's own risk")

	var plans, subs int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	require.NoError(t, db.Table("mitigation_subactions").Count(&subs).Error)
	assert.Equal(t, int64(1), plans)
	assert.Equal(t, int64(3), subs)
}

// Criterion 4 — the write commits, the post-commit read-back fails, and the
// client is still told the truth: 201, with the plan it created.
//
// Before #335 this path returned 500 while the plan sat committed in the
// database, which is what drove users to retry and create duplicates.
func TestCreateMitigation_PostCommitReadFailure_StillReturns201(t *testing.T) {
	app, db, _, riskID := newCreateMitigationApp(t, true) // Preload("Risk") fails

	status, body := postMitigation(t, app, riskID.String())

	assert.Equal(t, http.StatusCreated, status,
		"a committed write must never be reported to the client as a failure")
	assert.NotEqual(t, http.StatusInternalServerError, status)

	// Criterion 5 — the body identifies what was created, so a client that
	// retried can reconcile.
	assert.NotEmpty(t, body["id"], "the response must carry the plan id")
	assert.Nil(t, body["risk"], "the answer came from the committed entity, not the failed read-back")

	// The write really did commit, whole.
	var plans, subs int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	require.NoError(t, db.Table("mitigation_subactions").Count(&subs).Error)
	assert.Equal(t, int64(1), plans)
	assert.Equal(t, int64(3), subs, "the checklist committed with the plan")
}

// Criterion 6 — a request with no tenant context is refused before any write.
func TestCreateMitigation_Unauthorized_NoContext(t *testing.T) {
	_, db, _, _ := newCreateMitigationApp(t, false)

	app := fiber.New()
	app.Post("/api/v1/risks/:id/mitigations", CreateMitigation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks/"+uuid.New().String()+"/mitigations",
		bytes.NewReader([]byte(`{"title":"x"}`)))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

	var plans int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	assert.Equal(t, int64(0), plans, "an unauthorized request must write nothing")
}

// A malformed risk id is rejected before any write (NotFound-shaped input guard).
func TestCreateMitigation_InvalidRiskID_NoWrite(t *testing.T) {
	app, db, _, _ := newCreateMitigationApp(t, false)

	status, _ := postMitigation(t, app, "not-a-uuid")
	assert.Equal(t, http.StatusBadRequest, status)

	var plans int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	assert.Equal(t, int64(0), plans)
}

// GHSA-2w75-9qj8-fwwp — a member of another organisation posts a plan against
// this tenant's risk. The request is refused like a missing risk, and nothing
// is written in either tenant.
func TestCreateMitigation_Unauthorized_RiskOfAnotherTenant(t *testing.T) {
	app, db, _, _ := newCreateMitigationApp(t, false)
	victimRisk := seedRisk(t, db, uuid.New())

	status, body := postMitigation(t, app, victimRisk.String())

	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "risk not found", body["error"])

	var plans, subs int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	require.NoError(t, db.Table("mitigation_subactions").Count(&subs).Error)
	assert.Equal(t, int64(0), plans, "no plan may hang off another tenant's risk")
	assert.Equal(t, int64(0), subs)
}

// A well-formed id that matches no risk at all answers the same 404, so the
// response does not reveal whether the UUID exists in another tenant.
func TestCreateMitigation_NotFound_UnknownRisk(t *testing.T) {
	app, db, _, _ := newCreateMitigationApp(t, false)

	status, body := postMitigation(t, app, uuid.New().String())

	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "risk not found", body["error"])

	var plans int64
	require.NoError(t, db.Table("mitigations").Count(&plans).Error)
	assert.Equal(t, int64(0), plans)
}

// A soft-deleted risk of the caller's own tenant takes no new plan either.
func TestCreateMitigation_NotFound_DeletedRisk(t *testing.T) {
	app, db, _, riskID := newCreateMitigationApp(t, false)
	require.NoError(t, db.Exec(`UPDATE risks SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, riskID.String()).Error)

	status, _ := postMitigation(t, app, riskID.String())

	assert.Equal(t, http.StatusNotFound, status)
}
