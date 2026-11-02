// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// #1213 — assign_mitigation resolves its template plan inside the caller's
// tenant before the job is queued. These tests pin the HTTP answer: 400 when
// the id is missing or malformed, 404 when it is not a plan of this tenant.
// No case gets as far as queuing a job, so no worker goroutine is started.

func newBulkAssignHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:bulk_handler_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// Only the columns the template lookup reads or filters on.
	require.NoError(t, db.Exec(`CREATE TABLE mitigations (
		id TEXT PRIMARY KEY, tenant_id TEXT, risk_id TEXT, title TEXT,
		description TEXT, priority TEXT, deleted_at DATETIME
	);`).Error)
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	return db
}

func postBulkAssign(t *testing.T, tenant uuid.UUID, body string) (int, map[string]interface{}) {
	t.Helper()
	h := NewBulkOperationHandler()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	user := uuid.New()
	perms := []string{"mitigations:update"}
	app.Post("/bulk-operations", func(c *fiber.Ctx) error {
		c.Locals("permissions", perms)
		c.Locals("user", &authpkg.Claims{Sub: user, TenantID: tenant, Permissions: perms})
		middleware.SetContext(c, &middleware.RequestContext{UserID: user, OrganizationID: tenant})
		return c.Next()
	}, h.CreateBulkOperation)

	req := httptest.NewRequest("POST", "/bulk-operations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return resp.StatusCode, out
}

func TestBulkAssignHandler_MissingMitigationIDIs400(t *testing.T) {
	newBulkAssignHandlerDB(t)
	for _, body := range []string{
		`{"operation_type":"assign_mitigation"}`,
		`{"operation_type":"assign_mitigation","update_data":{"mitigation_id":"not-a-uuid"}}`,
	} {
		status, _ := postBulkAssign(t, uuid.New(), body)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
	}
}

func TestBulkAssignHandler_NotFound(t *testing.T) {
	newBulkAssignHandlerDB(t)
	status, out := postBulkAssign(t, uuid.New(),
		`{"operation_type":"assign_mitigation","mitigation_id":"`+uuid.NewString()+`"}`)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Contains(t, out["error"], "not_found")
}

// Another tenant's plan answers exactly like a missing one.
func TestBulkAssignHandler_Unauthorized(t *testing.T) {
	db := newBulkAssignHandlerDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	foreign := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO mitigations (id, tenant_id, risk_id, title) VALUES (?, ?, ?, 'Foreign plan')`,
		foreign.String(), tenantB.String(), uuid.NewString()).Error)

	status, _ := postBulkAssign(t, tenantA,
		`{"operation_type":"assign_mitigation","update_data":{"mitigation_id":"`+foreign.String()+`"}}`)
	assert.Equal(t, fiber.StatusNotFound, status)
}
