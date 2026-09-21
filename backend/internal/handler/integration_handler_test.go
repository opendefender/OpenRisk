// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// integRepoStub serves GetIntegration from a map keyed by (tenant, id); every
// other method is unused by the probe.
type integRepoStub struct {
	domain.VulnIntegrationRepository
	rows map[uuid.UUID]domain.VulnIntegration
}

func (r integRepoStub) GetIntegration(_ context.Context, id, tenantID uuid.UUID) (*domain.VulnIntegration, error) {
	row, ok := r.rows[id]
	if !ok || row.TenantID != tenantID {
		return nil, nil
	}
	return &row, nil
}

type probeFixture struct {
	app     *fiber.App
	tenant  uuid.UUID
	audits  []*domain.AuditLog
	dialed  *atomic.Int32
	handler *IntegrationTestHandler
}

// newProbeFixture wires the real handler. The injected client sends every
// connection to srv, whatever host the URL names — so the handler's own URL
// checks run against realistic public hostnames while the test stays offline.
func newProbeFixture(t *testing.T, srv *httptest.Server, rows ...domain.VulnIntegration) *probeFixture {
	t.Helper()
	f := &probeFixture{tenant: uuid.New(), dialed: &atomic.Int32{}}
	stub := integRepoStub{rows: map[uuid.UUID]domain.VulnIntegration{}}
	for _, r := range rows {
		if r.TenantID == uuid.Nil {
			r.TenantID = f.tenant
		}
		stub.rows[r.ID] = r
	}

	h := NewIntegrationTestHandler(vulnapp.NewGetIntegrationUseCase(stub))
	h.audit = func(l *domain.AuditLog) { f.audits = append(f.audits, l) }
	h.probe = &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test server
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				f.dialed.Add(1)
				if srv == nil {
					return nil, io.EOF
				}
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			},
		},
	}
	f.handler = h

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &authpkg.Claims{Sub: uuid.New(), TenantID: f.tenant})
		c.Locals("tenant_id", f.tenant)
		middleware.SetContext(c, &middleware.RequestContext{OrganizationID: f.tenant})
		return c.Next()
	})
	app.Post("/integrations/:id/test", h.TestIntegration)
	f.app = app
	return f
}

func (f *probeFixture) post(t *testing.T, id uuid.UUID, body string) (int, IntegrationTestResponse, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/integrations/"+id.String()+"/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out IntegrationTestResponse
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out, string(raw)
}

// Criterion 1: a metadata URL in the body → 400 naming it, no request made.
func TestIntegrationTest_RejectsMetadataTarget(t *testing.T) {
	id := uuid.New()
	f := newProbeFixture(t, nil, domain.VulnIntegration{ID: id, Source: domain.VulnSourceNessus})

	code, out, _ := f.post(t, id, `{"api_url":"http://169.254.169.254/latest/meta-data/"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	if !strings.Contains(out.Target, "169.254.169.254") {
		t.Errorf("response must name the rejected target, got %+v", out)
	}
	if f.dialed.Load() != 0 {
		t.Fatal("no outbound connection may be attempted")
	}
	if len(f.audits) != 1 || !strings.Contains(f.audits[0].ErrorMessage, "169.254.169.254") {
		t.Errorf("the attempt must be audited with its target, got %+v", f.audits)
	}
}

// Criterion 2: loopback and RFC1918 targets are refused the same way,
// whether they come from the body or from the stored integration.
func TestIntegrationTest_RejectsLoopbackAndPrivate(t *testing.T) {
	for _, target := range []string{
		"https://127.0.0.1:6379/",
		"https://localhost:8080/",
		"https://10.0.0.5/",
		"https://192.168.1.1/",
		"https://[::1]/",
	} {
		id := uuid.New()
		f := newProbeFixture(t, nil, domain.VulnIntegration{ID: id, Source: domain.VulnSourceNessus})
		if code, _, _ := f.post(t, id, `{"api_url":"`+target+`"}`); code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", target, code)
		}

		id2 := uuid.New()
		f2 := newProbeFixture(t, nil, domain.VulnIntegration{ID: id2, Source: domain.VulnSourceNessus, BaseURL: target})
		if code, _, _ := f2.post(t, id2, ``); code != http.StatusBadRequest {
			t.Errorf("stored %s: status = %d, want 400", target, code)
		}
		if f.dialed.Load()+f2.dialed.Load() != 0 {
			t.Errorf("%s: an outbound connection was attempted", target)
		}
	}
}

// Criterion 3: a failed probe reports status and reason, never the body.
func TestIntegrationTest_FailureDoesNotEchoBody(t *testing.T) {
	const secret = "redis_version:7.2.4 requirepass=hunter2"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, secret, http.StatusForbidden)
	}))
	defer srv.Close()

	id := uuid.New()
	f := newProbeFixture(t, srv, domain.VulnIntegration{ID: id, Source: domain.VulnSourceNessus, BaseURL: "https://nessus.example.com"})
	code, out, raw := f.post(t, id, ``)
	if code != http.StatusBadRequest || out.Success {
		t.Fatalf("status = %d success = %v, want 400 false", code, out.Success)
	}
	if out.Status != http.StatusForbidden || out.Reason == "" {
		t.Errorf("want status 403 and a reason, got %+v", out)
	}
	if strings.Contains(raw, "hunter2") || strings.Contains(raw, "redis_version") {
		t.Fatalf("response echoes the remote body: %s", raw)
	}
}

// Criterion 4: the URL probed is the one stored on the integration, even when
// the body names another; and the lookup is tenant-scoped.
func TestIntegrationTest_UsesStoredBaseURL(t *testing.T) {
	var gotHost string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	id := uuid.New()
	f := newProbeFixture(t, srv, domain.VulnIntegration{ID: id, Source: domain.VulnSourceNessus, BaseURL: "https://nessus.example.com"})
	code, out, _ := f.post(t, id, `{"api_url":"https://elsewhere.example.org/"}`)
	if code != http.StatusOK || !out.Success {
		t.Fatalf("status = %d out = %+v, want 200 success", code, out)
	}
	if gotHost != "nessus.example.com" {
		t.Errorf("probed host = %q, want the stored nessus.example.com", gotHost)
	}
	if len(f.audits) != 1 || f.audits[0].ResourceID == nil || *f.audits[0].ResourceID != id {
		t.Errorf("audit must reference the integration, got %+v", f.audits)
	}

	// Another tenant's integration id is a 404, and nothing is probed.
	other := uuid.New()
	f2 := newProbeFixture(t, srv, domain.VulnIntegration{ID: other, TenantID: uuid.New(), BaseURL: "https://nessus.example.com"})
	if code, _, _ := f2.post(t, other, ``); code != http.StatusNotFound {
		t.Errorf("cross-tenant id: status = %d, want 404", code)
	}
	if f2.dialed.Load() != 0 {
		t.Error("a cross-tenant id must not trigger a probe")
	}
}

// Criterion 5: a redirect from an allowed host to a denied one is not followed.
func TestIntegrationTest_DoesNotFollowRedirect(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("redirect was followed to %s", r.URL)
		}
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	id := uuid.New()
	f := newProbeFixture(t, srv, domain.VulnIntegration{ID: id, Source: domain.VulnSourceNessus, BaseURL: "https://nessus.example.com/"})
	code, out, _ := f.post(t, id, ``)
	if code != http.StatusBadRequest || out.Status != http.StatusFound || out.Reason != "redirect not followed" {
		t.Fatalf("status = %d out = %+v, want 400 / 302 / redirect not followed", code, out)
	}
	if f.dialed.Load() != 1 {
		t.Errorf("dialed %d times, want exactly 1 (no redirect hop)", f.dialed.Load())
	}
}

func TestIntegrationTest_Unauthorized(t *testing.T) {
	h := NewIntegrationTestHandler(vulnapp.NewGetIntegrationUseCase(integRepoStub{}))
	app := fiber.New()
	app.Post("/integrations/:id/test", h.TestIntegration)
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/integrations/"+uuid.NewString()+"/test", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestIntegrationTest_NotFound(t *testing.T) {
	f := newProbeFixture(t, nil)
	if code, _, _ := f.post(t, uuid.New(), ``); code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
}
