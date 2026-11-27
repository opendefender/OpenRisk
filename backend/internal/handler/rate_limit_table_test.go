// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/middleware"
)

// ---------------------------------------------------------------------------
// #1318 — every unauthenticated or credential-bearing route has a rate limit,
// and docs/security/RATE_LIMITS.md says which.
//
// The routes are parsed from main.go with parseRegistrations (see
// typed_token_route_ratchet_test.go for its limits). In scope: every route
// registered before `api.Use(middleware.Protected(…))`, and every /auth/ route
// wherever it sits. Each must have a row in the table; each row's limiters must
// reach the route and carry the budget the code applies; a `secret` route must
// carry a limiter of its own, not only the baseline quotas.
// ---------------------------------------------------------------------------

const rateLimitTablePath = "../../../docs/security/RATE_LIMITS.md"

// baselineLimiters apply to whole prefixes. They never satisfy the rule that a
// `secret` route has a limiter of its own.
var baselineLimiters = map[string]bool{"ipQuota": true, "tenantRateLimit": true}

type limitRow struct {
	route    string            // "POST /api/v1/auth/login"
	kind     string            // public | token | secret | session
	limiters map[string]string // identifier -> "15/5m"
}

type routeReg struct {
	registration
	recv string
	full string // path as a client sends it
}

var (
	limitRowRe   = regexp.MustCompile("^\\|\\s*`([A-Z]+ /[^`]*)`\\s*\\|\\s*(\\w+)\\s*\\|([^|]*)\\|")
	limiterRe    = regexp.MustCompile("`([\\w.]+)`\\s+(\\d+/\\d+[smh])")
	validKinds   = map[string]bool{"public": true, "token": true, "secret": true, "session": true}
	receiverRe   = regexp.MustCompile(`^(\w+)\.`)
	apiIPQuotaRe = regexp.MustCompile(`api\.Use\(\s*ipQuota\s*\)`)
	tenantUseRe  = regexp.MustCompile(`protected\.Use\(\s*tenantRateLimit\s*\)`)
)

func parseLimitTable(t *testing.T, doc string) []limitRow {
	t.Helper()
	var rows []limitRow
	for _, line := range strings.Split(doc, "\n") {
		m := limitRowRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		row := limitRow{route: m[1], kind: m[2], limiters: map[string]string{}}
		for _, l := range limiterRe.FindAllStringSubmatch(m[3], -1) {
			row.limiters[l[1]] = l[2]
		}
		rows = append(rows, row)
	}
	return rows
}

func fmtBudget(n int, window time.Duration) string {
	switch {
	case window%time.Hour == 0:
		return fmt.Sprintf("%d/%dh", n, window/time.Hour)
	case window%time.Minute == 0:
		return fmt.Sprintf("%d/%dm", n, window/time.Minute)
	}
	return fmt.Sprintf("%d/%ds", n, window/time.Second)
}

// expectedBudgets is what the code applies, per limiter identifier: the auth
// constants, the quota defaults, and the questionnaire budget read from its
// binding in main.go.
func expectedBudgets(t *testing.T, src string) map[string]string {
	t.Helper()
	t.Setenv("RATE_LIMIT_IP_PER_MINUTE", "")
	t.Setenv("RATE_LIMIT_TENANT_PER_MINUTE", "")
	q := middleware.LoadQuotaConfig()
	out := map[string]string{
		"ipQuota":                 fmtBudget(q.IPRequestsPerMinute, time.Minute),
		"tenantRateLimit":         fmtBudget(q.TenantRequestsPerMinute, time.Minute),
		"authLimits.Login":        fmtBudget(middleware.AuthLoginLimit, middleware.AuthLoginWindow),
		"authLimits.Register":     fmtBudget(middleware.AuthRegisterLimit, middleware.AuthRegisterWindow),
		"authLimits.Reset":        fmtBudget(middleware.AuthResetLimit, middleware.AuthResetWindow),
		"authLimits.Check":        fmtBudget(middleware.AuthCheckLimit, middleware.AuthCheckWindow),
		"authLimits.Reauth":       fmtBudget(middleware.AuthReauthLimit, middleware.AuthReauthWindow),
		"authLimits.MFAChallenge": fmtBudget(middleware.AuthMFAChallengeLimit, middleware.AuthMFAChallengeWindow),
		"vendorPublicRateLimit":   "",
	}
	// Every AuthLimiters field must have a budget above, or a new limiter
	// could be named in the table and never checked.
	for i, typ := 0, reflect.TypeOf(middleware.AuthLimiters{}); i < typ.NumField(); i++ {
		_, ok := out["authLimits."+typ.Field(i).Name]
		assert.True(t, ok, "middleware.AuthLimiters.%s has no expected budget in this test", typ.Field(i).Name)
	}
	vendor := regexp.MustCompile(`vendorPublicRateLimit\s*:=\s*middleware\.RateLimit\(middleware\.RateLimitConfig\{\s*MaxRequests:\s*(\d+),\s*WindowSize:\s*time\.(Minute|Hour)`).
		FindStringSubmatch(src)
	if vendor != nil {
		n, _ := strconv.Atoi(vendor[1])
		w := time.Minute
		if vendor[2] == "Hour" {
			w = time.Hour
		}
		out["vendorPublicRateLimit"] = fmtBudget(n, w)
	}
	return out
}

// limiterBindings are how each limiter identifier must be built in main.go,
// so a name in the table is a real limiter and not a pass-through.
var limiterBindings = map[string]*regexp.Regexp{
	"ipQuota":               regexp.MustCompile(`\bipQuota\s*:=\s*middleware\.IPRateLimit\(`),
	"tenantRateLimit":       regexp.MustCompile(`\btenantRateLimit\s*:=\s*middleware\.TenantRateLimit\(`),
	"authLimits":            regexp.MustCompile(`\bauthLimits\s*:=\s*middleware\.NewAuthLimiters\(`),
	"vendorPublicRateLimit": regexp.MustCompile(`\bvendorPublicRateLimit\s*:=\s*middleware\.RateLimit\(`),
}

// checkRateLimitTable returns one problem per broken rule. budgets may be nil
// to skip the budget check (the synthetic cases below).
func checkRateLimitTable(t *testing.T, src string, rows []limitRow, budgets map[string]string) []string {
	t.Helper()
	var problems []string
	gate := protectedGateOffset(t, src)

	ipUse := len(src)
	if loc := apiIPQuotaRe.FindStringIndex(src); loc != nil {
		ipUse = loc[0]
	}
	tenantUse := len(src)
	if loc := tenantUseRe.FindStringIndex(src); loc != nil {
		tenantUse = loc[0]
	}

	// Routes in scope.
	inScope := map[string]routeReg{}
	for _, r := range parseRegistrations(t, src) {
		if !strings.HasPrefix(r.path, "/") {
			continue // c.Get("User-Agent") and friends
		}
		m := receiverRe.FindStringSubmatch(r.body)
		reg := routeReg{registration: r, recv: m[1]}
		switch reg.recv {
		case "app":
			reg.full = r.path
		case "api", "protected":
			reg.full = "/api/v1" + r.path
		default:
			if r.offset < gate {
				problems = append(problems, fmt.Sprintf(
					"%s %s is registered on %q before the auth gate; this test cannot resolve its prefix — register it on app or api", r.method, r.path, reg.recv))
			}
			continue
		}
		if r.offset < gate || strings.Contains(reg.full, "/auth/") {
			inScope[r.method+" "+reg.full] = reg
		}
	}

	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.route] {
			problems = append(problems, row.route+": listed twice")
		}
		seen[row.route] = true
		reg, ok := inScope[row.route]
		if !ok {
			problems = append(problems, row.route+": in the table but not registered in main.go (or no longer public)")
			continue
		}
		if !validKinds[row.kind] {
			problems = append(problems, fmt.Sprintf("%s: kind %q is not public, token, secret or session", row.route, row.kind))
		}
		if len(row.limiters) == 0 {
			problems = append(problems, row.route+": no limiter")
		}
		own := false
		for name, budget := range row.limiters {
			base := strings.SplitN(name, ".", 2)[0]
			if b, ok := limiterBindings[base]; !ok || !b.MatchString(src) {
				problems = append(problems, fmt.Sprintf("%s: %s is not a limiter built in main.go", row.route, name))
				continue
			}
			if budgets != nil && budgets[name] != budget {
				problems = append(problems, fmt.Sprintf("%s: %s budget is %s in the table, %s in the code", row.route, name, budget, budgets[name]))
			}
			inBody := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(reg.body)
			reaches := inBody
			switch name {
			case "ipQuota":
				reaches = inBody || (strings.HasPrefix(reg.full, "/api/v1/") && reg.offset > ipUse)
			case "tenantRateLimit":
				reaches = inBody || (reg.recv != "app" && reg.offset > tenantUse)
			}
			if !reaches {
				problems = append(problems, fmt.Sprintf("%s: %s does not reach this route in main.go", row.route, name))
			}
			if reaches && !baselineLimiters[name] {
				own = true
			}
		}
		if row.kind == "secret" && !own {
			problems = append(problems, row.route+": a secret route needs a limiter of its own, not only ipQuota/tenantRateLimit")
		}
	}

	var missing []string
	for route := range inScope {
		if !seen[route] {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	for _, route := range missing {
		problems = append(problems, route+": public or /auth/ route with no row in docs/security/RATE_LIMITS.md")
	}
	return problems
}

func TestRateLimitTable_EveryPublicRouteIsListedAndLimited(t *testing.T) {
	src := readMainGo(t)
	raw, err := os.ReadFile(filepath.FromSlash(rateLimitTablePath))
	require.NoError(t, err)
	rows := parseLimitTable(t, string(raw))
	require.NotEmpty(t, rows, "no route row parsed from %s — the parser is broken, not the table", rateLimitTablePath)

	budgets := expectedBudgets(t, src)
	require.NotEmpty(t, budgets["vendorPublicRateLimit"], "vendorPublicRateLimit binding not parsed from main.go")

	for _, p := range checkRateLimitTable(t, src, rows, budgets) {
		t.Error(p)
	}
}

// The rules are pinned on a synthetic composition root, so a refactor that
// silently stops catching something fails here.
func TestRateLimitTable_CatchesEachBrokenRule(t *testing.T) {
	src := `ipQuota := middleware.IPRateLimit(store, cfg)
	authLimits := middleware.NewAuthLimiters(store, cfg)
	app.Get("/metrics", h.Metrics)
	api := app.Group("/api/v1")
	api.Use(ipQuota)
	api.Post("/auth/login", h.Login)
	api.Post("/auth/refresh", h.Refresh)
	app.Post("/api/v1/hook", h.Hook)
	pub := app.Group("/pub")
	pub.Get("/x", h.X)
	api.Use(middleware.Protected(keys, bl))
	protected.Post("/auth/password/change", authLimits.Reauth, h.Change)`
	rows := []limitRow{
		{route: "GET /metrics", kind: "public", limiters: map[string]string{"ipQuota": ""}},
		{route: "POST /api/v1/auth/login", kind: "secret", limiters: map[string]string{"ipQuota": ""}},
		{route: "POST /api/v1/auth/password/change", kind: "secret", limiters: map[string]string{"authLimits.Reauth": ""}},
		{route: "POST /api/v1/hook", kind: "token", limiters: map[string]string{"ipQuota": ""}},
		{route: "GET /api/v1/gone", kind: "public", limiters: map[string]string{"ipQuota": ""}},
	}
	got := strings.Join(checkRateLimitTable(t, src, rows, nil), "\n")

	for _, want := range []string{
		"GET /metrics: ipQuota does not reach this route",                    // outside /api/v1, not in the chain
		"POST /api/v1/auth/login: a secret route needs a limiter of its own", // baseline only
		"GET /api/v1/gone: in the table but not registered",                  // stale row
		"POST /api/v1/auth/refresh: public or /auth/ route with no row",      // missing row
		`GET /x is registered on "pub" before the auth gate`,                 // unresolvable prefix
	} {
		assert.Contains(t, got, want)
	}
	for _, ok := range []string{"POST /api/v1/auth/password/change", "POST /api/v1/hook"} {
		assert.NotContains(t, got, ok+":", "%s is listed and limited", ok)
	}
}
