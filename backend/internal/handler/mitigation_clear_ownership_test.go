// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// #1104 — PATCH /mitigations/:id with a slot set to null clears it. It used to
// answer 200 and keep the person on the plan.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/application/ownership"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
)

type clearOwnershipRig struct {
	app                    *fiber.App
	db                     *gorm.DB
	tenant, plan           uuid.UUID
	owner, assignee, revwr uuid.UUID
}

func newClearOwnershipRig(t *testing.T) *clearOwnershipRig {
	t.Helper()
	dsn := "file:mitigation_clear_" + uuid.New().String() + "?mode=memory&cache=private"
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

	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	// The service main wires. Clearing a slot needs no member lookup.
	previousOwnership := OwnershipServiceInstance()
	SetOwnershipService(ownership.NewService())
	t.Cleanup(func() { SetOwnershipService(previousOwnership) })

	r := &clearOwnershipRig{db: db, tenant: uuid.New(), plan: uuid.New(),
		owner: uuid.New(), assignee: uuid.New(), revwr: uuid.New()}
	owner, assignee, reviewer := r.owner, r.assignee, r.revwr
	require.NoError(t, db.Create(&domain.Mitigation{
		ID: r.plan, TenantID: r.tenant, RiskID: uuid.New(), Title: "Patch VPN",
		Status: domain.MitigationInProgress, Priority: "high", AssignedTo: domain.UUIDArray{assignee},
		Ownership: domain.Ownership{OwnerID: &owner, AssigneeID: &assignee, ReviewerID: &reviewer},
	}).Error)

	actor := uuid.New()
	r.app = fiber.New()
	r.app.Use(func(c *fiber.Ctx) error {
		tenant := r.tenant
		if c.Get("X-Test-Tenant") != "" {
			tenant = uuid.MustParse(c.Get("X-Test-Tenant"))
		}
		middleware.SetContext(c, &middleware.RequestContext{UserID: actor, OrganizationID: tenant})
		return c.Next()
	})
	r.app.Patch("/api/v1/mitigations/:id", UpdateMitigation)
	return r
}

func (r *clearOwnershipRig) patch(t *testing.T, body string, tenant ...uuid.UUID) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/mitigations/"+r.plan.String(), bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if len(tenant) == 1 {
		req.Header.Set("X-Test-Tenant", tenant[0].String())
	}
	res, err := r.app.Test(req, -1)
	require.NoError(t, err)
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode >= 400 {
		t.Logf("PATCH %s → %d %s", body, res.StatusCode, raw)
	}
	return res.StatusCode
}

func (r *clearOwnershipRig) slots(t *testing.T) (owner, assignee, reviewer *string, assignedTo string) {
	t.Helper()
	var row struct {
		OwnerID, AssigneeID, ReviewerID *string
		AssignedTo                      string
	}
	require.NoError(t, r.db.Raw(`SELECT owner_id, assignee_id, reviewer_id, assigned_to FROM mitigations WHERE id = ?`, r.plan.String()).Scan(&row).Error)
	return row.OwnerID, row.AssigneeID, row.ReviewerID, row.AssignedTo
}

func TestUpdateMitigation_ClearOwnership_Success(t *testing.T) {
	r := newClearOwnershipRig(t)

	require.Equal(t, http.StatusOK, r.patch(t, `{"assignee_id": null}`))
	owner, assignee, reviewer, assignedTo := r.slots(t)
	require.Nil(t, assignee, "the assignee is cleared")
	require.Equal(t, "[]", assignedTo, "with the legacy array")
	require.Equal(t, r.owner.String(), *owner, "slots absent from the payload are left alone")
	require.Equal(t, r.revwr.String(), *reviewer)

	require.Equal(t, http.StatusOK, r.patch(t, `{"owner_id": null, "reviewer_id": null}`))
	owner, _, reviewer, _ = r.slots(t)
	require.Nil(t, owner)
	require.Nil(t, reviewer)
}

// Another tenant's plan: 404, nothing cleared.
func TestUpdateMitigation_ClearOwnership_NotFound(t *testing.T) {
	r := newClearOwnershipRig(t)
	require.Equal(t, http.StatusNotFound, r.patch(t, `{"assignee_id": null}`, uuid.New()))
	_, assignee, _, _ := r.slots(t)
	require.NotNil(t, assignee)
}

// A title-only edit does not touch the slots.
func TestUpdateMitigation_ClearOwnership_UnrelatedEditKeepsSlots(t *testing.T) {
	r := newClearOwnershipRig(t)
	require.Equal(t, http.StatusOK, r.patch(t, `{"title": "Patch VPN now"}`))
	owner, assignee, reviewer, _ := r.slots(t)
	require.NotNil(t, owner)
	require.NotNil(t, assignee)
	require.NotNil(t, reviewer)
}
