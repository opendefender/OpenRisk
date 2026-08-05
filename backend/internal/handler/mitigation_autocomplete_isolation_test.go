// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
)

// Probe for POST /scanner/mitigations/auto-complete.
//
// This endpoint took its tenant from the request body while its only other
// gate was a hardcoded key compiled into the source. Any authenticated user
// could therefore mark a sub-action complete in any other tenant, simply by
// naming that tenant in the payload.
//
// In a GRC product that is worse than it sounds: "remediation completed" is
// compliance evidence, so the write falsifies another organisation's audit
// posture rather than merely corrupting a row.

// DDL mirroring the columns the mitigation repository reads. Kept explicit
// because AutoMigrate cannot render the Postgres defaults on the domain models.
const mitigationPlansDDL = `
CREATE TABLE IF NOT EXISTS mitigations (
	id TEXT PRIMARY KEY,
	tenant_id TEXT NOT NULL,
	risk_id TEXT,
	title TEXT,
	description TEXT,
	status TEXT DEFAULT 'PLANNED',
	priority TEXT DEFAULT 'medium',
	assigned_to BLOB,
	progress INTEGER DEFAULT 0,
	created_by TEXT,
	approved_by TEXT,
	approved_at DATETIME,
	source TEXT DEFAULT 'manual',
	auto_detected_at DATETIME,
	scanner_config_id TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME,
	organization_id TEXT,
	assignee TEXT,
	cost INTEGER DEFAULT 1,
	mitigation_time INTEGER DEFAULT 1,
	due_date DATETIME
);`

const mitigationSubActionsDDL = `
CREATE TABLE IF NOT EXISTS mitigation_subactions (
	id TEXT PRIMARY KEY,
	mitigation_id TEXT NOT NULL,
	title TEXT,
	description TEXT,
	completed INTEGER DEFAULT 0,
	completed_at DATETIME,
	completed_by TEXT,
	completed_source TEXT,
	auto_detected_at DATETIME,
	depends_on TEXT,
	"order" INTEGER DEFAULT 0,
	evidence TEXT,
	scanner_job_id TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME
);`

type autoCompleteProbe struct {
	app    *fiber.App
	db     *gorm.DB
	acting *uuid.UUID
}

func newAutoCompleteProbe(t *testing.T) *autoCompleteProbe {
	t.Helper()

	dsn := "file:autocomplete_isolation_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// Explicit DDL rather than AutoMigrate: the domain models carry Postgres
	// defaults (gen_random_uuid(), ::jsonb) that sqlite rejects. The columns and
	// table names below are the ones the repository actually queries, so the
	// probe exercises the real lookup instead of a shape invented for the test.
	require.NoError(t, db.Exec(mitigationPlansDDL).Error)
	require.NoError(t, db.Exec(mitigationSubActionsDDL).Error)

	orig := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = orig })

	p := &autoCompleteProbe{db: db, acting: new(uuid.UUID)}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{
			UserID:         uuid.New(),
			OrganizationID: *p.acting,
		})
		return c.Next()
	})
	app.Post("/api/v1/scanner/mitigations/auto-complete", AutoCompleteMitigationSubAction)
	p.app = app

	return p
}

func (p *autoCompleteProbe) as(tenant uuid.UUID) { *p.acting = tenant }

// seed creates a plan and one pending sub-action owned by the given tenant.
func (p *autoCompleteProbe) seed(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()

	planID, subID := uuid.New(), uuid.New()

	require.NoError(t, p.db.Exec(
		`INSERT INTO mitigations (id, tenant_id, risk_id, title, status, assigned_to, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'PLANNED', ?, ?, ?, ?)`,
		planID.String(), tenant.String(), uuid.New().String(), "Remediation plan",
		// UUIDArray.Scan requires []byte; sqlite hands back a string for a TEXT
		// column, so the value is written as a BLOB.
		[]byte("[]"),
		uuid.New().String(), time.Now(), time.Now()).Error)

	require.NoError(t, p.db.Exec(
		`INSERT INTO mitigation_subactions (id, mitigation_id, title, completed, created_at, updated_at)
		 VALUES (?, ?, ?, 0, ?, ?)`,
		subID.String(), planID.String(), "Patch the server", time.Now(), time.Now()).Error)

	return subID
}

func (p *autoCompleteProbe) post(t *testing.T, body map[string]any, apiKey string) int {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scanner/mitigations/auto-complete", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Internal-API-Key", apiKey)
	}

	resp, err := p.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// completed reports whether the sub-action has been marked done.
func (p *autoCompleteProbe) completed(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var completed int
	require.NoError(t, p.db.Raw(`SELECT completed FROM mitigation_subactions WHERE id = ?`, id.String()).Scan(&completed).Error)
	return completed == 1
}

// TestAutoComplete_CannotCompleteAnotherTenantsSubAction is the regression test:
// tenant A naming tenant B in the payload must not reach B's data.
func TestAutoComplete_CannotCompleteAnotherTenantsSubAction(t *testing.T) {
	p := newAutoCompleteProbe(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	victimID := p.seed(t, tenantB)

	p.as(tenantA)
	status := p.post(t, map[string]any{
		// The old handler trusted this field over the session.
		"tenant_id":      tenantB.String(),
		"sub_action_id":  victimID.String(),
		"scanner_job_id": "job-1",
		"evidence":       "forged",
	}, "internal-scanner-key") // the key that was compiled into the source

	require.NotEqual(t, fiber.StatusOK, status,
		"tenant A completed tenant B's sub-action by naming B in the payload")
	require.False(t, p.completed(t, victimID),
		"tenant B's sub-action was mutated across tenants")
}

// TestAutoComplete_WorksWithinOwnTenant guards the other direction: the fix must
// not break the legitimate scanner flow it exists to serve.
func TestAutoComplete_WorksWithinOwnTenant(t *testing.T) {
	p := newAutoCompleteProbe(t)
	tenant := uuid.New()
	subID := p.seed(t, tenant)

	p.as(tenant)
	status := p.post(t, map[string]any{
		"sub_action_id":  subID.String(),
		"scanner_job_id": "job-1",
		"evidence":       "scan confirmed the port is closed",
	}, "")

	require.Equal(t, fiber.StatusOK, status, "the scanner must still be able to auto-complete within its own tenant")
	require.True(t, p.completed(t, subID))
}

// TestAutoComplete_IgnoresPayloadTenant pins the root cause rather than the
// symptom: even a well-formed request must derive the tenant from the session,
// so a future refactor cannot quietly reintroduce the payload field.
func TestAutoComplete_IgnoresPayloadTenant(t *testing.T) {
	p := newAutoCompleteProbe(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	ownID := p.seed(t, tenantA)
	victimID := p.seed(t, tenantB)

	p.as(tenantA)
	// Claim to be B while acting as A, targeting A's own sub-action. If the
	// payload still won, this would fail to resolve; the session must decide.
	status := p.post(t, map[string]any{
		"tenant_id":     tenantB.String(),
		"sub_action_id": ownID.String(),
	}, "")

	require.Equal(t, fiber.StatusOK, status, "the session tenant must decide, not the payload")
	require.True(t, p.completed(t, ownID))
	require.False(t, p.completed(t, victimID), "B's data must be untouched")
}

// TestAutoComplete_RequiresAuthenticatedTenant pins fail-closed behaviour when
// no tenant can be resolved.
func TestAutoComplete_RequiresAuthenticatedTenant(t *testing.T) {
	p := newAutoCompleteProbe(t)
	tenant := uuid.New()
	subID := p.seed(t, tenant)

	p.as(uuid.Nil)
	status := p.post(t, map[string]any{"sub_action_id": subID.String()}, "")

	require.Equal(t, fiber.StatusUnauthorized, status, "an unresolved tenant must be refused, not defaulted")
	require.False(t, p.completed(t, subID))
}
