// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/auth"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #944 — the Protected gate accepts a full access token and nothing else.
//
// After a correct password, login hands out an MFA_REQUIRED token meant only
// for POST /auth/mfa/challenge (or, for a role that must enrol, an
// MFA_ENROLLMENT token meant only for /auth/mfa/setup and /auth/mfa/verify).
// Both are signed with the session key. Protected used to check signature,
// expiry and blacklist but never the type, so a password alone opened the whole
// API: mint a PAT, turn MFA off.
// ---------------------------------------------------------------------------

type tokenFixture struct {
	keys    *authpkg.RSAKeys
	user    uuid.UUID
	tenant  uuid.UUID
	revoked map[string]bool
}

func newTokenFixture(t *testing.T) *tokenFixture {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &tokenFixture{
		keys:    &authpkg.RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey},
		user:    uuid.New(),
		tenant:  uuid.New(),
		revoked: map[string]bool{},
	}
}

func (f *tokenFixture) blacklist(jti string) (bool, error) { return f.revoked[jti], nil }

// mint issues a token of the given type the way the token manager does: access
// tokens carry permissions, typed tokens carry none.
func (f *tokenFixture) mint(t *testing.T, typ string) (token, jti string) {
	t.Helper()
	var perms []string
	if typ == authpkg.TokenTypeAccess {
		perms = []string{"*"}
	}
	token, jti, err := authpkg.GenerateTypedToken(f.keys, f.user, f.tenant,
		map[uuid.UUID]string{f.tenant: "admin"}, perms, nil, 5*time.Minute, typ)
	require.NoError(t, err)
	return token, jti
}

// protectedApp mounts the chain main.go uses on /api/v1: PATMiddleware, then
// Protected, then a route that reports what the gate published.
func (f *tokenFixture) protectedApp(t *testing.T, patSvc *auth.PersonalAccessTokenService, resolve auth.OrgSessionResolver) *fiber.App {
	t.Helper()
	app := fiber.New()
	api := app.Group("/api/v1")
	if patSvc != nil {
		api.Use(PATMiddleware(patSvc, resolve))
	}
	api.Use(Protected(f.keys, f.blacklist))
	echo := func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"reached": true, "user_id": c.Locals("user_id"), "tenant_id": c.Locals("tenant_id")})
	}
	api.Get("/auth/me", echo)
	api.Post("/auth/pat", echo)
	return app
}

type credential struct {
	bearer string
	cookie string
	csrf   bool
}

func send(t *testing.T, app *fiber.App, method, path string, cred credential) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if cred.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+cred.bearer)
	}
	if cred.cookie != "" {
		req.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: cred.cookie})
	}
	if cred.csrf {
		req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "csrf-value"})
		req.Header.Set(CSRFHeader, "csrf-value")
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestProtected_Success(t *testing.T) {
	f := newTokenFixture(t)
	app := f.protectedApp(t, nil, nil)
	access, _ := f.mint(t, authpkg.TokenTypeAccess)

	for name, cred := range map[string]credential{
		"bearer":      {bearer: access},
		"cookie+csrf": {cookie: access, csrf: true},
	} {
		t.Run(name, func(t *testing.T) {
			for _, m := range []string{http.MethodGet, http.MethodPost} {
				path := map[string]string{http.MethodGet: "/api/v1/auth/me", http.MethodPost: "/api/v1/auth/pat"}[m]
				status, body := send(t, app, m, path, cred)
				require.Equal(t, http.StatusOK, status, "%s %s: %v", m, path, body)
				assert.Equal(t, f.user.String(), body["user_id"])
				assert.Equal(t, f.tenant.String(), body["tenant_id"])
			}
		})
	}

	// The cookie path keeps its CSRF check: the access-type check sits after it,
	// not instead of it.
	status, body := send(t, app, http.MethodPost, "/api/v1/auth/pat", credential{cookie: access})
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "CSRF_FAILED", body["code"])
}

// A PAT authenticates through PATMiddleware upstream; the type check in the JWT
// gate must not touch it (criterion 3).
func TestProtected_PATUnchanged(t *testing.T) {
	f := newTokenFixture(t)
	svc := auth.NewPersonalAccessTokenService(&patRepoFake{})
	resolve := func(_ context.Context, uid, orgID uuid.UUID) (*auth.SessionClaims, error) {
		if uid != f.user || orgID != f.tenant {
			return nil, fmt.Errorf("not a member")
		}
		return &auth.SessionClaims{TenantID: orgID, Permissions: []string{"risks:read"}}, nil
	}
	app := f.protectedApp(t, svc, resolve)
	_, raw, err := svc.CreateToken(context.Background(), f.user, f.tenant, "CI", "", []string{"*"}, nil)
	require.NoError(t, err)

	status, body := send(t, app, http.MethodGet, "/api/v1/auth/me", credential{bearer: raw})
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, f.tenant.String(), body["tenant_id"])
}

// "Not found" for a bearer credential: a token the server does not recognise —
// its JTI revoked, or signed by a key that is not ours.
func TestProtected_NotFound(t *testing.T) {
	f := newTokenFixture(t)
	app := f.protectedApp(t, nil, nil)

	revoked, jti := f.mint(t, authpkg.TokenTypeAccess)
	f.revoked[jti] = true
	status, body := send(t, app, http.MethodGet, "/api/v1/auth/me", credential{bearer: revoked})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "TOKEN_REVOKED", body["code"])
	assert.Nil(t, body["reached"])

	foreign := newTokenFixture(t)
	alien, _ := foreign.mint(t, authpkg.TokenTypeAccess)
	status, body = send(t, app, http.MethodGet, "/api/v1/auth/me", credential{bearer: alien})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "TOKEN_INVALID", body["code"])
	assert.Nil(t, body["reached"])
}

// THE bug (criteria 1 and 2): neither typed token passes the gate, by header or
// by cookie, on a read or on a write.
func TestProtected_Unauthorized(t *testing.T) {
	f := newTokenFixture(t)
	app := f.protectedApp(t, nil, nil)

	for _, typ := range []string{authpkg.TokenTypeMFARequired, authpkg.TokenTypeMFAEnrollment} {
		tok, _ := f.mint(t, typ)
		for name, cred := range map[string]credential{
			"bearer":      {bearer: tok},
			"cookie+csrf": {cookie: tok, csrf: true},
		} {
			for _, rt := range [][2]string{{http.MethodGet, "/api/v1/auth/me"}, {http.MethodPost, "/api/v1/auth/pat"}} {
				t.Run(typ+"/"+name+"/"+rt[0]+rt[1], func(t *testing.T) {
					status, body := send(t, app, rt[0], rt[1], cred)
					assert.Equal(t, http.StatusUnauthorized, status)
					assert.Equal(t, "TOKEN_INVALID", body["code"])
					assert.Nil(t, body["reached"], "the handler must never run")
				})
			}
		}
	}

	status, body := send(t, app, http.MethodGet, "/api/v1/auth/me", credential{})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "UNAUTHORIZED", body["code"])
}

// Criterion 4: the challenge accepts MFA_REQUIRED and only MFA_REQUIRED.
func TestMFATokenMiddleware_AcceptsOnlyMFARequired(t *testing.T) {
	f := newTokenFixture(t)
	app := fiber.New()
	app.Post("/api/v1/auth/mfa/challenge", MFATokenMiddleware(f.keys, f.blacklist), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"reached": true, "mfa_required": c.Locals("mfa_required")})
	})

	mfa, mfaJTI := f.mint(t, authpkg.TokenTypeMFARequired)
	status, body := send(t, app, http.MethodPost, "/api/v1/auth/mfa/challenge", credential{bearer: mfa})
	require.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, true, body["mfa_required"])

	for _, typ := range []string{authpkg.TokenTypeAccess, authpkg.TokenTypeMFAEnrollment} {
		tok, _ := f.mint(t, typ)
		status, body := send(t, app, http.MethodPost, "/api/v1/auth/mfa/challenge", credential{bearer: tok})
		assert.Equal(t, http.StatusUnauthorized, status, "type %q", typ)
		assert.Nil(t, body["reached"], "type %q", typ)
	}

	f.revoked[mfaJTI] = true
	status, body = send(t, app, http.MethodPost, "/api/v1/auth/mfa/challenge", credential{bearer: mfa})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "TOKEN_REVOKED", body["code"])
}

// Enrolment accepts a full access token and MFA_ENROLLMENT, never MFA_REQUIRED
// (which would let a stolen password overwrite the victim's authenticator).
func TestMFAEnrollmentMiddleware_TokenTypes(t *testing.T) {
	f := newTokenFixture(t)
	app := fiber.New()
	app.Post("/api/v1/auth/mfa/setup", MFAEnrollmentMiddleware(f.keys, f.blacklist), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"reached": true, "user_id": c.Locals("user_id"), "tenant_id": c.Locals("tenant_id"),
			"mfa_enrollment_token": c.Locals("mfa_enrollment_token"),
		})
	})

	for typ, mandated := range map[string]bool{authpkg.TokenTypeAccess: false, authpkg.TokenTypeMFAEnrollment: true} {
		tok, _ := f.mint(t, typ)
		status, body := send(t, app, http.MethodPost, "/api/v1/auth/mfa/setup", credential{bearer: tok})
		require.Equal(t, http.StatusOK, status, "type %q: %v", typ, body)
		assert.Equal(t, f.user.String(), body["user_id"])
		assert.Equal(t, f.tenant.String(), body["tenant_id"])
		assert.Equal(t, mandated, body["mfa_enrollment_token"])
	}

	mfa, _ := f.mint(t, authpkg.TokenTypeMFARequired)
	status, body := send(t, app, http.MethodPost, "/api/v1/auth/mfa/setup", credential{bearer: mfa})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Nil(t, body["reached"])
}

// Enrolment is mounted outside Protected now (#944), so the CSRF check Protected
// used to apply to a cookie-authenticated call must be applied here instead.
func TestMFAEnrollmentMiddleware_CookieRequiresCSRF(t *testing.T) {
	f := newTokenFixture(t)
	app := fiber.New()
	app.Post("/api/v1/auth/mfa/setup", MFAEnrollmentMiddleware(f.keys, f.blacklist), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"reached": true})
	})
	access, _ := f.mint(t, authpkg.TokenTypeAccess)

	status, body := send(t, app, http.MethodPost, "/api/v1/auth/mfa/setup", credential{cookie: access})
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "CSRF_FAILED", body["code"])
	assert.Nil(t, body["reached"])

	status, body = send(t, app, http.MethodPost, "/api/v1/auth/mfa/setup", credential{cookie: access, csrf: true})
	assert.Equal(t, http.StatusOK, status, "%v", body)
}

// OptionalAuth stamps identity only from a full session; a typed token is
// treated as no token at all.
func TestOptionalAuth_TypedTokenIsAnonymous(t *testing.T) {
	f := newTokenFixture(t)
	app := fiber.New()
	app.Get("/api/v1/invitations/preview", OptionalAuth(f.keys, f.blacklist), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"user_id": c.Locals("user_id")})
	})

	access, _ := f.mint(t, authpkg.TokenTypeAccess)
	_, body := send(t, app, http.MethodGet, "/api/v1/invitations/preview", credential{bearer: access})
	assert.Equal(t, f.user.String(), body["user_id"])

	for _, typ := range []string{authpkg.TokenTypeMFARequired, authpkg.TokenTypeMFAEnrollment} {
		tok, _ := f.mint(t, typ)
		status, body := send(t, app, http.MethodGet, "/api/v1/invitations/preview", credential{bearer: tok})
		assert.Equal(t, http.StatusOK, status)
		assert.Nil(t, body["user_id"], "type %q must not stamp an identity", typ)
	}
}
