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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	applicationrisk "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/pkg/crq"
)

// importApp mounts POST /risks/import exactly as main.go does — behind
// risks:create — over a real sqlite database, so the transaction is real.
type importApp struct {
	app    *fiber.App
	db     *gorm.DB
	tenant *uuid.UUID
	perms  *[]string
}

func newImportApp(t *testing.T) *importApp {
	t.Helper()

	dsn := "file:risk_import_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&UserT{}, &MitigationT{}, &RiskHistoryT{}))
	createRisksTable(t, db)
	// The import resolves assets tenant-scoped and links them, so the assets
	// table needs its tenant and criticality columns, and the join table.
	require.NoError(t, db.Exec(`CREATE TABLE assets (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
		criticality TEXT NOT NULL DEFAULT 'MEDIUM', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "assets", &domain.Asset{}))
	require.NoError(t, db.Exec(`CREATE TABLE risk_assets (risk_id TEXT NOT NULL, asset_id TEXT NOT NULL)`).Error)

	orig := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = orig })

	h := &importApp{db: db, tenant: new(uuid.UUID), perms: &[]string{"risks:create"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: *h.tenant})
		c.Locals("permissions", *h.perms)
		return c.Next()
	})

	riskRepo := repository.NewGormRiskRepository(db)
	handler := NewRiskHandler(
		applicationrisk.NewCreateRiskUseCase(riskRepo),
		applicationrisk.NewGetRiskUseCase(riskRepo),
		applicationrisk.NewListRisksUseCase(riskRepo),
		applicationrisk.NewUpdateRiskUseCase(riskRepo),
		applicationrisk.NewDeleteRiskUseCase(riskRepo),
		applicationrisk.NewMarkRiskReviewedUseCase(riskRepo),
		applicationrisk.NewTransitionRiskStateUseCase(riskRepo),
		nil,
		crq.NewQuantifier(0, crq.Reference{}),
	).WithImport(applicationrisk.NewImportRisksUseCase(repository.RunRiskTx(db)).
		WithAssets(repository.ListImportAssetRefs(db)))

	app.Post("/api/v1/risks/import", middleware.RequirePermission("risks:create"), handler.ImportRisks)
	h.app = app
	return h
}

func (h *importApp) upload(t *testing.T, filename, content string) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, _ = part.Write([]byte(content))
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks/import", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}

func (h *importApp) risksOf(t *testing.T, tenant uuid.UUID) []domain.Risk {
	t.Helper()
	var out []domain.Risk
	require.NoError(t, h.db.Where("tenant_id = ?", tenant).Find(&out).Error)
	return out
}

const validImport = "title,description,probability,impact,tags\n" +
	"Phishing,Credential theft,0.6,8,email;people\n" +
	"Ransomware,,0.3,10,\n" +
	"Supplier outage,,0.2,5,\n"

func TestRiskImportHTTP_Success(t *testing.T) {
	h := newImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant

	status, body := h.upload(t, "register.csv", validImport)
	require.Equal(t, fiber.StatusOK, status, "%v", body)
	require.EqualValues(t, 3, body["created"])
	require.EqualValues(t, 0, body["rejected"])
	require.Len(t, body["risk_ids"], 3)
	require.Empty(t, body["errors"])

	rows := h.risksOf(t, tenant)
	require.Len(t, rows, 3, "every CSV row is persisted")
	for _, r := range rows {
		require.Equal(t, domain.SourceImport, r.Source)
		require.InDelta(t, r.Probability*r.Impact, r.Score, 1e-9, "each row is scored on create")
		require.Equal(t, domain.CriticalityFromScore(r.Score), r.Criticality)
	}
}

func TestRiskImportHTTP_Validation_NothingPersisted(t *testing.T) {
	h := newImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant

	status, body := h.upload(t, "register.csv", validImport+"Broken,,2,5,\n")
	require.Equal(t, fiber.StatusUnprocessableEntity, status, "%v", body)
	require.EqualValues(t, 0, body["created"])
	require.EqualValues(t, 1, body["rejected"])
	errs, _ := body["errors"].([]any)
	require.Len(t, errs, 1)
	first, _ := errs[0].(map[string]any)
	require.EqualValues(t, 5, first["line"])
	require.Equal(t, "probability", first["column"])

	require.Empty(t, h.risksOf(t, tenant), "one invalid row means zero rows persisted")
}

func TestRiskImportHTTP_Unauthorized(t *testing.T) {
	h := newImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant
	*h.perms = []string{"risks:read"}

	status, _ := h.upload(t, "register.csv", validImport)
	require.Equal(t, fiber.StatusForbidden, status)
	require.Empty(t, h.risksOf(t, tenant))
}

func TestRiskImportHTTP_NotFound_NoFile(t *testing.T) {
	h := newImportApp(t)
	*h.tenant = uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/risks/import", nil)
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	status, _ := h.upload(t, "register.xlsx", validImport)
	require.Equal(t, fiber.StatusBadRequest, status, "only CSV is accepted")
}

func TestRiskImportHTTP_RowsLandOnlyInCallersTenant(t *testing.T) {
	h := newImportApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()

	*h.tenant = tenantB
	status, body := h.upload(t, "b.csv", "title,probability,impact\nB own,0.5,5\n")
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	*h.tenant = tenantA
	status, body = h.upload(t, "a.csv", validImport)
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	require.Len(t, h.risksOf(t, tenantA), 3)
	bRows := h.risksOf(t, tenantB)
	require.Len(t, bRows, 1, "tenant A's import must not land in tenant B")
	require.Equal(t, "B own", bRows[0].Title)
}

// The "assets" column resolves names inside the caller's tenant only, links
// through the same transaction, and a name from another tenant refuses the file.
func TestRiskImportHTTP_AssetsResolveOnlyInCallersTenant(t *testing.T) {
	h := newImportApp(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	ownAsset, foreignAsset := uuid.New(), uuid.New()
	require.NoError(t, h.db.Exec(`INSERT INTO assets (id, tenant_id, name, criticality) VALUES (?, ?, 'Core DB', 'CRITICAL'), (?, ?, 'Payroll', 'HIGH')`,
		ownAsset, tenantA, foreignAsset, tenantB).Error)

	*h.tenant = tenantA
	status, body := h.upload(t, "x.csv", "title,probability,impact,assets\nLeak,0.5,6,Payroll\n")
	require.Equal(t, fiber.StatusUnprocessableEntity, status, "%v", body)
	require.Empty(t, h.risksOf(t, tenantA))

	status, body = h.upload(t, "ok.csv", "title,probability,impact,assets\nLeak,0.5,6,core db\nOther,0.5,6,\n")
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	var links []struct{ RiskID, AssetID string }
	require.NoError(t, h.db.Raw(`SELECT risk_id, asset_id FROM risk_assets`).Scan(&links).Error)
	require.Len(t, links, 1)
	assert.Equal(t, ownAsset.String(), links[0].AssetID)

	scores := map[string]float64{}
	for _, r := range h.risksOf(t, tenantA) {
		scores[r.Title] = r.Score
	}
	assert.Greater(t, scores["Leak"], scores["Other"], "the critical asset is in the stored score")
}
