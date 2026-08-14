// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appcompliance "github.com/opendefender/openrisk/internal/application/compliance"
	appevidence "github.com/opendefender/openrisk/internal/application/evidence"
	appreport "github.com/opendefender/openrisk/internal/application/report"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/infrastructure/workers"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/storage"
)

// The acceptance scenario for the reporting engine, driven through the real HTTP
// stack: real router, real handler, real service, real worker, real repository,
// real renderers. Only the database (sqlite) and the auth middleware are stood
// in for.
//
// It is written as a test rather than as a one-off run against a live server
// because a report is a compliance artifact: the properties asserted here — that
// the hash on the page is the hash of the file, that a published document cannot
// be edited, that the French and English versions are genuinely different
// documents — are the ones that must not silently regress.

type reportHarness struct {
	app      *fiber.App
	worker   *workers.ReportWorker
	tenantID uuid.UUID
	db       *gorm.DB
}

func buildReportApp(t *testing.T, tenantID uuid.UUID) *reportHarness {
	t.Helper()

	db := setupComplianceSchema(t)
	require.NoError(t, db.AutoMigrate(&domain.Report{}, &domain.ReportComment{}))

	store, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	complianceRepo := repository.NewGormComplianceRepository(db)
	evidenceRepo := repository.NewGormEvidenceRepository(db)
	reportRepo := repository.NewGormReportRepository(db)

	sources := appreport.Sources{
		Compliance: complianceRepo,
		Evidence:   evidenceRepo,
		// Risks, incidents, audits and org are deliberately absent: this harness
		// proves the compliance report end to end AND that a missing source
		// degrades its own report rather than the engine.
	}

	svc := appreport.NewService(reportRepo, sources)
	h := NewReportHandler(svc, nil) // no Redis: the progress endpoint must still work

	complianceH := NewComplianceHandler(
		appcompliance.NewCreateFrameworkUseCase(complianceRepo),
		appcompliance.NewGetFrameworkUseCase(complianceRepo),
		appcompliance.NewListFrameworksUseCase(complianceRepo),
		appcompliance.NewDeleteFrameworkUseCase(complianceRepo),
		appcompliance.NewCreateControlUseCase(complianceRepo),
		appcompliance.NewGetControlUseCase(complianceRepo),
		appcompliance.NewListControlsUseCase(complianceRepo),
		appcompliance.NewUpdateControlUseCase(complianceRepo),
		appcompliance.NewDeleteControlUseCase(complianceRepo),
		appcompliance.NewGetComplianceProgressUseCase(complianceRepo),
		appcompliance.NewListCatalogsUseCase(),
		appcompliance.NewImportCatalogUseCase(complianceRepo),
		appcompliance.NewGenerateComplianceReportUseCase(complianceRepo,
			repository.NewGormOrganizationRepository(db), repository.NewGormUserRepository(db)),
		appcompliance.NewGetGapAnalysisUseCase(complianceRepo),
		appcompliance.NewCreateControlMappingUseCase(repository.NewGormControlCrosswalkRepository(db), complianceRepo),
		appcompliance.NewListControlMappingsUseCase(repository.NewGormControlCrosswalkRepository(db), complianceRepo),
		appcompliance.NewDeleteControlMappingUseCase(repository.NewGormControlCrosswalkRepository(db)),
	)
	evidenceH := NewEvidenceHandler(appevidence.NewService(evidenceRepo, complianceRepo, store))

	app := fiber.New()
	userID := uuid.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: userID, OrganizationID: tenantID})
		c.Locals("user", &domain.UserClaims{ID: userID, RoleName: "admin"})
		return c.Next()
	})

	api := app.Group("/api/v1")
	api.Post("/compliance/frameworks", complianceH.CreateFramework)
	api.Post("/compliance/frameworks/:frameworkId/import-catalog", complianceH.ImportCatalog)
	api.Get("/compliance/frameworks/:frameworkId/controls", complianceH.ListControls)
	api.Patch("/compliance/controls/:controlId", complianceH.UpdateControl)
	api.Post("/evidence", evidenceH.Create)

	api.Get("/reports/types", h.Catalogue)
	api.Get("/reports", h.List)
	api.Post("/reports", h.Create)
	api.Get("/reports/:reportId", h.Get)
	api.Delete("/reports/:reportId", h.Delete)
	api.Get("/reports/:reportId/download", h.Download)
	api.Get("/reports/:reportId/verify", h.Verify)
	api.Get("/reports/:reportId/versions", h.Versions)
	api.Get("/reports/:reportId/compare", h.Compare)
	api.Post("/reports/:reportId/comments", h.Comment)
	api.Post("/reports/:reportId/transition", h.Transition)

	worker := workers.NewReportWorker(reportRepo, sources, zerolog.New(io.Discard))

	return &reportHarness{app: app, worker: worker, tenantID: tenantID, db: db}
}

// do issues a request and decodes the JSON body.
func (h *reportHarness) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, mustJSON(t, body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func (h *reportHarness) download(t *testing.T, id string) (int, []byte, http.Header) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/"+id+"/download", nil)
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw, resp.Header
}

// drain renders everything queued, the way the worker's own loop does.
func (h *reportHarness) drain(t *testing.T) {
	t.Helper()
	for i := 0; i < 20; i++ {
		worked, err := h.worker.RunOnce(context.Background())
		require.NoError(t, err)
		if !worked {
			return
		}
	}
	t.Fatal("the queue did not drain — the worker may be looping on a report it cannot finish")
}

// seedCOBAC imports the COBAC catalogue and evidences part of it, so the report
// has something to say beyond zeroes.
func seedCOBAC(t *testing.T, h *reportHarness) string {
	t.Helper()

	status, fw := h.do(t, http.MethodPost, "/api/v1/compliance/frameworks",
		map[string]string{"name": "COBAC R-2016/04", "version": "2016"})
	require.Equal(t, 201, status)
	frameworkID, _ := fw["id"].(string)
	require.NotEmpty(t, frameworkID)

	status, _ = h.do(t, http.MethodPost,
		"/api/v1/compliance/frameworks/"+frameworkID+"/import-catalog",
		map[string]string{"catalog_key": "cobac"})
	require.Equal(t, 200, status)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/frameworks/"+frameworkID+"/controls", nil)
	resp, err := h.app.Test(req, -1)
	require.NoError(t, err)
	var controls []domain.ComplianceControl
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&controls))
	resp.Body.Close()
	require.NotEmpty(t, controls, "the COBAC catalogue should have imported controls")

	// Evidence and implement the first few, so coverage is neither 0% nor 100%.
	for _, c := range controls[:4] {
		status, _ := h.do(t, http.MethodPost, "/api/v1/evidence", map[string]any{
			"title":       "Procédure " + c.ReferenceCode,
			"description": "validée par le comité d'audit",
			"control_ids": []string{c.ID.String()},
		})
		require.Equal(t, 201, status)

		status, _ = h.do(t, http.MethodPatch, "/api/v1/compliance/controls/"+c.ID.String(),
			map[string]string{"status": "implemented"})
		require.Equal(t, 200, status)
	}
	return frameworkID
}

// TestReportEngine_COBAC_FrenchThenEnglish is the scenario the task specifies:
// from Compliance, generate a COBAC report in French then in English, download
// the PDF, verify the hash, approve it, and delete a draft.
func TestReportEngine_COBAC_FrenchThenEnglish(t *testing.T) {
	h := buildReportApp(t, uuid.New())
	frameworkID := seedCOBAC(t, h)

	// --- the configurator's catalogue -------------------------------------
	status, catalogue := h.do(t, http.MethodGet, "/api/v1/reports/types?locale=fr", nil)
	require.Equal(t, 200, status)
	types, _ := catalogue["types"].([]any)
	require.Len(t, types, 6, "the configurator offers six report types")
	locales, _ := catalogue["locales"].([]any)
	require.Len(t, locales, 2, "and two document languages")

	// --- French -----------------------------------------------------------
	status, fr := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "fr",
		"framework_id": frameworkID,
		"recipients":   []string{"comite.audit@banque.cm"},
	})
	require.Equal(t, 202, status, "queued, not rendered — the request returns an address")
	frID, _ := fr["id"].(string)
	assert.Equal(t, "queued", fr["run_state"])
	assert.Equal(t, "draft", fr["lifecycle"], "a fresh report is nobody's decision yet")
	assert.NotEmpty(t, fr["template_version"], "the layout version is pinned at request time")

	h.drain(t)

	status, frDone := h.do(t, http.MethodGet, "/api/v1/reports/"+frID, nil)
	require.Equal(t, 200, status)
	require.Equal(t, "succeeded", frDone["run_state"], "render failed: %v", frDone["error"])
	assert.EqualValues(t, 100, frDone["progress"])
	frHash, _ := frDone["content_hash"].(string)
	require.Len(t, frHash, 64, "a sha-256, printed on the document and recorded on the row")

	// --- English, same report --------------------------------------------
	status, en := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "en",
		"framework_id": frameworkID,
	})
	require.Equal(t, 202, status)
	enID, _ := en["id"].(string)
	h.drain(t)

	status, enDone := h.do(t, http.MethodGet, "/api/v1/reports/"+enID, nil)
	require.Equal(t, 200, status)
	require.Equal(t, "succeeded", enDone["run_state"], "render failed: %v", enDone["error"])

	frTitle, _ := frDone["title"].(string)
	enTitle, _ := enDone["title"].(string)
	assert.NotEqual(t, frTitle, enTitle,
		"the document language is independent of the interface: the two must actually differ")
	assert.NotEqual(t, frHash, enDone["content_hash"],
		"two languages are two documents")

	// --- download the PDF and check the bytes -----------------------------
	status, body, headers := h.download(t, frID)
	require.Equal(t, 200, status)
	assert.Equal(t, "application/pdf", headers.Get("Content-Type"))
	require.True(t, bytes.HasPrefix(body, []byte("%PDF-")), "the download must be a real PDF")

	sum := sha256.Sum256(body)
	recomputed := hex.EncodeToString(sum[:])
	assert.Equal(t, frHash, recomputed,
		"the hash on the row must be the hash of the bytes actually served")
	assert.Equal(t, frHash, headers.Get("X-Content-SHA256"),
		"the hash travels with the download so a client can check it without a second call")

	// The value PRINTED on the page is the content fingerprint, not the file
	// hash: a file cannot carry the hash of itself, since printing it changes the
	// bytes being hashed. So the assertion is that the printed number is the one
	// the API reports as the fingerprint — the two must not drift apart, which is
	// exactly the bug this test found when they did.
	frFingerprint, _ := frDone["content_fingerprint"].(string)
	require.Len(t, frFingerprint, 64, "the report must carry a content fingerprint")
	assert.NotEqual(t, frHash, frFingerprint,
		"the file hash and the content fingerprint answer different questions")
	// Searching the raw bytes finds nothing — PDF content streams are
	// Flate-compressed — so the streams are inflated first.
	assert.True(t, pdfContainsText(t, body, frFingerprint[:16]),
		"the content fingerprint the API reports must be the one printed on the page")

	// --- the server's verification verdict --------------------------------
	status, verdict := h.do(t, http.MethodGet, "/api/v1/reports/"+frID+"/verify", nil)
	require.Equal(t, 200, status)
	assert.Equal(t, true, verdict["intact"])
	assert.Equal(t, frHash, verdict["recomputed_hash"])
	assert.Equal(t, frFingerprint, verdict["content_fingerprint"],
		"verify reports both numbers, so a reader can tell which one they hold")

	// --- approve ----------------------------------------------------------
	status, r := h.do(t, http.MethodPost, "/api/v1/reports/"+frID+"/transition",
		map[string]string{"to": "in_review", "comment": "à relire avant le comité"})
	require.Equal(t, 200, status)
	assert.Equal(t, "in_review", r["lifecycle"])

	status, r = h.do(t, http.MethodPost, "/api/v1/reports/"+frID+"/transition",
		map[string]string{"to": "approved", "comment": "conforme, approuvé"})
	require.Equal(t, 200, status)
	assert.Equal(t, "approved", r["lifecycle"])
	assert.NotNil(t, r["approved_by"], "an approval records who gave it")
	assert.NotNil(t, r["approved_at"], "and when")
	comments, _ := r["comments"].([]any)
	assert.Len(t, comments, 2, "the review trail travels with the document")

	status, r = h.do(t, http.MethodPost, "/api/v1/reports/"+frID+"/transition",
		map[string]string{"to": "published"})
	require.Equal(t, 200, status)
	assert.Equal(t, "published", r["lifecycle"])

	// --- published is frozen ---------------------------------------------
	status, refusal := h.do(t, http.MethodPost, "/api/v1/reports/"+frID+"/transition",
		map[string]string{"to": "draft", "comment": "revenir en arrière"})
	assert.Equal(t, 400, status, "a published document cannot be edited")
	assert.Contains(t, firstString(refusal), "version",
		"and the refusal has to say what to do instead")

	status, _ = h.do(t, http.MethodDelete, "/api/v1/reports/"+frID, nil)
	assert.Equal(t, 400, status, "nor deleted — people already hold it")

	// --- delete a draft ---------------------------------------------------
	status, draft := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "fr",
		"framework_id": frameworkID,
	})
	require.Equal(t, 202, status)
	draftID, _ := draft["id"].(string)
	h.drain(t)

	status, _ = h.do(t, http.MethodDelete, "/api/v1/reports/"+draftID, nil)
	assert.Equal(t, 204, status, "a draft is the author's to discard")
	status, _ = h.do(t, http.MethodGet, "/api/v1/reports/"+draftID, nil)
	assert.Equal(t, 404, status)
}

// DOCX and XLSX must be packages the reader opens, not files that download and
// then fail — the failure a user experiences as the product being broken.
func TestReportEngine_ProducesOpenableOfficeDocuments(t *testing.T) {
	h := buildReportApp(t, uuid.New())
	frameworkID := seedCOBAC(t, h)

	for _, tc := range []struct{ format, part string }{
		{"docx", "word/document.xml"},
		{"xlsx", "xl/workbook.xml"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			status, created := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
				"type": "compliance_framework", "format": tc.format, "locale": "fr",
				"framework_id": frameworkID,
			})
			require.Equal(t, 202, status)
			id, _ := created["id"].(string)
			h.drain(t)

			status, done := h.do(t, http.MethodGet, "/api/v1/reports/"+id, nil)
			require.Equal(t, 200, status)
			require.Equal(t, "succeeded", done["run_state"], "render failed: %v", done["error"])

			status, body, headers := h.download(t, id)
			require.Equal(t, 200, status)
			assert.Contains(t, headers.Get("Content-Type"), "openxmlformats",
				"the browser has to be told what it is receiving")

			zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
			require.NoError(t, err, "the download must be a readable package")
			names := map[string]bool{}
			for _, f := range zr.File {
				names[f.Name] = true
			}
			assert.True(t, names[tc.part], "the package must contain %s", tc.part)
			assert.True(t, names["[Content_Types].xml"], "and its content types part")

			// The file hash still has to match what was recorded.
			sum := sha256.Sum256(body)
			assert.Equal(t, done["content_hash"], hex.EncodeToString(sum[:]))
			assert.NotEmpty(t, done["content_fingerprint"])
		})
	}
}

// The same content rendered in two formats must fingerprint identically. That is
// what makes the printed number worth comparing: someone holding the PDF and
// someone holding the spreadsheet can tell they are reading the same report.
func TestReportEngine_FingerprintIsFormatIndependent(t *testing.T) {
	h := buildReportApp(t, uuid.New())
	frameworkID := seedCOBAC(t, h)

	fingerprints := map[string]string{}
	hashes := map[string]string{}
	for _, format := range []string{"pdf", "docx", "xlsx"} {
		status, created := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
			"type": "compliance_framework", "format": format, "locale": "fr",
			"framework_id": frameworkID,
		})
		require.Equal(t, 202, status)
		id, _ := created["id"].(string)
		h.drain(t)

		_, done := h.do(t, http.MethodGet, "/api/v1/reports/"+id, nil)
		require.Equal(t, "succeeded", done["run_state"], "render failed: %v", done["error"])
		fingerprints[format], _ = done["content_fingerprint"].(string)
		hashes[format], _ = done["content_hash"].(string)
	}

	assert.Equal(t, fingerprints["pdf"], fingerprints["docx"],
		"the same content in two containers is the same content")
	assert.Equal(t, fingerprints["pdf"], fingerprints["xlsx"])

	// The file hashes must all differ: they are different files.
	assert.NotEqual(t, hashes["pdf"], hashes["docx"])
	assert.NotEqual(t, hashes["docx"], hashes["xlsx"])
}

// A report the engine cannot produce must fail with a reason the requester can
// act on, not sit queued or return an empty document.
func TestReportEngine_FailsWithAReason(t *testing.T) {
	h := buildReportApp(t, uuid.New())

	// A compliance report needs a framework; asking without one is a mistake the
	// user can fix, so the message has to name it.
	status, created := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "fr",
	})
	require.Equal(t, 202, status)
	id, _ := created["id"].(string)
	h.drain(t)

	status, done := h.do(t, http.MethodGet, "/api/v1/reports/"+id, nil)
	require.Equal(t, 200, status)
	assert.Equal(t, "failed", done["run_state"])
	assert.Contains(t, done["error"], "référentiel", "the reason must name what is missing")

	// A failed report is still a report the user can open and read the reason on.
	status, _ = h.do(t, http.MethodGet, "/api/v1/reports/"+id, nil)
	assert.Equal(t, 200, status)

	// It cannot be approved: approving a document that does not exist would be
	// approving nothing.
	status, _ = h.do(t, http.MethodPost, "/api/v1/reports/"+id+"/transition",
		map[string]string{"to": "approved", "comment": "ok"})
	assert.Equal(t, 400, status)

	// And its download says so rather than serving an empty file.
	status, _, _ = h.download(t, id)
	assert.Equal(t, 400, status)
}

// A source the deployment does not have must degrade its own report, not the
// engine: the other five must still be produced.
func TestReportEngine_MissingSourceDegradesOnlyItsOwnReport(t *testing.T) {
	h := buildReportApp(t, uuid.New()) // no risk, incident or audit source wired
	frameworkID := seedCOBAC(t, h)

	status, created := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "incident", "format": "pdf", "locale": "fr",
	})
	require.Equal(t, 202, status)
	incidentID, _ := created["id"].(string)

	status, created = h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "fr",
		"framework_id": frameworkID,
	})
	require.Equal(t, 202, status)
	complianceID, _ := created["id"].(string)

	h.drain(t)

	_, incident := h.do(t, http.MethodGet, "/api/v1/reports/"+incidentID, nil)
	assert.Equal(t, "failed", incident["run_state"])
	assert.Contains(t, incident["error"], "not available",
		"it should say the module is missing, not produce an empty incident report")

	_, compliance := h.do(t, http.MethodGet, "/api/v1/reports/"+complianceID, nil)
	assert.Equal(t, "succeeded", compliance["run_state"],
		"one missing module must not stop the reports that do not need it")
}

// Versions and the comparison a reviewer reads.
func TestReportEngine_VersionsAndComparison(t *testing.T) {
	h := buildReportApp(t, uuid.New())
	frameworkID := seedCOBAC(t, h)

	status, v1 := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "fr",
		"framework_id": frameworkID,
	})
	require.Equal(t, 202, status)
	v1ID, _ := v1["id"].(string)
	h.drain(t)

	// A new version in another language, so the diff has something to report.
	status, v2 := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
		"type": "compliance_framework", "format": "pdf", "locale": "en",
		"framework_id": frameworkID, "supersedes": v1ID,
	})
	require.Equal(t, 202, status)
	v2ID, _ := v2["id"].(string)
	assert.EqualValues(t, 2, v2["version"])
	h.drain(t)

	status, lineage := h.do(t, http.MethodGet, "/api/v1/reports/"+v2ID+"/versions", nil)
	require.Equal(t, 200, status)
	versions, _ := lineage["versions"].([]any)
	require.Len(t, versions, 2, "the lineage must hold both versions")

	status, diff := h.do(t, http.MethodGet, "/api/v1/reports/"+v1ID+"/compare?with="+v2ID, nil)
	require.Equal(t, 200, status)
	changes, _ := diff["changes"].([]any)
	require.NotEmpty(t, changes)
	assert.Equal(t, false, diff["same_document"])

	joined := ""
	for _, c := range changes {
		joined += c.(string) + " | "
	}
	assert.Contains(t, joined, "language", "the diff must name what actually changed")
}

// The recent-reports list the task asks for, and the property that keeps it fast.
func TestReportEngine_RecentReportsListing(t *testing.T) {
	h := buildReportApp(t, uuid.New())
	frameworkID := seedCOBAC(t, h)

	for i := 0; i < 3; i++ {
		status, _ := h.do(t, http.MethodPost, "/api/v1/reports", map[string]any{
			"type": "compliance_framework", "format": "pdf", "locale": "fr",
			"framework_id": frameworkID,
		})
		require.Equal(t, 202, status)
	}
	h.drain(t)

	status, listed := h.do(t, http.MethodGet, "/api/v1/reports?limit=2&sort=-created_at", nil)
	require.Equal(t, 200, status)
	items, _ := listed["items"].([]any)
	assert.Len(t, items, 2, "the limit is honoured")
	assert.EqualValues(t, 3, listed["total"], "and the total still counts everything")

	// The listing must not carry the documents themselves.
	for _, raw := range items {
		item := raw.(map[string]any)
		_, hasArtifact := item["artifact"]
		assert.False(t, hasArtifact, "a listing of titles must not drag the documents with it")
		assert.NotEmpty(t, item["content_hash"], "but the hash is cheap and worth showing")
	}
}

// pdfContainsText inflates every FlateDecode content stream and looks for the
// needle in the drawing operators.
//
// Necessary because a PDF's text lives inside compressed streams: asserting on
// the raw bytes would pass or fail for reasons unrelated to what is on the page.
func pdfContainsText(t *testing.T, pdf []byte, needle string) bool {
	t.Helper()
	const marker = "stream\n"
	rest := pdf
	for {
		i := bytes.Index(rest, []byte(marker))
		if i < 0 {
			return false
		}
		rest = rest[i+len(marker):]
		end := bytes.Index(rest, []byte("\nendstream"))
		if end < 0 {
			return false
		}
		zr, err := zlib.NewReader(bytes.NewReader(rest[:end]))
		if err == nil {
			plain, err := io.ReadAll(zr)
			zr.Close()
			if err == nil && bytes.Contains(plain, []byte(needle)) {
				return true
			}
		}
		rest = rest[end:]
	}
}

// firstString pulls whichever error field the handler used.
func firstString(m map[string]any) string {
	for _, key := range []string{"error", "message"} {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}
