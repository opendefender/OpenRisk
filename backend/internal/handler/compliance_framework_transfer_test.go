// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	applicationcompliance "github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// buildFrameworkTransferApp wires the two routes with the exact guards main.go
// uses (middleware.RequirePermission on the c.Locals("permissions") list).
func buildFrameworkTransferApp(db *gorm.DB, tenant uuid.UUID, perms []string) *fiber.App {
	repo := repository.NewGormComplianceRepository(db)
	h := (&ComplianceHandler{}).WithFrameworkTransfer(
		applicationcompliance.NewExportFrameworkUseCase(repo),
		applicationcompliance.NewImportFrameworkUseCase(repo),
	)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		c.Locals("permissions", perms)
		return c.Next()
	})
	read := middleware.RequirePermission("compliance:frameworks:read")
	create := middleware.RequirePermission("compliance:frameworks:create")
	api := app.Group("/api/v1")
	api.Post("/compliance/frameworks/import", create, h.ImportFramework)
	api.Get("/compliance/frameworks/:frameworkId/export", read, h.ExportFramework)
	return app
}

func doReq(t *testing.T, app *fiber.App, method, path string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, out
}

const transferDoc = `{"schema_version":1,"framework":{"name":"Circulaire 01","version":"2026","description":"Desc"},
"controls":[{"reference_code":"A.2","name":"Deux","description":"d2","source_reference":"art. 2"},
{"reference_code":"A.1","name":"Un","description":"d1","source_reference":"art. 1"}]}`

var allPerms = []string{"compliance:frameworks:read", "compliance:frameworks:create"}

func importOK(t *testing.T, app *fiber.App, doc string) map[string]any {
	t.Helper()
	code, _, out := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(doc))
	require.Equal(t, 201, code, string(out))
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	return m
}

func TestImportExportFramework_RoundTrip(t *testing.T) {
	db := setupComplianceSchema(t)
	tenant := uuid.New()
	app := buildFrameworkTransferApp(db, tenant, allPerms)

	created := importOK(t, app, transferDoc)
	assert.EqualValues(t, 2, created["controls_created"])
	assert.Equal(t, tenant.String(), created["tenant_id"])
	assert.Equal(t, "", created["catalog_key"])

	code, hdr, out := doReq(t, app, http.MethodGet, "/api/v1/compliance/frameworks/"+created["id"].(string)+"/export", nil)
	require.Equal(t, 200, code, string(out))
	assert.Contains(t, hdr.Get("Content-Type"), "application/json")
	assert.Contains(t, hdr.Get("Content-Disposition"), "attachment")
	assert.Contains(t, hdr.Get("Content-Disposition"), ".json")

	var exported, original applicationcompliance.FrameworkDocument
	require.NoError(t, json.Unmarshal(out, &exported))
	require.NoError(t, json.Unmarshal([]byte(transferDoc), &original))
	assert.Equal(t, original.Framework, exported.Framework)
	assert.Equal(t, 1, exported.SchemaVersion)
	require.Len(t, exported.Controls, 2)
	assert.Equal(t, "A.1", exported.Controls[0].ReferenceCode, "deterministic order")
	assert.ElementsMatch(t, original.Controls, exported.Controls)

	// Criterion 1: no id, tenant, status or evidence leaks into the file.
	for _, forbidden := range []string{"tenant_id", `"id"`, "status", "evidence", "framework_id"} {
		assert.NotContains(t, string(out), forbidden)
	}

	// Re-importing the exported document into another tenant is lossless.
	other := buildFrameworkTransferApp(db, uuid.New(), allPerms)
	again := importOK(t, other, string(out))
	_, _, out2 := doReq(t, other, http.MethodGet, "/api/v1/compliance/frameworks/"+again["id"].(string)+"/export", nil)
	assert.JSONEq(t, string(out), string(out2))
}

func TestImportFramework_IgnoresTenantAndStatusFromFile(t *testing.T) {
	db := setupComplianceSchema(t)
	tenant := uuid.New()
	app := buildFrameworkTransferApp(db, tenant, allPerms)
	doc := `{"schema_version":1,"tenant_id":"` + uuid.NewString() + `","framework":{"name":"F","tenant_id":"` + uuid.NewString() + `"},
"controls":[{"reference_code":"A","name":"a","status":"implemented","tenant_id":"` + uuid.NewString() + `"}]}`
	created := importOK(t, app, doc)
	assert.Equal(t, tenant.String(), created["tenant_id"])

	var n int64
	require.NoError(t, db.Table("compliance_controls").Where("tenant_id = ? AND status = ?", tenant, "not_implemented").Count(&n).Error)
	assert.EqualValues(t, 1, n)
	require.NoError(t, db.Table("compliance_controls").Where("tenant_id <> ?", tenant).Count(&n).Error)
	assert.Zero(t, n)
}

func TestImportFramework_ValidationBody(t *testing.T) {
	db := setupComplianceSchema(t)
	app := buildFrameworkTransferApp(db, uuid.New(), allPerms)
	doc := `{"schema_version":1,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"},{"reference_code":"","name":"b"}]}`
	code, _, out := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(doc))
	require.Equal(t, 400, code)
	var body struct {
		Error  string `json:"error"`
		Errors []struct {
			Index   *int   `json:"index"`
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(out, &body))
	assert.NotEmpty(t, body.Error)
	require.Len(t, body.Errors, 1)
	require.NotNil(t, body.Errors[0].Index)
	assert.Equal(t, 1, *body.Errors[0].Index)
	assert.Equal(t, "reference_code", body.Errors[0].Field)
	var n int64
	require.NoError(t, db.Table("compliance_frameworks").Count(&n).Error)
	assert.Zero(t, n, "nothing written")

	// Framework-level problem: index is null.
	code, _, out = doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(`{"schema_version":2,"framework":{"name":"F"},"controls":[{"reference_code":"A","name":"a"}]}`))
	require.Equal(t, 400, code)
	assert.Contains(t, string(out), `"index":null`)
}

func TestImportFramework_OversizeBodyIs400(t *testing.T) {
	app := buildFrameworkTransferApp(setupComplianceSchema(t), uuid.New(), allPerms)
	big := `{"schema_version":1,"pad":"` + strings.Repeat("a", 2<<20) + `"}`
	code, _, _ := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(big))
	assert.Equal(t, 400, code)
}

func TestImportFramework_Conflict409(t *testing.T) {
	app := buildFrameworkTransferApp(setupComplianceSchema(t), uuid.New(), allPerms)
	importOK(t, app, transferDoc)
	code, _, _ := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(transferDoc))
	assert.Equal(t, 409, code)
}

func TestImportFramework_Unauthorized_NoTenant(t *testing.T) {
	app := buildFrameworkTransferApp(setupComplianceSchema(t), uuid.Nil, allPerms)
	code, _, _ := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(transferDoc))
	assert.Equal(t, 401, code)
	code, _, _ = doReq(t, app, http.MethodGet, "/api/v1/compliance/frameworks/"+uuid.NewString()+"/export", nil)
	assert.Equal(t, 401, code)
}

func TestExportFramework_CrossTenantIsIdenticalToMissing(t *testing.T) {
	db := setupComplianceSchema(t)
	owner := buildFrameworkTransferApp(db, uuid.New(), allPerms)
	created := importOK(t, owner, transferDoc)
	intruder := buildFrameworkTransferApp(db, uuid.New(), allPerms)

	codeForeign, _, bodyForeign := doReq(t, intruder, http.MethodGet, "/api/v1/compliance/frameworks/"+created["id"].(string)+"/export", nil)
	codeMissing, _, bodyMissing := doReq(t, intruder, http.MethodGet, "/api/v1/compliance/frameworks/"+uuid.NewString()+"/export", nil)
	assert.Equal(t, 404, codeForeign)
	assert.Equal(t, 404, codeMissing)
	assert.JSONEq(t, string(bodyMissing), string(bodyForeign), "a foreign id must not be distinguishable from a missing one")
}

func TestFrameworkTransferRoutes_RBAC(t *testing.T) {
	db := setupComplianceSchema(t)
	tenant := uuid.New()
	created := importOK(t, buildFrameworkTransferApp(db, tenant, allPerms), transferDoc)
	exportPath := "/api/v1/compliance/frameworks/" + created["id"].(string) + "/export"

	noCreate := buildFrameworkTransferApp(db, tenant, []string{"compliance:frameworks:read"})
	code, _, _ := doReq(t, noCreate, http.MethodPost, "/api/v1/compliance/frameworks/import", []byte(`{"schema_version":1,"framework":{"name":"Other"},"controls":[{"reference_code":"A","name":"a"}]}`))
	assert.Equal(t, 403, code, "import without compliance:frameworks:create")
	var n int64
	require.NoError(t, db.Table("compliance_frameworks").Count(&n).Error)
	assert.EqualValues(t, 1, n, "a forbidden import writes nothing")
	code, _, _ = doReq(t, noCreate, http.MethodGet, exportPath, nil)
	assert.Equal(t, 200, code, "read permission alone may export")

	noRead := buildFrameworkTransferApp(db, tenant, []string{"compliance:frameworks:create"})
	code, _, _ = doReq(t, noRead, http.MethodGet, exportPath, nil)
	assert.Equal(t, 403, code, "export without compliance:frameworks:read")

	none := buildFrameworkTransferApp(db, tenant, nil)
	code, _, _ = doReq(t, none, http.MethodGet, exportPath, nil)
	assert.Equal(t, 403, code)
}
