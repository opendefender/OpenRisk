// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

	applicationcompliance "github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/storage"
)

// setupComplianceSchema creates a shared in-memory SQLite DB with the
// compliance tables + unique indexes, mirroring
// backend/migrations/0028_create_compliance_schema.up.sql.
func setupComplianceSchema(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_frameworks (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL DEFAULT '',
			description TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX idx_compliance_frameworks_name_version
			ON compliance_frameworks(name, version) WHERE deleted_at IS NULL;
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_controls (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, framework_id TEXT NOT NULL,
			reference_code TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, description TEXT,
			status TEXT NOT NULL DEFAULT 'not_implemented',
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX idx_compliance_controls_tenant_fw_ref
			ON compliance_controls(tenant_id, framework_id, reference_code)
			WHERE deleted_at IS NULL AND reference_code != '';
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE control_evidences (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, control_id TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '', description TEXT,
			uploaded_by TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)

	return db
}

// buildComplianceApp wires a real handler (real repo, real use cases, real
// local storage) behind a fake auth middleware that stamps both the tenant
// context (used by the handler) and the role claim (used by
// middleware.RequirePermissions) — so RBAC is exercised for real, not
// bypassed.
func buildComplianceApp(t *testing.T, db *gorm.DB, store storage.Storage, tenantID uuid.UUID, role string) *fiber.App {
	t.Helper()

	ps := service.NewPermissionService()
	require.NoError(t, ps.InitializeDefaultRoles())

	repo := repository.NewGormComplianceRepository(db)
	h := NewComplianceHandler(
		applicationcompliance.NewCreateFrameworkUseCase(repo),
		applicationcompliance.NewGetFrameworkUseCase(repo),
		applicationcompliance.NewListFrameworksUseCase(repo),
		applicationcompliance.NewCreateControlUseCase(repo),
		applicationcompliance.NewGetControlUseCase(repo),
		applicationcompliance.NewListControlsUseCase(repo),
		applicationcompliance.NewUpdateControlUseCase(repo),
		applicationcompliance.NewDeleteControlUseCase(repo),
		applicationcompliance.NewCreateEvidenceUseCase(repo, store),
		applicationcompliance.NewListEvidencesUseCase(repo),
		applicationcompliance.NewDeleteEvidenceUseCase(repo, store),
		applicationcompliance.NewDownloadEvidenceUseCase(repo, store),
		applicationcompliance.NewGetComplianceProgressUseCase(repo),
	)

	app := fiber.New()
	userID := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: userID, OrganizationID: tenantID})
		c.Locals("user", &domain.UserClaims{ID: userID, RoleName: role})
		return c.Next()
	})

	frameworkRead := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceFramework, Action: domain.PermissionRead})
	frameworkCreate := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceFramework, Action: domain.PermissionCreate})
	controlRead := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceControl, Action: domain.PermissionRead})
	controlCreate := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceControl, Action: domain.PermissionCreate})
	controlUpdate := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceControl, Action: domain.PermissionUpdate})
	controlDelete := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceControl, Action: domain.PermissionDelete})
	evidenceRead := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceEvidence, Action: domain.PermissionRead})
	evidenceCreate := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceEvidence, Action: domain.PermissionCreate})
	evidenceDelete := middleware.RequirePermissions(ps, domain.Permission{Resource: domain.PermissionResourceComplianceEvidence, Action: domain.PermissionDelete})

	api := app.Group("/api/v1")
	api.Get("/compliance/frameworks", frameworkRead, h.ListFrameworks)
	api.Post("/compliance/frameworks", frameworkCreate, h.CreateFramework)
	api.Get("/compliance/frameworks/:frameworkId", frameworkRead, h.GetFramework)
	api.Get("/compliance/frameworks/:frameworkId/progress", controlRead, h.GetProgress)
	api.Get("/compliance/frameworks/:frameworkId/controls", controlRead, h.ListControls)
	api.Post("/compliance/frameworks/:frameworkId/controls", controlCreate, h.CreateControl)
	api.Get("/compliance/controls/:controlId", controlRead, h.GetControl)
	api.Patch("/compliance/controls/:controlId", controlUpdate, h.UpdateControl)
	api.Delete("/compliance/controls/:controlId", controlDelete, h.DeleteControl)
	api.Get("/compliance/controls/:controlId/evidences", evidenceRead, h.ListEvidences)
	api.Post("/compliance/controls/:controlId/evidences", evidenceCreate, h.CreateEvidence)
	api.Get("/compliance/evidences/:evidenceId/download", evidenceRead, h.DownloadEvidence)
	api.Delete("/compliance/evidences/:evidenceId", evidenceDelete, h.DeleteEvidence)

	return app
}

func mustJSON(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

func newMultipartFileRequest(t *testing.T, url, fieldFilename, description, content string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fieldFilename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("description", description))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// TestComplianceE2EFlow is the automated proof of ROADMAP.md's M1
// acceptance criterion: a tenant can walk a framework end-to-end via the
// API (admin creates the framework, analyst instantiates/works a control,
// attaches and downloads evidence, and progress reflects reality).
func TestComplianceE2EFlow(t *testing.T) {
	db := setupComplianceSchema(t)
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	tenantID := uuid.New()

	adminApp := buildComplianceApp(t, db, store, tenantID, "admin")
	analystApp := buildComplianceApp(t, db, store, tenantID, "analyst")

	// 1. Admin creates a framework.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks",
		mustJSON(t, map[string]string{"name": "ISO 27001", "version": "2022"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var fw domain.ComplianceFramework
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fw))
	resp.Body.Close()

	// 2. Analyst instantiates a control under that framework.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/controls",
		mustJSON(t, map[string]string{"reference_code": "A.5.1.1", "name": "Policies for information security"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var control domain.ComplianceControl
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&control))
	resp.Body.Close()
	require.Equal(t, domain.ControlStatusNotImplemented, control.Status)

	// 3. Analyst changes the control's status.
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/compliance/controls/"+control.ID.String(),
		mustJSON(t, map[string]string{"status": "implemented"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()

	// 4. Analyst uploads evidence (real multipart file).
	req = newMultipartFileRequest(t, "/api/v1/compliance/controls/"+control.ID.String()+"/evidences",
		"audit-report.pdf", "Q3 internal audit", "pdf bytes here")
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 201, resp.StatusCode)
	var evidence domain.ControlEvidence
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&evidence))
	resp.Body.Close()

	// 5. Analyst downloads it back and gets the exact bytes.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/evidences/"+evidence.ID.String()+"/download", nil)
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	downloaded, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, "pdf bytes here", string(downloaded))

	// 6. Progress reflects the one implemented control.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/progress", nil)
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var progress applicationcompliance.ComplianceProgress
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&progress))
	resp.Body.Close()
	require.Equal(t, 1, progress.Total)
	require.InDelta(t, 100.0, progress.PercentComplete, 0.001)

	// 7. Analyst cannot delete evidence or the control (admin-only, see
	// permission.go's audit-trail-integrity rationale).
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/compliance/evidences/"+evidence.ID.String(), nil)
	resp, err = analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode)
	resp.Body.Close()

	// 8. Admin deletes evidence, then the control.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/compliance/evidences/"+evidence.ID.String(), nil)
	resp, err = adminApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/compliance/controls/"+control.ID.String(), nil)
	resp, err = adminApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()
}

func TestCreateFramework_AnalystForbidden(t *testing.T) {
	db := setupComplianceSchema(t)
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	analystApp := buildComplianceApp(t, db, store, uuid.New(), "analyst")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks",
		mustJSON(t, map[string]string{"name": "COBAC"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := analystApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode, "an Analyst must not be able to create a global framework")
}

func TestCreateControl_ViewerForbidden(t *testing.T) {
	db := setupComplianceSchema(t)
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	tenantID := uuid.New()
	adminApp := buildComplianceApp(t, db, store, tenantID, "admin")
	viewerApp := buildComplianceApp(t, db, store, tenantID, "viewer")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks",
		mustJSON(t, map[string]string{"name": "PCI-DSS", "version": "4.0"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := adminApp.Test(req)
	require.NoError(t, err)
	var fw domain.ComplianceFramework
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fw))
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/controls",
		mustJSON(t, map[string]string{"name": "Requirement 1"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = viewerApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode, "a Viewer must not be able to create a control")
}

func TestControl_CrossTenant_NotFoundAtHandlerLevel(t *testing.T) {
	db := setupComplianceSchema(t)
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	tenantA := uuid.New()
	tenantB := uuid.New()
	appA := buildComplianceApp(t, db, store, tenantA, "admin")
	appB := buildComplianceApp(t, db, store, tenantB, "admin")

	// Create framework + control under tenantA.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks",
		mustJSON(t, map[string]string{"name": "SOC 2", "version": "2023"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := appA.Test(req)
	require.NoError(t, err)
	var fw domain.ComplianceFramework
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fw))
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/controls",
		mustJSON(t, map[string]string{"name": "CC1.1"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = appA.Test(req)
	require.NoError(t, err)
	var control domain.ComplianceControl
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&control))
	resp.Body.Close()

	// tenantB (different app instance, same DB) must get 404 on Get/Patch/Delete.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/controls/"+control.ID.String(), nil)
	resp, err = appB.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/compliance/controls/"+control.ID.String(),
		mustJSON(t, map[string]string{"status": "implemented"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = appB.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/compliance/controls/"+control.ID.String(), nil)
	resp, err = appB.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()

	// The control must still exist for tenantA.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/controls/"+control.ID.String(), nil)
	resp, err = appA.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()
}

func TestEvidenceDownload_CrossTenant_NotFound(t *testing.T) {
	db := setupComplianceSchema(t)
	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	tenantA := uuid.New()
	tenantB := uuid.New()
	appA := buildComplianceApp(t, db, store, tenantA, "admin")
	appB := buildComplianceApp(t, db, store, tenantB, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks",
		mustJSON(t, map[string]string{"name": "NIST CSF", "version": "2.0"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := appA.Test(req)
	require.NoError(t, err)
	var fw domain.ComplianceFramework
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fw))
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/controls",
		mustJSON(t, map[string]string{"name": "ID.AM-1"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = appA.Test(req)
	require.NoError(t, err)
	var control domain.ComplianceControl
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&control))
	resp.Body.Close()

	req = newMultipartFileRequest(t, "/api/v1/compliance/controls/"+control.ID.String()+"/evidences",
		"secret.pdf", "confidential", "sensitive content")
	resp, err = appA.Test(req)
	require.NoError(t, err)
	var evidence domain.ControlEvidence
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&evidence))
	resp.Body.Close()

	// tenantB guessing the evidence ID must not be able to download it.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/compliance/evidences/"+evidence.ID.String()+"/download", nil)
	resp, err = appB.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	resp.Body.Close()
}
