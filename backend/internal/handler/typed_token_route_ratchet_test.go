// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #944 — a typed token (MFA_REQUIRED, MFA_ENROLLMENT) opens exactly three
// routes, plus logout, which accepts one only to revoke it.
//
// MFA_REQUIRED is earned by the password alone; MFA_ENROLLMENT by the password
// of an account that must still enrol. Both are signed with the session key.
// Before #944 the Protected gate never read the type, so either token opened
// the whole API — PAT minting and MFA disable included.
//
// Two rules hold that shut, both read from the composition root:
//
//  1. Only /auth/mfa/challenge, /auth/mfa/setup and /auth/mfa/verify use a
//     middleware that accepts a typed token, and all three are registered
//     BEFORE `api.Use(Protected)`. `api.Use` mounts on the /api/v1 prefix, so a
//     route registered after it runs behind Protected whatever group it sits on.
//  2. Every route mounted behind Protected answers 401 TOKEN_INVALID to both
//     typed tokens and still serves a full access token. The route list is
//     parsed from main.go, so a route added tomorrow is covered without anyone
//     editing this file.
//
// Parser limits, so nobody trusts it further than it reaches: it reads main.go
// only, and only `<recv>.Get|Post|Put|Patch|Delete|All("literal", …)` calls.
// Rule 2 sees routes on `protected` and on `api`; routes on the protected
// sub-groups (incidents, notifications, score engine, rbac) are not listed,
// though at runtime they sit behind the same prefix-mounted gate.
// ---------------------------------------------------------------------------

// typedTokenRoutes is the closed set of routes allowed to accept a typed token.
// It may only shrink.
var typedTokenRoutes = []string{
	"POST /auth/mfa/challenge",
	"POST /auth/mfa/setup",
	"POST /auth/mfa/verify",
}

// typedTokenAcceptorFuncs are the functions that validate a token without
// refusing the typed ones (middleware.* and authpkg.ValidateToken).
const typedTokenAcceptorFuncs = `MFATokenMiddleware|MFAEnrollmentMiddleware|ValidateToken`

// typedTokenAcceptors matches a registration that reaches a typed-token
// acceptor, by call or through any variable main.go binds one to — the way
// mfaEnrollmentGuard is bound today. Matching the literal names alone would let
// `guard := middleware.MFATokenMiddleware(…)` open a fourth route unseen.
func typedTokenAcceptors(t *testing.T, src string) *regexp.Regexp {
	t.Helper()
	names := []string{`\b(?:middleware|authpkg)\.(?:` + typedTokenAcceptorFuncs + `)\(`}
	bind := regexp.MustCompile(`(\w+)\s*:?=\s*(?:middleware|authpkg)\.(?:` + typedTokenAcceptorFuncs + `)\(`)
	for _, m := range bind.FindAllStringSubmatch(src, -1) {
		names = append(names, `\b`+regexp.QuoteMeta(m[1])+`\b`)
	}
	return regexp.MustCompile(strings.Join(names, "|"))
}

type registration struct {
	method string
	path   string
	body   string
	offset int
}

// parseRegistrations returns every `<recv>.<Method>("path", …)` call in main.go,
// read to its closing paren so multi-line registrations are seen whole.
func parseRegistrations(t *testing.T, src string) []registration {
	t.Helper()
	call := regexp.MustCompile(`\b(\w+)\.(Get|Post|Put|Patch|Delete|All)\(\s*"([^"]*)"`)
	var out []registration
	for _, loc := range call.FindAllStringSubmatchIndex(src, -1) {
		depth, i := 1, strings.Index(src[loc[0]:], "(")+loc[0]+1
		for i < len(src) && depth > 0 {
			switch src[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
			i++
		}
		out = append(out, registration{
			method: strings.ToUpper(src[loc[4]:loc[5]]),
			path:   src[loc[6]:loc[7]],
			body:   src[loc[0]:i],
			offset: loc[0],
		})
	}
	return out
}

func readMainGo(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	require.NoError(t, err)
	return string(raw)
}

// protectedGateOffset is where `api.Use(... Protected ...)` sits in main.go.
func protectedGateOffset(t *testing.T, src string) int {
	t.Helper()
	gate := regexp.MustCompile(`api\.Use\(\s*middleware\.Protected\(`).FindStringIndex(src)
	require.NotNil(t, gate, "api.Use(middleware.Protected(...)) not found — the parser is broken, not the server")
	return gate[0]
}

// Rule 1.
func TestTypedTokenRoutes_OnlyTheMFAStepsAcceptATypedToken(t *testing.T) {
	src := readMainGo(t)
	gate := protectedGateOffset(t, src)

	acceptors := typedTokenAcceptors(t, src)
	var accepting []string
	for _, r := range parseRegistrations(t, src) {
		if !acceptors.MatchString(r.body) {
			continue
		}
		key := r.method + " " + r.path
		accepting = append(accepting, key)
		assert.Less(t, r.offset, gate,
			"%s accepts a typed token but is registered after api.Use(Protected): Protected refuses typed tokens, so this route is either dead or Protected has been weakened", key)
	}
	sort.Strings(accepting)

	want := append([]string(nil), typedTokenRoutes...)
	sort.Strings(want)
	assert.Equal(t, want, accepting,
		"the set of routes that accept an MFA_REQUIRED or MFA_ENROLLMENT token changed. A typed token is earned without the second factor: a new entry is a review decision, not a refactor")
}

// The alias detection is the part a refactor would quietly break, so it is
// pinned on a synthetic composition root.
func TestTypedTokenRoutes_AcceptorAliasIsCaught(t *testing.T) {
	src := `challengeGuard := middleware.MFATokenMiddleware(rsaKeys, bl)
	api.Post("/auth/pat/bootstrap", challengeGuard, h.Bootstrap)
	api.Post("/auth/mfa/challenge", middleware.MFATokenMiddleware(rsaKeys, bl), h.Challenge)
	api.Get("/plain", h.Plain)`
	acceptors := typedTokenAcceptors(t, src)
	var hit []string
	for _, r := range parseRegistrations(t, src) {
		if acceptors.MatchString(r.body) {
			hit = append(hit, r.method+" "+r.path)
		}
	}
	assert.Equal(t, []string{"POST /auth/pat/bootstrap", "POST /auth/mfa/challenge"}, hit)
}

// typeAgnosticCallers are the only files allowed to validate a token without
// refusing the typed ones. Rule 1 reads main.go; a handler mounted elsewhere
// (the SSE stream validates its own ?token=) could call ValidateToken in its
// own file and slip past it. This list may only shrink.
var typeAgnosticCallers = []string{
	"internal/handler/auth/handler.go",      // logout: revokes whatever it is shown
	"internal/middleware/auth.go",           // MFATokenMiddleware: asserts MFA_REQUIRED
	"internal/middleware/mfa_enrollment.go", // asserts access or MFA_ENROLLMENT
}

func TestTypedTokenRoutes_ValidateTokenHasNoNewCaller(t *testing.T) {
	root := filepath.Join("..", "..")
	var callers []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), "authpkg.ValidateToken(") {
			rel, _ := filepath.Rel(root, path)
			callers = append(callers, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(callers)
	assert.Equal(t, typeAgnosticCallers, callers,
		"authpkg.ValidateToken skips the token-type check. Anything that grants a session must use ValidateAccessToken; a new caller here is a review decision")
}

// gatedRoutes lists every route that runs behind the Protected gate: the
// `protected.*` registrations (the same parse the authz coverage ratchet uses)
// plus the routes on `api` that carry Protected inline, such as /auth/me.
func gatedRoutes(t *testing.T) []registration {
	t.Helper()
	src := readMainGo(t)
	gate := protectedGateOffset(t, src)

	seen := map[string]bool{}
	var out []registration
	add := func(method, path string) {
		key := method + " " + path
		if path == "?" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, registration{method: method, path: path})
	}
	for _, r := range parseProtectedRoutes(t) {
		add(strings.ToUpper(r.method), r.path)
	}
	for _, r := range parseRegistrations(t, src) {
		if strings.HasPrefix(r.body, "api.") && (r.offset > gate || strings.Contains(r.body, "middleware.Protected(")) {
			add(r.method, r.path)
		}
	}
	return out
}

// Rule 2, and criteria 1-3 of #944 route by route.
func TestTypedTokenRoutes_ProtectedRefusesTypedTokensOnEveryRoute(t *testing.T) {
	routes := gatedRoutes(t)

	// The routes the issue names must be among those parsed; if one is not, the
	// parser has lost it and the loop below would prove nothing about it.
	index := map[string]bool{}
	for _, r := range routes {
		index[r.method+" "+r.path] = true
	}
	for _, must := range []string{
		"GET /auth/me", "GET /auth/sessions", "GET /users/me",
		"POST /auth/pat", "GET /auth/pat", "DELETE /auth/pat/:id",
		"POST /auth/mfa/disable",
	} {
		require.True(t, index[must], "%s is no longer seen behind Protected — the parser is broken, or the route moved out from behind the gate", must)
	}
	for _, typed := range typedTokenRoutes {
		assert.False(t, index[typed], "%s must be registered before Protected, which refuses the token it exists for", typed)
	}

	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey}
	user, tenant := uuid.New(), uuid.New()
	mint := func(typ string) string {
		var perms []string
		if typ == authpkg.TokenTypeAccess {
			perms = []string{"*"}
		}
		tok, _, err := authpkg.GenerateTypedToken(keys, user, tenant,
			map[uuid.UUID]string{tenant: "root"}, perms, nil, 5*time.Minute, typ)
		require.NoError(t, err)
		return tok
	}
	noBlacklist := func(string) (bool, error) { return false, nil }

	// The gate exactly as main.go mounts it, with an echo behind every parsed
	// route. Only the gate is under test, so each route's own guards are left
	// out: what matters is whether the request gets past Protected at all.
	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(middleware.Protected(keys, noBlacklist))
	reached := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"reached": true}) }
	for _, r := range routes {
		api.Add(r.method, r.path, reached)
	}

	concrete := regexp.MustCompile(`:\w+\??|\*`)
	send := func(method, path, token string) (int, map[string]any) {
		req := httptest.NewRequest(method, "/api/v1"+concrete.ReplaceAllString(path, uuid.NewString()), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		out := map[string]any{}
		_ = json.Unmarshal(raw, &out)
		return resp.StatusCode, out
	}

	access := mint(authpkg.TokenTypeAccess)
	typed := map[string]string{
		authpkg.TokenTypeMFARequired:   mint(authpkg.TokenTypeMFARequired),
		authpkg.TokenTypeMFAEnrollment: mint(authpkg.TokenTypeMFAEnrollment),
	}

	for _, r := range routes {
		key := r.method + " " + r.path
		for typ, tok := range typed {
			status, body := send(r.method, r.path, tok)
			assert.Equal(t, http.StatusUnauthorized, status, "%s accepted a %s token", key, typ)
			assert.Equal(t, "TOKEN_INVALID", body["code"], "%s with a %s token", key, typ)
			assert.Nil(t, body["reached"], "%s ran its handler for a %s token", key, typ)
		}
		status, body := send(r.method, r.path, access)
		assert.Equal(t, http.StatusOK, status, "%s refused a full access token: %v", key, body)
	}
	t.Logf("%d routes behind Protected checked against MFA_REQUIRED, MFA_ENROLLMENT and a full access token", len(routes))
}
