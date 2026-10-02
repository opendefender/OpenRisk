// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// assetImportApp mounts POST /assets/import as main.go does — behind
// assets:create — over a real sqlite database, so the transaction is real.
type assetImportApp struct {
	app    *fiber.App
	db     *gorm.DB
	tenant *uuid.UUID
	perms  *[]string
}

func newAssetImportApp(t *testing.T) *assetImportApp {
	t.Helper()
	dsn := "file:asset_import_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
		criticality TEXT NOT NULL DEFAULT 'MEDIUM', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "assets", &domain.Asset{}))

	h := &assetImportApp{db: db, tenant: new(uuid.UUID), perms: &[]string{"assets:create"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: *h.tenant})
		c.Locals("permissions", *h.perms)
		return c.Next()
	})
	handler := NewAssetImportHandler(assetuc.NewImportAssetsUseCase(repository.RunAssetTx(db), repository.ListAssetNames(db)))
	app.Post("/api/v1/assets/import", middleware.RequirePermission("assets:create"), handler.ImportAssets)
	h.app = app
	return h
}

func (h *assetImportApp) upload(t *testing.T, filename, content string) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, _ = part.Write([]byte(content))
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/import", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}

func (h *assetImportApp) assetsOf(t *testing.T, tenant uuid.UUID) []domain.Asset {
	t.Helper()
	var out []domain.Asset
	require.NoError(t, h.db.Where("tenant_id = ?", tenant).Order("name").Find(&out).Error)
	return out
}

const validAssetImport = "name,type,criticality,owner\n" +
	"Core banking DB,Database,CRITICAL,DSI\n" +
	"Kiosk,Laptop,low,\n"

func TestAssetImportHTTP_Success(t *testing.T) {
	h := newAssetImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant

	status, body := h.upload(t, "inventory.csv", validAssetImport)
	require.Equal(t, fiber.StatusOK, status, "%v", body)
	require.EqualValues(t, 2, body["created"])
	require.Len(t, body["asset_ids"], 2)

	rows := h.assetsOf(t, tenant)
	require.Len(t, rows, 2)
	require.Equal(t, "Core banking DB", rows[0].Name)
	require.Equal(t, domain.CriticalityCritical, rows[0].Criticality)
	require.Equal(t, "IMPORT", rows[0].Source)
	require.Equal(t, domain.CriticalityLow, rows[1].Criticality)

	// Re-importing the same file never doubles the inventory.
	status, body = h.upload(t, "inventory.csv", validAssetImport)
	require.Equal(t, fiber.StatusUnprocessableEntity, status, "%v", body)
	require.Len(t, h.assetsOf(t, tenant), 2)
}

func TestAssetImportHTTP_Validation_NothingPersisted(t *testing.T) {
	h := newAssetImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant

	status, body := h.upload(t, "inventory.csv", validAssetImport+"Printer,Device,urgent,\n")
	require.Equal(t, fiber.StatusUnprocessableEntity, status, "%v", body)
	require.EqualValues(t, 0, body["created"])
	errs, _ := body["errors"].([]any)
	require.Len(t, errs, 1)
	first, _ := errs[0].(map[string]any)
	require.EqualValues(t, 4, first["line"])
	require.Equal(t, "criticality", first["column"])
	require.Equal(t, "invalid_criticality", first["code"])
	require.Empty(t, h.assetsOf(t, tenant), "one invalid row means zero rows persisted")
}

func TestAssetImportHTTP_Unauthorized(t *testing.T) {
	h := newAssetImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant
	*h.perms = []string{"assets:read"}

	status, _ := h.upload(t, "inventory.csv", validAssetImport)
	require.Equal(t, fiber.StatusForbidden, status)
	require.Empty(t, h.assetsOf(t, tenant))
}

func TestAssetImportHTTP_NotFound_NoFile(t *testing.T) {
	h := newAssetImportApp(t)
	*h.tenant = uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/import", nil)
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	status, _ := h.upload(t, "inventory.xlsx", validAssetImport)
	require.Equal(t, fiber.StatusBadRequest, status, "only CSV is accepted")
}

func TestAssetImportHTTP_RowsLandOnlyInCallersTenant(t *testing.T) {
	h := newAssetImportApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	*h.tenant = tenantB
	status, body := h.upload(t, "b.csv", "name\nKiosk\n")
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	// Tenant B already has "Kiosk"; that must not block tenant A's own.
	*h.tenant = tenantA
	status, body = h.upload(t, "a.csv", validAssetImport)
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	require.Len(t, h.assetsOf(t, tenantA), 2)
	bRows := h.assetsOf(t, tenantB)
	require.Len(t, bRows, 1, "tenant A's import must not land in tenant B")
	require.Equal(t, "Kiosk", bRows[0].Name)
}
