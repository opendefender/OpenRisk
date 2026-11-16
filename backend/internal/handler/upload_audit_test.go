// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// Proof for #1316 (uploads, input validation, output encoding). Every claim in
// docs/security/upload-input-audit-1316.md names one of these tests.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/fileupload"
	"github.com/opendefender/openrisk/pkg/storage"
)

const (
	uaPDF  = "%PDF-1.7\n1 0 obj\n<<>>\nendobj\n"
	uaHTML = "<html><body><script>alert(1)</script></body></html>"
	uaSVG  = `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
)

var uaPayloads = []string{
	`<script>alert(1)</script>`,
	`"><img src=x onerror=alert(1)>`,
}

// uaUpload posts a multipart file whose part claims declaredType, whatever the
// bytes are: the server must not believe the header.
func uaUpload(t *testing.T, app *fiber.App, url, filename, declaredType, content string, fields map[string]string) (int, http.Header, []byte) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", declaredType)
	part, err := w.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, raw
}

type uaEvidenceEnv struct {
	db      *gorm.DB
	dir     string
	store   storage.Storage
	tenantA uuid.UUID
	tenantB uuid.UUID
	appA    *fiber.App
	appB    *fiber.App
	control uuid.UUID
	uploadA string
}

func newUAEvidenceEnv(t *testing.T) *uaEvidenceEnv {
	t.Helper()
	db := setupComplianceSchema(t)
	dir := t.TempDir()
	store, err := storage.NewLocalStorage(dir)
	require.NoError(t, err)
	e := &uaEvidenceEnv{db: db, dir: dir, store: store, tenantA: uuid.New(), tenantB: uuid.New()}
	e.appA = buildComplianceApp(t, db, store, e.tenantA, "admin")
	e.appB = buildComplianceApp(t, db, store, e.tenantB, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks", mustJSON(t, map[string]string{"name": "NIST CSF", "version": "2.0"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.appA.Test(req)
	require.NoError(t, err)
	var fw domain.ComplianceFramework
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fw))
	resp.Body.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/compliance/frameworks/"+fw.ID.String()+"/controls", mustJSON(t, map[string]string{"name": "ID.AM-1"}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = e.appA.Test(req)
	require.NoError(t, err)
	var ctl domain.ComplianceControl
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ctl))
	resp.Body.Close()
	e.control = ctl.ID
	e.uploadA = "/api/v1/compliance/controls/" + ctl.ID.String() + "/evidences"
	return e
}

func (e *uaEvidenceEnv) storedFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	require.NoError(t, filepath.Walk(e.dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(e.dir, p)
			out = append(out, rel)
		}
		return err
	}))
	return out
}

func TestEvidenceUpload_Success_StoredNameIsServerGenerated(t *testing.T) {
	e := newUAEvidenceEnv(t)
	status, _, raw := uaUpload(t, e.appA, e.uploadA, "Payroll Q3 <secret>.PDF", "application/pdf", uaPDF, nil)
	require.Equal(t, 201, status, string(raw))

	var ev domain.Evidence
	require.NoError(t, json.Unmarshal(raw, &ev))
	require.Equal(t, "Payroll Q3 <secret>.PDF", ev.Filename, "the client name is kept for display only")

	files := e.storedFiles(t)
	require.Len(t, files, 1)
	require.Equal(t, e.tenantA.String(), filepath.Dir(files[0]), "stored under the tenant namespace")
	base := filepath.Base(files[0])
	require.True(t, strings.HasSuffix(base, ".pdf"), base)
	_, err := uuid.Parse(strings.TrimSuffix(base, ".pdf"))
	require.NoError(t, err, "stored name must be <uuid>.pdf, got %q", base)
	require.NotContains(t, ev.FileRef, "Payroll")
}

func TestEvidenceUpload_WrongType_ScriptBearingFilesAreRejected(t *testing.T) {
	for name, body := range map[string]string{
		"html declared as pdf":    uaHTML,
		"svg declared as pdf":     uaSVG,
		"xml svg declared as pdf": `<?xml version="1.0"?>` + uaSVG,
		"exe declared as pdf":     "MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00",
	} {
		t.Run(name, func(t *testing.T) {
			e := newUAEvidenceEnv(t)
			status, _, raw := uaUpload(t, e.appA, e.uploadA, "report.pdf", "application/pdf", body, nil)
			require.Equal(t, 400, status, string(raw))
			require.Contains(t, string(raw), "not allowed")
			require.Empty(t, e.storedFiles(t), "a refused upload must not touch storage")
			var n int64
			require.NoError(t, e.db.Model(&domain.Evidence{}).Count(&n).Error)
			require.Zero(t, n)
		})
	}
}

func TestEvidenceUpload_Oversize_Is413(t *testing.T) {
	e := newUAEvidenceEnv(t)
	big := uaPDF + strings.Repeat("a", fileupload.EvidenceMaxBytes)
	status, _, raw := uaUpload(t, e.appA, e.uploadA, "big.pdf", "application/pdf", big, nil)
	require.Equal(t, 413, status, string(raw))
	require.Empty(t, e.storedFiles(t))
}

func TestEvidenceUpload_Unauthorized_RoleWithoutCreatePermission(t *testing.T) {
	e := newUAEvidenceEnv(t)
	viewer := buildComplianceApp(t, e.db, e.store, e.tenantA, "viewer")
	status, _, _ := uaUpload(t, viewer, e.uploadA, "a.pdf", "application/pdf", uaPDF, nil)
	require.Equal(t, 403, status)
	require.Empty(t, e.storedFiles(t))
}

func TestEvidenceUpload_CrossTenant_ControlOfAnotherTenantIsNotFound(t *testing.T) {
	e := newUAEvidenceEnv(t)
	status, _, raw := uaUpload(t, e.appB, e.uploadA, "a.pdf", "application/pdf", uaPDF, nil)
	require.Equal(t, 404, status, string(raw))
	require.Empty(t, e.storedFiles(t))
}

// seedEvidenceFile writes an evidence row (the filename is whatever we choose,
// as a hostile uploader's would be) and its blob for tenant.
func (e *uaEvidenceEnv) seedEvidenceFile(t *testing.T, tenant uuid.UUID, filename string) uuid.UUID {
	t.Helper()
	key, err := e.store.Save(t.Context(), tenant, "evidence.pdf", strings.NewReader(uaPDF))
	require.NoError(t, err)
	ev := domain.Evidence{ID: uuid.New(), TenantID: tenant, Title: "t", Type: domain.EvidenceTypeDocument,
		FileRef: key, Filename: filename, CollectedAt: time.Now(), Review: domain.EvidenceReviewAccepted, Source: domain.EvidenceSourceManual}
	require.NoError(t, e.db.Create(&ev).Error)
	return ev.ID
}

func (e *uaEvidenceEnv) download(t *testing.T, app *fiber.App, id uuid.UUID) (int, http.Header, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/compliance/evidences/"+id.String()+"/download", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(raw)
}

func TestEvidenceDownload_HeadersWithHostileFilename(t *testing.T) {
	e := newUAEvidenceEnv(t)
	id := e.seedEvidenceFile(t, e.tenantA, "a\"\r\nX-Evil: 1.pdf")
	status, h, body := e.download(t, e.appA, id)
	require.Equal(t, 200, status)
	require.Equal(t, uaPDF, body)
	require.Empty(t, h.Get("X-Evil"), "a filename must never inject a header")
	cd := h.Get("Content-Disposition")
	require.True(t, strings.HasPrefix(cd, "attachment; "), cd)
	require.NotContains(t, cd, "\r")
	require.NotContains(t, cd, "\n")
	require.Equal(t, 2, strings.Count(cd, `"`), "no inner quote survives: %q", cd)
	require.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	require.Equal(t, "default-src 'none'; sandbox", h.Get("Content-Security-Policy"))
	require.Equal(t, "application/octet-stream", h.Get("Content-Type"))
}

func TestEvidenceDownload_UTF8FilenameIsPercentEncoded(t *testing.T) {
	e := newUAEvidenceEnv(t)
	id := e.seedEvidenceFile(t, e.tenantA, "rapport sécurité.pdf")
	_, h, _ := e.download(t, e.appA, id)
	require.Equal(t, `attachment; filename="rapport s_curit_.pdf"; filename*=UTF-8''rapport%20s%C3%A9curit%C3%A9.pdf`, h.Get("Content-Disposition"))
}

func TestEvidenceDownload_CrossTenant_NoBytesLeak(t *testing.T) {
	e := newUAEvidenceEnv(t)
	id := e.seedEvidenceFile(t, e.tenantA, "a.pdf")
	status, _, body := e.download(t, e.appB, id)
	require.Equal(t, 404, status)
	require.NotContains(t, body, "%PDF")
}

// The global header middleware, mounted as main.go mounts it, puts nosniff on
// every API response (downloads included).
func TestSecurityHeaders_NosniffOnEveryResponse(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.SecurityHeaders(middleware.LoadSecurityHeadersConfig()))
	app.Get("/download", func(c *fiber.Ctx) error { return c.SendString("x") })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/download", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
}

// ---------------------------------------------------------------------------
// CSV imports: the type is decided from the bytes, not the .csv extension.
// ---------------------------------------------------------------------------

const uaPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00"

func TestRiskImportHTTP_RefusesBinaryNamedCSV(t *testing.T) {
	h := newImportApp(t)
	*h.tenant = uuid.New()
	for name, body := range map[string]string{"png": uaPNG, "html": uaHTML + "\x00"} {
		status, out := h.upload(t, "risks.csv", body)
		require.Equal(t, 400, status, name)
		require.Equal(t, "validation_failed", out["error"])
		require.Contains(t, out["message"], "CSV")
	}
	require.Empty(t, h.risksOf(t, *h.tenant))
}

func TestRiskImportHTTP_Oversize_Is413(t *testing.T) {
	h := newImportApp(t)
	*h.tenant = uuid.New()
	status, out := h.upload(t, "risks.csv", "title,probability,impact\n"+strings.Repeat("a", fileupload.CSVMaxBytes))
	require.Equal(t, 413, status)
	require.Equal(t, "validation_failed", out["error"])
}

func TestAssetImportHTTP_RefusesBinaryNamedCSV(t *testing.T) {
	h := newAssetImportApp(t)
	*h.tenant = uuid.New()
	status, out := h.upload(t, "assets.csv", uaPNG)
	require.Equal(t, 400, status)
	require.Equal(t, "validation_failed", out["error"])
	require.Empty(t, h.assetsOf(t, *h.tenant))
}

func TestAssetImportHTTP_Oversize_Is413(t *testing.T) {
	h := newAssetImportApp(t)
	*h.tenant = uuid.New()
	status, _ := h.upload(t, "assets.csv", "name,type\n"+strings.Repeat("a", fileupload.CSVMaxBytes))
	require.Equal(t, 413, status)
}

// ---------------------------------------------------------------------------
// Output encoding (criterion 4). Every free-text field is stored verbatim and
// encoded on output: Fiber's JSON encoder is encoding/json, which writes < > &
// as < > &, so no response carries a live tag. The responses are
// application/json, which a browser does not render as HTML.
// ---------------------------------------------------------------------------

func uaAssertEncoded(t *testing.T, where string, h http.Header, raw []byte) {
	t.Helper()
	require.True(t, strings.HasPrefix(h.Get("Content-Type"), "application/json"), "%s: content type %q", where, h.Get("Content-Type"))
	s := string(raw)
	require.NotContains(t, s, "<script", where)
	require.NotContains(t, s, "<img", where)
	require.NotContains(t, s, "\"><", where)
	require.Contains(t, s, "\\u003c", "%s: the payload must be present, escaped", where)
}

// Every domain entity that carries user free text, served through c.JSON as the
// handlers do.
func TestOutputEncoding_EveryFreeTextField(t *testing.T) {
	for _, p := range uaPayloads {
		entities := map[string]any{
			"risk title+description":     domain.Risk{Title: p, Description: p},
			"asset name+type+owner":      domain.Asset{Name: p, Type: p, Owner: p},
			"organization name+industry": domain.Organization{Name: p, Industry: p},
			"user full_name+bio+dept":    domain.User{FullName: p, Bio: p, Department: p, Phone: p},
			"mitigation title+desc":      domain.Mitigation{Title: p, Description: p},
			"framework name+desc":        domain.ComplianceFramework{Name: p, Description: p},
			"control name+desc":          domain.ComplianceControl{Name: p, Description: p},
			"evidence free text":         domain.Evidence{Title: p, Description: p, Filename: p, SourceDetail: p, ReviewNote: p, ExternalURL: p},
		}
		for where, v := range entities {
			app := fiber.New()
			app.Get("/x", func(c *fiber.Ctx) error { return c.JSON(v) })
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil), -1)
			require.NoError(t, err)
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			uaAssertEncoded(t, where, resp.Header, raw)

			// Encoding is lossless: the client gets the exact text back once it
			// decodes the JSON, and renders it as text.
			var back map[string]any
			require.NoError(t, json.Unmarshal(raw, &back))
			require.Contains(t, string(raw), strings.NewReplacer("<", "\\u003c", ">", "\\u003e").Replace(p), where)
		}
	}
}

// End to end through the evidence handler: written by the user, read back.
func TestOutputEncoding_EvidenceThroughHandler(t *testing.T) {
	e := newUAEvidenceEnv(t)
	for _, p := range uaPayloads {
		status, h, raw := uaUpload(t, e.appA, e.uploadA, "a.pdf", "application/pdf", uaPDF, map[string]string{"title": p, "description": p})
		require.Equal(t, 201, status, string(raw))
		uaAssertEncoded(t, "create response", h, raw)
	}
	resp, err := e.appA.Test(httptest.NewRequest(http.MethodGet, e.uploadA, nil), -1)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	uaAssertEncoded(t, "list by control", resp.Header, raw)
}

// End to end through the risk handler: imported as CSV, read back by id.
func TestOutputEncoding_RiskThroughHandler(t *testing.T) {
	h := newImportApp(t)
	tenant := uuid.New()
	*h.tenant = tenant
	for i, p := range uaPayloads {
		csv := "title,description,probability,impact\n" +
			`"` + strings.ReplaceAll(p, `"`, `""`) + ` ` + string(rune('a'+i)) + `","` + strings.ReplaceAll(p, `"`, `""`) + `",0.5,5` + "\n"
		status, out := h.upload(t, "risks.csv", csv)
		require.Equal(t, 200, status, out)
	}
	risks := h.risksOf(t, tenant)
	require.Len(t, risks, 2)
	for _, r := range risks {
		resp, err := h.app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/risks/"+r.ID.String(), nil), -1)
		require.NoError(t, err)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode, string(raw))
		uaAssertEncoded(t, "GET /risks/:id", resp.Header, raw)
	}
}
