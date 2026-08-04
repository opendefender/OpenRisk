// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
)

// newHeaderProbe exercises the middleware over plain HTTP.
func newHeaderProbe(t *testing.T, h fiber.Handler) *http.Response {
	t.Helper()
	return probe(t, fiber.New(), h, nil)
}

// newTLSHeaderProbe simulates the real production shape: TLS terminated at a
// trusted load balancer that forwards X-Forwarded-Proto. app.Test cannot open a
// genuine TLS connection, and this is the path that actually matters — helmet
// only emits HSTS when c.Protocol() reports https, which behind a proxy depends
// entirely on the trusted-proxy configuration (see TrustedProxies / F-04).
func newTLSHeaderProbe(t *testing.T, h fiber.Handler) *http.Response {
	t.Helper()
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0"}, // app.Test dials from 0.0.0.0
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	return probe(t, app, h, map[string]string{fiber.HeaderXForwardedProto: "https"})
}

func probe(t *testing.T, app *fiber.App, h fiber.Handler, headers map[string]string) *http.Response {
	t.Helper()
	app.Use(h)
	app.Get("/probe", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/probe", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("probe request: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestSecurityHeaders_EmitsCSP is half the regression test for audit finding
// F-01: the app shipped helmet.New() with no config, which silently omits
// Content-Security-Policy. CSP is transport-independent, so it must be present
// on every response.
func TestSecurityHeaders_EmitsCSP(t *testing.T) {
	resp := newHeaderProbe(t, SecurityHeaders(LoadSecurityHeadersConfig()))

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP should lock down default-src for a JSON API, got %q", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP should forbid framing, got %q", csp)
	}
}

// TestSecurityHeaders_EmitsHSTSOverTLS is the other half of F-01.
func TestSecurityHeaders_EmitsHSTSOverTLS(t *testing.T) {
	resp := newTLSHeaderProbe(t, SecurityHeaders(LoadSecurityHeadersConfig()))

	hsts := resp.Header.Get("Strict-Transport-Security")
	if hsts == "" {
		t.Fatal("Strict-Transport-Security header missing over TLS")
	}
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Errorf("HSTS max-age should be one year, got %q", hsts)
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Errorf("HSTS should include subdomains by default, got %q", hsts)
	}
}

// TestSecurityHeaders_NoHSTSOverPlainHTTP documents intended behaviour rather
// than a defect: sending HSTS over cleartext is meaningless (a MITM strips it)
// and the spec tells agents to ignore it. This also pins the operational
// consequence — a deployment that terminates TLS upstream without configuring
// TRUSTED_PROXIES will not get HSTS, because c.Protocol() cannot see https.
func TestSecurityHeaders_NoHSTSOverPlainHTTP(t *testing.T) {
	resp := newHeaderProbe(t, SecurityHeaders(LoadSecurityHeadersConfig()))

	if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS should not be sent over cleartext, got %q", got)
	}
}

// TestSecurityHeaders_RetainsHelmetDefaults guards the headers that already
// worked, so hardening CSP/HSTS cannot silently regress them.
func TestSecurityHeaders_RetainsHelmetDefaults(t *testing.T) {
	resp := newHeaderProbe(t, SecurityHeaders(LoadSecurityHeadersConfig()))

	for header, want := range map[string]string{
		"X-Frame-Options":        "SAMEORIGIN",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s: got %q, want %q", header, got, want)
		}
	}
}

// TestSecurityHeaders_BareHelmetOmitsBoth documents the upstream behaviour that
// caused the finding. If a future Fiber release starts emitting these by
// default this test fails, telling us the explicit config is now redundant.
func TestSecurityHeaders_BareHelmetOmitsBoth(t *testing.T) {
	resp := newHeaderProbe(t, helmet.New())

	if got := resp.Header.Get("Content-Security-Policy"); got != "" {
		t.Errorf("bare helmet now emits CSP (%q) — revisit the explicit config", got)
	}
	if got := resp.Header.Get("Strict-Transport-Security"); got != "" {
		t.Errorf("bare helmet now emits HSTS (%q) — revisit the explicit config", got)
	}
}

func TestLoadSecurityHeadersConfig_EnvOverrides(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("CONTENT_SECURITY_POLICY", "")
		t.Setenv("HSTS_MAX_AGE", "")
		t.Setenv("HSTS_INCLUDE_SUBDOMAINS", "")

		cfg := LoadSecurityHeadersConfig()
		if cfg.ContentSecurityPolicy != DefaultAPIContentSecurityPolicy {
			t.Errorf("got CSP %q, want default", cfg.ContentSecurityPolicy)
		}
		if cfg.HSTSMaxAge != defaultHSTSMaxAge {
			t.Errorf("got max-age %d, want %d", cfg.HSTSMaxAge, defaultHSTSMaxAge)
		}
		if !cfg.HSTSIncludeSubdomains {
			t.Error("includeSubDomains should default to true")
		}
	})

	t.Run("custom policy for same-origin SPA", func(t *testing.T) {
		t.Setenv("CONTENT_SECURITY_POLICY", "default-src 'self'")
		if got := LoadSecurityHeadersConfig().ContentSecurityPolicy; got != "default-src 'self'" {
			t.Errorf("got %q, want the override", got)
		}
	})

	t.Run("policy can be disabled", func(t *testing.T) {
		t.Setenv("CONTENT_SECURITY_POLICY", "off")
		if got := LoadSecurityHeadersConfig().ContentSecurityPolicy; got != "" {
			t.Errorf("got %q, want empty so helmet omits the header", got)
		}
	})

	t.Run("hsts disabled when gateway already sets it", func(t *testing.T) {
		t.Setenv("HSTS_MAX_AGE", "0")
		if got := LoadSecurityHeadersConfig().HSTSMaxAge; got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("invalid values fall back to defaults", func(t *testing.T) {
		t.Setenv("HSTS_MAX_AGE", "not-a-number")
		t.Setenv("HSTS_INCLUDE_SUBDOMAINS", "maybe")

		cfg := LoadSecurityHeadersConfig()
		if cfg.HSTSMaxAge != defaultHSTSMaxAge {
			t.Errorf("got max-age %d, want fallback %d", cfg.HSTSMaxAge, defaultHSTSMaxAge)
		}
		if !cfg.HSTSIncludeSubdomains {
			t.Error("invalid bool should keep the secure default")
		}
	})
}
