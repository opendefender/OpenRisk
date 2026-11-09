// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #1314 — the browser-side protections, proven on every kind of response and
// on every cookie the API sets, rather than on one happy-path probe.
// ---------------------------------------------------------------------------

// headerStack mounts the global chain in main.go's order — recover, then the
// security headers — under main.go's error handler, with one route per kind of
// response the API produces.
func headerStack(t *testing.T, production bool) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0"}, // app.Test dials from 0.0.0.0
		ProxyHeader:             fiber.HeaderXForwardedFor,
		ErrorHandler:            ErrorHandler(production),
	})
	app.Use(recover.New())
	app.Use(SecurityHeaders(LoadSecurityHeadersConfig()))
	app.Get("/ok", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	app.Get("/csv", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/csv")
		return c.SendString("a,b\n1,2\n")
	})
	app.Get("/handled-error", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"code": "FORBIDDEN"})
	})
	app.Get("/returned-error", func(c *fiber.Ctx) error { return fiber.NewError(fiber.StatusBadRequest, "bad input") })
	app.Get("/internal-error", func(c *fiber.Ctx) error { return errors.New("pq: relation \"risks\" does not exist") })
	app.Get("/panic", func(c *fiber.Ctx) error { panic("boom") })
	return app
}

// responseKinds are the paths of headerStack, plus one no route matches, which
// Fiber answers through the error handler.
var responseKinds = map[string]int{
	"/ok":             http.StatusOK,
	"/csv":            http.StatusOK,
	"/handled-error":  http.StatusForbidden,
	"/returned-error": http.StatusBadRequest,
	"/internal-error": http.StatusInternalServerError,
	"/panic":          http.StatusInternalServerError,
	"/no-such-route":  http.StatusNotFound,
}

// Criterion 1: CSP, X-Content-Type-Options, Referrer-Policy and frame
// protection on every response, error responses included; HSTS over TLS only.
func TestBrowserHeaders_EveryResponseKind(t *testing.T) {
	for _, production := range []bool{false, true} {
		for _, tls := range []bool{false, true} {
			app := headerStack(t, production)
			for path, wantStatus := range responseKinds {
				name := path
				if production {
					name += "/production"
				}
				if tls {
					name += "/tls"
				}
				t.Run(name, func(t *testing.T) {
					req := httptest.NewRequest(http.MethodGet, "http://api.example.com"+path, nil)
					if tls {
						req.Header.Set(fiber.HeaderXForwardedProto, "https")
					}
					resp, err := app.Test(req, -1)
					require.NoError(t, err)
					defer func() { _ = resp.Body.Close() }()
					require.Equal(t, wantStatus, resp.StatusCode)

					h := resp.Header
					assert.Equal(t, DefaultAPIContentSecurityPolicy, h.Get("Content-Security-Policy"))
					assert.Contains(t, h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
					assert.Equal(t, "SAMEORIGIN", h.Get("X-Frame-Options"))
					assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
					assert.Equal(t, "no-referrer", h.Get("Referrer-Policy"))
					assert.Equal(t, "no-store", h.Get(fiber.HeaderCacheControl))

					// HSTS follows the transport, not APP_ENV: over cleartext a
					// browser ignores it, and production terminates TLS upstream
					// (TRUSTED_PROXIES makes c.Protocol() see https).
					if tls {
						assert.Equal(t, "max-age=31536000; includeSubDomains", h.Get("Strict-Transport-Security"))
					} else {
						assert.Empty(t, h.Get("Strict-Transport-Security"))
					}
				})
			}
		}
	}
}

// The panic path must not leak what blew up, or the header proof above would
// be proving headers on a response that is itself a disclosure.
func TestBrowserHeaders_ProductionErrorBodiesStayGeneric(t *testing.T) {
	app := headerStack(t, true)
	for _, path := range []string{"/internal-error", "/panic"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), -1)
		require.NoError(t, err)
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.NotContains(t, string(raw), "boom", path)
		assert.NotContains(t, string(raw), "relation", path)
	}
}

// wantCookie is what a browser must be told about one cookie.
type wantCookie struct {
	path     string
	httpOnly bool
}

// Criterion 2: every cookie the API sets, under every APP_ENV class. Secure
// follows the environment (production sits behind TLS-terminating proxies, so
// the request scheme cannot decide it), SameSite is always Lax, and each
// credential is scoped to the narrowest path that reads it.
func TestBrowserCookies_EveryCookieEveryEnv(t *testing.T) {
	want := map[string]wantCookie{
		AccessTokenCookie:  {path: "/", httpOnly: true},
		RefreshTokenCookie: {path: "/api/v1/auth/refresh", httpOnly: true},
		CSRFCookie:         {path: "/", httpOnly: false}, // double-submit: the SPA reads it
		SSOMFACookie:       {path: SSOMFACookiePath, httpOnly: true},
	}

	for env, secure := range map[string]bool{
		"development": false,
		"test":        false,
		"production":  true,
		"staging":     true,
		"":            true, // unset is production (IsProductionEnv)
	} {
		t.Run("APP_ENV="+env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)

			app := fiber.New()
			app.Post("/issue", func(c *fiber.Ctx) error {
				if _, err := IssueSessionCookies(c, "access", "refresh", 15*time.Minute, 24*time.Hour); err != nil {
					return err
				}
				IssueSSOMFACookie(c, "pending", time.Minute)
				SetTrustedDeviceCookie(c, "device", time.Hour)
				return c.SendStatus(fiber.StatusNoContent)
			})
			resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/issue", nil), -1)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			trusted := TrustedDeviceCookieName()
			if secure {
				assert.Equal(t, "__Secure-or_trusted_device", trusted, "over HTTPS the trusted-device cookie carries the __Secure- prefix")
			}
			expect := map[string]wantCookie{trusted: {path: "/api/v1/auth", httpOnly: true}}
			for k, v := range want {
				expect[k] = v
			}

			got := map[string]*http.Cookie{}
			for _, ck := range resp.Cookies() {
				got[ck.Name] = ck
			}
			assert.Len(t, got, len(expect), "an unlisted cookie is a cookie nobody has reviewed")
			for name, w := range expect {
				ck := got[name]
				if !assert.NotNil(t, ck, "%s not set", name) {
					continue
				}
				assert.Equal(t, w.httpOnly, ck.HttpOnly, "%s HttpOnly", name)
				assert.Equal(t, secure, ck.Secure, "%s Secure under APP_ENV=%q", name, env)
				assert.Equal(t, http.SameSiteLaxMode, ck.SameSite, "%s SameSite", name)
				assert.Equal(t, w.path, ck.Path, "%s Path", name)
				assert.Empty(t, ck.Domain, "%s must stay host-only: a Domain attribute shares it with every subdomain", name)
			}
		})
	}
}

// Criterion 2 on the way out: clearing keeps the attributes, so the browser
// matches and drops the cookie it holds rather than a sibling.
func TestBrowserCookies_ClearKeepsAttributes(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	app := fiber.New()
	app.Post("/clear", func(c *fiber.Ctx) error {
		ClearSessionCookies(c)
		ClearSSOMFACookie(c)
		return c.SendStatus(fiber.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/clear", nil), -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	paths := map[string]string{
		AccessTokenCookie:  "/",
		RefreshTokenCookie: "/api/v1/auth/refresh",
		CSRFCookie:         "/",
		SSOMFACookie:       SSOMFACookiePath,
	}
	require.Len(t, resp.Cookies(), len(paths))
	for _, ck := range resp.Cookies() {
		assert.Equal(t, paths[ck.Name], ck.Path, ck.Name)
		assert.True(t, ck.Secure, ck.Name)
		assert.Equal(t, http.SameSiteLaxMode, ck.SameSite, ck.Name)
		assert.Empty(t, ck.Value, ck.Name)
	}
}

// optionalAuthApp mounts OptionalAuth the way main.go does on the invitation
// routes, with an echo that reports the identity it stamped.
func (f *tokenFixture) optionalAuthApp() *fiber.App {
	app := fiber.New()
	echo := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"reached": true, "user_id": c.Locals("user_id")}) }
	app.Get("/api/v1/invitations/preview", OptionalAuth(f.keys, f.blacklist), echo)
	app.Post("/api/v1/invitations/accept", OptionalAuth(f.keys, f.blacklist), echo)
	return app
}

// Criterion 3 on the routes outside Protected: a signed-in browser accepting an
// invitation goes through with the double-submit token.
func TestOptionalAuth_CSRF_Success(t *testing.T) {
	f := newTokenFixture(t)
	app := f.optionalAuthApp()
	access, _ := f.mint(t, authpkg.TokenTypeAccess)

	status, body := send(t, app, http.MethodPost, "/api/v1/invitations/accept", credential{cookie: access, csrf: true})
	assert.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, f.user.String(), body["user_id"])
}

// The forged request: the browser attaches the session cookie to a POST another
// page started, and that page cannot read the CSRF cookie to echo it.
func TestOptionalAuth_CSRF_Unauthorized(t *testing.T) {
	f := newTokenFixture(t)
	app := f.optionalAuthApp()
	access, _ := f.mint(t, authpkg.TokenTypeAccess)

	status, body := send(t, app, http.MethodPost, "/api/v1/invitations/accept", credential{cookie: access})
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "CSRF_FAILED", body["code"])
	assert.Nil(t, body["reached"], "the handler ran for a forged request")
}

// What the check must not touch: a bearer header (no browser attaches one on
// its own), a safe method, an anonymous invitee, and a dead cookie, which
// carries no identity to abuse and goes on anonymously as before.
func TestOptionalAuth_CSRF_NotFound(t *testing.T) {
	f := newTokenFixture(t)
	app := f.optionalAuthApp()
	access, _ := f.mint(t, authpkg.TokenTypeAccess)

	status, body := send(t, app, http.MethodPost, "/api/v1/invitations/accept", credential{bearer: access})
	assert.Equal(t, http.StatusOK, status, "bearer: %v", body)
	assert.Equal(t, f.user.String(), body["user_id"])

	status, body = send(t, app, http.MethodGet, "/api/v1/invitations/preview", credential{cookie: access})
	assert.Equal(t, http.StatusOK, status, "GET: %v", body)
	assert.Equal(t, f.user.String(), body["user_id"])

	status, body = send(t, app, http.MethodPost, "/api/v1/invitations/accept", credential{})
	assert.Equal(t, http.StatusOK, status, "anonymous: %v", body)
	assert.Nil(t, body["user_id"])

	status, body = send(t, app, http.MethodPost, "/api/v1/invitations/accept", credential{cookie: "not-a-jwt"})
	assert.Equal(t, http.StatusOK, status, "dead cookie: %v", body)
	assert.Nil(t, body["user_id"])
}
