// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

// Proof for #554 (evidence record integrity) at the HTTP boundary: real
// handler, real repository, LocalStorage in a TempDir, the real audit chain.
// TestEvidence_Expired is not here on purpose: freshness against cadence is #379.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	applicationevidence "github.com/opendefender/openrisk/internal/application/evidence"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

type evIntEnv struct {
	*uaEvidenceEnv
	svcApp func(tenant, user uuid.UUID) *fiber.App
	userA  uuid.UUID
}

func newEvIntEnv(t *testing.T) *evIntEnv {
	t.Helper()
	e := newUAEvidenceEnv(t)
	require.NoError(t, e.db.AutoMigrate(&domain.AuditEvent{}))
	chain := repository.NewGormAuditChainRepository(e.db)
	repo := repository.NewGormEvidenceRepository(e.db)
	svc := applicationevidence.NewService(repo, repository.NewGormComplianceRepository(e.db), e.store).WithIntegrityAudit(chain)
	h := NewEvidenceHandler(svc)
	build := func(tenant, user uuid.UUID) *fiber.App {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			middleware.SetContext(c, &middleware.RequestContext{UserID: user, OrganizationID: tenant})
			return c.Next()
		})
		api := app.Group("/api/v1")
		api.Get("/evidence", h.List)
		api.Post("/evidence", h.Create)
		api.Get("/evidence/:evidenceId", h.Get)
		api.Patch("/evidence/:evidenceId", h.Update)
		api.Delete("/evidence/:evidenceId", h.Delete)
		api.Get("/evidence/:evidenceId/download", h.Download)
		api.Get("/compliance/evidences/:evidenceId/download", h.Download)
		api.Get("/compliance/controls/:controlId/evidences", h.ListByControl)
		return app
	}
	return &evIntEnv{uaEvidenceEnv: e, svcApp: build, userA: uuid.New()}
}

func (e *evIntEnv) appFor(tenant uuid.UUID) *fiber.App { return e.svcApp(tenant, e.userA) }

func (e *evIntEnv) create(t *testing.T, app *fiber.App, fields map[string]string) domain.Evidence {
	t.Helper()
	status, _, raw := uaUpload(t, app, "/api/v1/evidence", "report.pdf", "application/pdf", uaPDF, fields)
	require.Equal(t, 201, status, string(raw))
	var ev domain.Evidence
	require.NoError(t, json.Unmarshal(raw, &ev))
	return ev
}

func (e *evIntEnv) get(t *testing.T, app *fiber.App, method, path string) (int, []byte) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func (e *evIntEnv) alerts(t *testing.T, tenant uuid.UUID) []domain.AuditEvent {
	t.Helper()
	var out []domain.AuditEvent
	require.NoError(t, e.db.Where("tenant_id = ? AND action = ?", tenant, "integrity_alert").Find(&out).Error)
	return out
}

func (e *evIntEnv) blobPath(t *testing.T, ev domain.Evidence) string {
	t.Helper()
	files := e.storedFiles(t)
	for _, f := range files {
		if strings.HasSuffix(ev.FileRef, filepath.Base(f)) {
			return filepath.Join(e.dir, f)
		}
	}
	t.Fatalf("blob for %q not found in %v", ev.FileRef, files)
	return ""
}

// AC1: the row's hash is the SHA-256 of the uploaded bytes; a client-sent hash
// is ignored. Downloading the untouched file is "verified".
func TestEvidence_Success(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	sum := sha256.Sum256([]byte(uaPDF))
	want := hex.EncodeToString(sum[:])

	ev := e.create(t, app, map[string]string{
		"title": "access review", "content_hash": strings.Repeat("0", 64), "hash_algorithm": "md5",
	})
	require.Equal(t, want, ev.ContentHash, "the client-sent hash must be ignored")
	require.Equal(t, "sha256", ev.HashAlgorithm)
	require.Equal(t, domain.IntegrityHashed, ev.Integrity)
	require.False(t, ev.SignaturePresent)
	require.False(t, ev.SignatureVerified)

	// Read back through GET and straight from the database.
	status, raw := e.get(t, app, http.MethodGet, "/api/v1/evidence/"+ev.ID.String())
	require.Equal(t, 200, status)
	var got domain.Evidence
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, want, got.ContentHash)
	var row domain.Evidence
	require.NoError(t, e.db.First(&row, "id = ? AND tenant_id = ?", ev.ID, e.tenantA).Error)
	require.Equal(t, want, row.ContentHash)

	// JSON body creation ignores a hash too.
	status, _, raw2 := jsonPost(t, app, "/api/v1/evidence", map[string]any{"title": "stmt", "description": "d", "content_hash": "abc"})
	require.Equal(t, 201, status, string(raw2))
	var stmt domain.Evidence
	require.NoError(t, json.Unmarshal(raw2, &stmt))
	require.Empty(t, stmt.ContentHash)

	status, h, body := e.download(t, app, ev.ID)
	require.Equal(t, 200, status)
	require.Equal(t, uaPDF, body)
	require.Equal(t, "verified", h.Get("X-Evidence-Integrity"))
}

func jsonPost(t *testing.T, app *fiber.App, url string, v any) (int, http.Header, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, url, mustJSON(t, v))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, raw
}

// AC2: an altered blob serves no byte, answers the typed integrity error and
// leaves an integrity_alert audit event for that tenant (and only that tenant).
func TestEvidence_HashMismatch(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	ev := e.create(t, app, map[string]string{"title": "access review"})

	require.NoError(t, os.WriteFile(e.blobPath(t, ev), []byte(uaPDF+"tampered"), 0o600))

	status, h, body := e.download(t, app, ev.ID)
	require.Equal(t, 409, status)
	require.NotContains(t, body, "%PDF", "no content byte may be served")
	require.NotContains(t, body, "tampered")
	require.Empty(t, h.Get("Content-Disposition"))
	var errBody map[string]string
	require.NoError(t, json.Unmarshal([]byte(body), &errBody))
	require.Equal(t, domain.ReasonEvidenceIntegrity, errBody["code"])

	alerts := e.alerts(t, e.tenantA)
	require.Len(t, alerts, 1)
	require.Equal(t, "evidence", alerts[0].EntityType)
	require.Equal(t, ev.ID.String(), alerts[0].EntityID)
	require.NotNil(t, alerts[0].ActorID)
	require.Equal(t, e.userA, *alerts[0].ActorID)
	require.Contains(t, alerts[0].Summary, "evidence.integrity_alert")
	require.Empty(t, e.alerts(t, e.tenantB), "the alert belongs to the evidence's tenant only")

	// Tenant B probing the tampered id learns nothing and raises nothing.
	status, _, _ = e.download(t, e.appFor(e.tenantB), ev.ID)
	require.Equal(t, 404, status)
	require.Len(t, e.alerts(t, e.tenantA), 1)
}

// AC9: a legacy row (no hash) is served, flagged unverified, never failed.
func TestEvidence_LegacyWithoutHashIsServedUnverified(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	id := e.seedEvidenceFile(t, e.tenantA, "legacy.pdf") // no hash, no collection method

	status, h, body := e.download(t, app, id)
	require.Equal(t, 200, status)
	require.Equal(t, uaPDF, body)
	require.Equal(t, "unverified", h.Get("X-Evidence-Integrity"))

	status, raw := e.get(t, app, http.MethodGet, "/api/v1/evidence/"+id.String())
	require.Equal(t, 200, status)
	var got domain.Evidence
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, domain.IntegrityUnhashed, got.Integrity)
}

func TestEvidence_NotFound(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	missing := uuid.New().String()
	for _, p := range []string{"/api/v1/evidence/" + missing, "/api/v1/evidence/" + missing + "/download"} {
		status, _ := e.get(t, app, http.MethodGet, p)
		require.Equal(t, 404, status, p)
	}
}

// AC8: tenant B holding tenant A's evidence id gets 404 on get, download,
// update and delete; the tenant-wide list is empty; the control-scoped list 404s
// through the control lookup.
func TestEvidence_Unauthorized(t *testing.T) {
	e := newEvIntEnv(t)
	ev := e.create(t, e.appFor(e.tenantA), map[string]string{"title": "A's proof", "control_ids": e.control.String()})
	b := e.appFor(e.tenantB)
	id := ev.ID.String()

	for _, p := range []string{"/api/v1/evidence/" + id, "/api/v1/evidence/" + id + "/download"} {
		status, raw := e.get(t, b, http.MethodGet, p)
		require.Equal(t, 404, status, p)
		require.NotContains(t, string(raw), "%PDF")
	}
	for _, m := range []string{http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(m, "/api/v1/evidence/"+id, strings.NewReader(`{"title":"hijack"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := b.Test(req, -1)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, 404, resp.StatusCode, m)
	}
	status, raw := e.get(t, b, http.MethodGet, "/api/v1/evidence")
	require.Equal(t, 200, status)
	var list struct {
		Items []domain.Evidence `json:"items"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Empty(t, list.Items, "a tenant-wide list for another tenant is empty")

	status, _ = e.get(t, b, http.MethodGet, "/api/v1/compliance/controls/"+e.control.String()+"/evidences")
	require.Equal(t, 404, status, "control-scoped list is a 404 through the control lookup")

	// Nothing of A's changed.
	var row domain.Evidence
	require.NoError(t, e.db.First(&row, "id = ?", ev.ID).Error)
	require.Equal(t, "A's proof", row.Title)
}

// AC4, AC5 over HTTP, and the reserved automated methods: all 400.
func TestEvidence_RecordValidationIs400(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	base := func(extra map[string]any) map[string]any {
		m := map[string]any{"title": "x", "description": "d", "collected_at": "2026-08-10"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := map[string]map[string]any{
		"observed_from after observed_to": {"observed_from": "2026-08-09", "observed_to": "2026-08-08"},
		"observed_to after collected_at":  {"observed_from": "2026-08-09", "observed_to": "2026-08-11"},
		"unknown collection method":       {"collection_method": "carrier_pigeon"},
		"automated_collector over HTTP":   {"collection_method": "automated_collector", "collector_ref": "scanner-1"},
		"api_pull over HTTP":              {"collection_method": "api_pull", "collector_ref": "okta:prod"},
	}
	for name, extra := range cases {
		status, _, raw := jsonPost(t, app, "/api/v1/evidence", base(extra))
		require.Equal(t, 400, status, "%s: %s", name, raw)
	}
}

// A source label sent over HTTP never yields an automated method, and a posted
// collector is replaced by the authenticated user. Max assurance is moderate.
func TestEvidence_HTTPCannotClaimAnAutomatedCollector(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	status, _, raw := jsonPost(t, app, "/api/v1/evidence", map[string]any{
		"title": "x", "description": "d", "collected_at": "2026-08-10", "source": "integration",
		"collector_kind": "connector", "collector_ref": "okta:prod",
		"signature": "c2ln", "signature_algorithm": "ed25519",
	})
	require.Equal(t, 201, status, string(raw))
	var ev domain.Evidence
	require.NoError(t, json.Unmarshal(raw, &ev))
	require.Equal(t, domain.CollectionManualUpload, ev.CollectionMethod)
	require.Equal(t, domain.CollectorUser, ev.CollectorKind)
	require.Equal(t, e.userA.String(), ev.CollectorRef)
	require.True(t, ev.SignaturePresent)
	require.False(t, ev.SignatureVerified)
	require.Equal(t, domain.AssuranceLow, ev.Assurance, "signed but unverified manual_upload is one below moderate")
}

// The read model carries the derived fields on the wire, on Get and on List.
func TestEvidence_ReadJSONCarriesTheDerivedFields(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	ev := e.create(t, app, map[string]string{"title": "proof"})
	keys := []string{"assurance", "confidence", "integrity", "signature_present", "signature_verified", "content_hash", "collector_kind", "collector_ref"}

	status, raw := e.get(t, app, http.MethodGet, "/api/v1/evidence/"+ev.ID.String())
	require.Equal(t, 200, status)
	var one map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &one))
	for _, k := range keys {
		require.Contains(t, one, k, "Get: %s", k)
	}
	require.Equal(t, "false", string(one["signature_present"]))
	require.Equal(t, "false", string(one["signature_verified"]))
	require.NotContains(t, one, "signature", "the signature itself is never serialised")

	status, raw = e.get(t, app, http.MethodGet, "/api/v1/evidence")
	require.Equal(t, 200, status)
	var list struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil || len(list.Items) == 0 {
		var arr []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &arr), string(raw))
		list.Items = arr
	}
	require.NotEmpty(t, list.Items)
	for _, k := range keys {
		require.Contains(t, list.Items[0], k, "List: %s", k)
	}
}

// Update can never rewrite the integrity of the record, even when a client
// tries to send the field.
func TestEvidence_UpdateCannotRewriteTheHash(t *testing.T) {
	e := newEvIntEnv(t)
	app := e.appFor(e.tenantA)
	ev := e.create(t, app, map[string]string{"title": "proof"})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/evidence/"+ev.ID.String(),
		strings.NewReader(`{"title":"renamed","content_hash":"`+strings.Repeat("f", 64)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)

	var row domain.Evidence
	require.NoError(t, e.db.First(&row, "id = ?", ev.ID).Error)
	require.Equal(t, "renamed", row.Title)
	require.Equal(t, ev.ContentHash, row.ContentHash)
	status, _, _ := e.download(t, app, ev.ID)
	require.Equal(t, 200, status)
}
