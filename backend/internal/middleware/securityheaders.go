// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
)

// DefaultAPIContentSecurityPolicy is the policy applied to API responses.
//
// This backend only ever emits application/json, text/csv, application/pdf,
// application/xml and text/event-stream — it never serves HTML or scripts. So
// the correct policy is the maximally restrictive one: deny every fetch
// directive, forbid framing, and forbid the document-level primitives an
// injected payload would need. Nothing legitimate is lost because no browser
// context is ever created from these responses.
//
// This is deliberately NOT the policy for the SPA. The frontend is served by a
// separate origin (Vite in dev, a static host or reverse proxy in production)
// and needs its own, looser policy allowing its bundle and Google Fonts. Whoever
// serves the SPA owns that header; conflating the two would either break the app
// or water down the API policy to nothing.
const DefaultAPIContentSecurityPolicy = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// defaultHSTSMaxAge is one year in seconds, the value required for inclusion in
// browser preload lists.
const defaultHSTSMaxAge = 31536000

// SecurityHeadersConfig captures the knobs resolved from the environment.
type SecurityHeadersConfig struct {
	ContentSecurityPolicy string
	HSTSMaxAge            int
	HSTSIncludeSubdomains bool
}

// LoadSecurityHeadersConfig resolves the security header policy from env vars.
//
// Both knobs exist because deployments differ: an operator terminating TLS at a
// gateway that already injects HSTS may want to avoid a duplicate header, and an
// operator serving the SPA from this same origin needs a looser CSP.
//
//	CONTENT_SECURITY_POLICY  override the API policy ("off" disables the header)
//	HSTS_MAX_AGE             override max-age in seconds ("0" disables HSTS)
//	HSTS_INCLUDE_SUBDOMAINS  "false" to drop includeSubDomains (default true)
func LoadSecurityHeadersConfig() SecurityHeadersConfig {
	cfg := SecurityHeadersConfig{
		ContentSecurityPolicy: DefaultAPIContentSecurityPolicy,
		HSTSMaxAge:            defaultHSTSMaxAge,
		HSTSIncludeSubdomains: true,
	}

	if raw := strings.TrimSpace(os.Getenv("CONTENT_SECURITY_POLICY")); raw != "" {
		if strings.EqualFold(raw, "off") {
			cfg.ContentSecurityPolicy = ""
		} else {
			cfg.ContentSecurityPolicy = raw
		}
	}

	if raw := strings.TrimSpace(os.Getenv("HSTS_MAX_AGE")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			cfg.HSTSMaxAge = parsed
		}
	}

	if raw := strings.TrimSpace(os.Getenv("HSTS_INCLUDE_SUBDOMAINS")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			cfg.HSTSIncludeSubdomains = parsed
		}
	}

	return cfg
}

// SecurityHeaders returns the hardened helmet middleware.
//
// Audit finding F-01: the app previously called helmet.New() with no config.
// Helmet's defaults cover X-Frame-Options, Referrer-Policy and nosniff, but it
// only emits Content-Security-Policy when the policy string is non-empty and
// Strict-Transport-Security when HSTSMaxAge is non-zero — both are zero values
// by default, so neither header was ever sent. That left no defence-in-depth
// against XSS and no protection against an HTTP downgrade.
//
// Note on HSTS: helmet only emits it when the request arrives over HTTPS. Behind
// a TLS-terminating proxy this depends on the trusted-proxy configuration (see
// TrustedProxies) for c.Protocol() to report "https".
func SecurityHeaders(cfg SecurityHeadersConfig) fiber.Handler {
	return helmet.New(helmet.Config{
		ContentSecurityPolicy: cfg.ContentSecurityPolicy,
		HSTSMaxAge:            cfg.HSTSMaxAge,
		HSTSExcludeSubdomains: !cfg.HSTSIncludeSubdomains,
	})
}
