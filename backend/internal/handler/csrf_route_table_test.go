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
// #1314 — every state-changing route is listed here with the defence that
// stops a cross-origin request riding the browser's session cookie.
//
// The list is closed. The test parses every POST/PUT/PATCH/DELETE/All
// registration in main.go, resolving the prefixes of its route groups, and
// fails when a route is missing from the table, or listed but gone. A new
// mutating route therefore cannot ship until someone has decided which
// defence it relies on.
//
// The defences, and how each is proven:
//
//   viaProtected      registered after `api.Use(Protected)`, whose prefix mount
//                     runs the double-submit check on cookie callers. Proven
//                     here, route by route, through the real gate.
//   viaMFAEnrollment  MFAEnrollmentMiddleware, which repeats the check (#944).
//                     Proven here through the real middleware.
//   viaOptionalAuth   OptionalAuth, which repeats the check once the cookie
//                     carries a live session. Proven here; the anonymous and
//                     bearer paths in middleware/browser_protections_test.go.
//   viaHandler        the handler reads the cookie and checks itself. Proven
//                     in internal/handler/auth/refresh_csrf_e2e_test.go.
//   notCookie(...)    the route reads no session cookie: its credential is a
//                     body, a header or a token a cross-site page cannot
//                     attach for the victim. The reason is part of the entry.
//   acceptedRisk(...) reads a cookie and is left unchecked on purpose; the
//                     reason is part of the entry.
//
// Parser limits: main.go only, `<recv>.<Method>("literal", …)` calls only, on
// app, api, protected and the groups main.go derives from them. Any other
// receiver, and any registration form the parser cannot read (Add, Route,
// Mount, Register*Routes), fails the test rather than being skipped.
// ---------------------------------------------------------------------------

type csrfDefence string

const (
	viaProtected     csrfDefence = "protected"
	viaMFAEnrollment csrfDefence = "mfa-enrollment"
	viaOptionalAuth  csrfDefence = "optional-auth"
	viaHandler       csrfDefence = "handler"
)

func notCookie(reason string) csrfDefence    { return csrfDefence("not-cookie: " + reason) }
func acceptedRisk(reason string) csrfDefence { return csrfDefence("accepted-risk: " + reason) }

// mutatingRouteDefences is the table. Paths are relative to /api/v1.
var mutatingRouteDefences = map[string]csrfDefence{
	// --- Outside the Protected gate --------------------------------------
	"POST /auth/login":                      notCookie("credentials in the body; it reads only the trusted-device cookie. Login CSRF is #1357"),
	"POST /auth/register":                   notCookie("credentials in the body, no session yet"),
	"POST /auth/refresh":                    viaHandler,
	"POST /auth/logout":                     acceptedRisk("a forged logout can only end the victim's own session; refusing a logout would leave a live session behind, which is the worse failure"),
	"POST /auth/password/forgot":            notCookie("email in the body, no session"),
	"POST /auth/password/reset":             notCookie("the reset token in the body is the credential"),
	"POST /auth/password/check":             notCookie("stateless strength check on the body"),
	"POST /auth/saml2/acs":                  notCookie("the IdP's signed assertion is the credential; a cross-site POST is how SAML works"),
	"POST /auth/sso/mfa":                    acceptedRisk("reads the single-use or_sso_mfa cookie before any session, so no CSRF cookie exists yet; a forgery only burns the pending step, and CORS keeps the response unreadable"),
	"POST /auth/mfa/challenge":              notCookie("MFATokenMiddleware reads the Authorization header only"),
	"POST /auth/mfa/setup":                  viaMFAEnrollment,
	"POST /auth/mfa/verify":                 viaMFAEnrollment,
	"POST /invitations/accept":              viaOptionalAuth,
	"PUT /public/vendor-assessment/answers": notCookie("X-Vendor-Assessment-Token header, which no browser attaches on its own"),
	"POST /public/vendor-assessment/submit": notCookie("X-Vendor-Assessment-Token header, which no browser attaches on its own"),
	"POST /scanner/agents/register":         notCookie("scanner registration token in the request"),
	"POST /scanner/agent/push":              notCookie("agent token and HMAC signature"),
	"POST /scanner/agent/inventory":         notCookie("agent token and HMAC signature"),
	"POST /scanner/agent/heartbeat":         notCookie("agent token"),
	"POST /vulnerabilities/webhook/:source": notCookie("the integration's webhook token"),

	// --- Behind the Protected gate ---------------------------------------
	"POST /activation/celebrated":                             viaProtected,
	"POST /ai/assistant/query":                                viaProtected,
	"POST /ai/audits/:id/report":                              viaProtected,
	"POST /ai/emerging-risks":                                 viaProtected,
	"POST /ai/evidence/:id/analyze":                           viaProtected,
	"POST /ai/risks/:id/treatment-plan":                       viaProtected,
	"PUT /analytics/financial/appetite":                       viaProtected,
	"PUT /analytics/financial/currency":                       viaProtected,
	"POST /asset-dependencies":                                viaProtected,
	"DELETE /asset-dependencies/:id":                          viaProtected,
	"POST /assets":                                            viaProtected,
	"POST /assets/bulk/delete":                                viaProtected,
	"POST /assets/bulk/preview":                               viaProtected,
	"DELETE /assets/:id":                                      viaProtected,
	"PATCH /assets/:id":                                       viaProtected,
	"POST /assets/import":                                     viaProtected,
	"POST /attack-surface/draft-risks/review":                 viaProtected,
	"PUT /attack-surface/risk-rule":                           viaProtected,
	"POST /attack-surface/risk-rule/preview":                  viaProtected,
	"PUT /attack-surface/schemas/:category":                   viaProtected,
	"POST /attack-surface/schemas/:category/reset":            viaProtected,
	"POST /auth/mfa/disable":                                  viaProtected,
	"POST /auth/mfa/backup-codes/regenerate":                  viaProtected,
	"POST /auth/password/change":                              viaProtected,
	"POST /auth/pat":                                          viaProtected,
	"DELETE /auth/pat/:id":                                    viaProtected,
	"DELETE /auth/sessions/:id":                               viaProtected,
	"DELETE /auth/sessions/others":                            viaProtected,
	"POST /auth/switch-org":                                   viaProtected,
	"PUT /automation/channels":                                viaProtected,
	"POST /automation/channels/test":                          viaProtected,
	"POST /automation/dry-runs/:id/cancel":                    viaProtected,
	"POST /automation/executions/:id/replay":                  viaProtected,
	"POST /automation/rules":                                  viaProtected,
	"DELETE /automation/rules/:id":                            viaProtected,
	"PUT /automation/rules/:id":                               viaProtected,
	"POST /automation/rules/:id/dry-run":                      viaProtected,
	"POST /automation/rules/:id/enable":                       viaProtected,
	"POST /automation/rules/:id/run":                          viaProtected,
	"POST /automation/rules/:id/suspend":                      viaProtected,
	"POST /automation/templates/:key/adopt":                   viaProtected,
	"POST /billing/cancel":                                    viaProtected,
	"POST /billing/change-plan":                               viaProtected,
	"POST /billing/checkout":                                  viaProtected,
	"POST /billing/trial":                                     viaProtected,
	"POST /bulk-operations":                                   viaProtected,
	"POST /compliance/audits":                                 viaProtected,
	"DELETE /compliance/audits/:id":                           viaProtected,
	"PATCH /compliance/audits/:id":                            viaProtected,
	"POST /compliance/audits/:id/generate-remediations":       viaProtected,
	"POST /compliance/control-mappings":                       viaProtected,
	"DELETE /compliance/control-mappings/:mappingId":          viaProtected,
	"DELETE /compliance/controls/:controlId":                  viaProtected,
	"PATCH /compliance/controls/:controlId":                   viaProtected,
	"POST /compliance/controls/:controlId/evidences":          viaProtected,
	"DELETE /compliance/evidences/:evidenceId":                viaProtected,
	"POST /compliance/frameworks":                             viaProtected,
	"DELETE /compliance/frameworks/:frameworkId":              viaProtected,
	"POST /compliance/frameworks/:frameworkId/controls":       viaProtected,
	"POST /compliance/frameworks/:frameworkId/import-catalog": viaProtected,
	"POST /compliance/remediations":                           viaProtected,
	"DELETE /compliance/remediations/:id":                     viaProtected,
	"PATCH /compliance/remediations/:id":                      viaProtected,
	"POST /cti/match":                                         viaProtected,
	"POST /cti/sync":                                          viaProtected,
	"DELETE /cti/threats/:cve/dismissal":                      viaProtected,
	"PUT /cti/threats/:cve/dismissal":                         viaProtected,
	"POST /custom-fields":                                     viaProtected,
	"DELETE /custom-fields/:id":                               viaProtected,
	"PATCH /custom-fields/:id":                                viaProtected,
	"POST /custom-fields/templates/:id/apply":                 viaProtected,
	"POST /evidence":                                          viaProtected,
	"DELETE /evidence/:evidenceId":                            viaProtected,
	"PATCH /evidence/:evidenceId":                             viaProtected,
	"POST /evidence/:evidenceId/links":                        viaProtected,
	"DELETE /evidence/:evidenceId/links/:controlId":           viaProtected,
	"POST /evidence/:evidenceId/review":                       viaProtected,
	"POST /governance/approvals":                              viaProtected,
	"POST /governance/approvals/:id/cancel":                   viaProtected,
	"POST /governance/approvals/:id/decide":                   viaProtected,
	"POST /governance/approvals/:id/defer":                    viaProtected,
	"PUT /governance/audit-retention":                         viaProtected,
	"POST /governance/audit-retention/apply":                  viaProtected,
	"POST /governance/delegations":                            viaProtected,
	"POST /governance/delegations/:id/revoke":                 viaProtected,
	"POST /governance/workflows":                              viaProtected,
	"DELETE /governance/workflows/:id":                        viaProtected,
	"PUT /governance/workflows/:id":                           viaProtected,
	"POST /incidents":                                         viaProtected,
	"DELETE /incidents/:id":                                   viaProtected,
	"PUT /incidents/:id":                                      viaProtected,
	"POST /incidents/:id/actions":                             viaProtected,
	"PUT /incidents/:id/actions/:actionId":                    viaProtected,
	"PUT /incidents/:id/post-mortem":                          viaProtected,
	"POST /incidents/:id/post-mortem/publish":                 viaProtected,
	"POST /incidents/:id/risks/:riskId":                       viaProtected,
	"POST /incidents/:id/transfer-owner":                      viaProtected,
	"POST /integrations/:id/test":                             viaProtected,
	"DELETE /mitigations/:id":                                 viaProtected,
	"PATCH /mitigations/:id":                                  viaProtected,
	"PATCH /mitigations/:id/reorder-subactions":               viaProtected,
	"POST /mitigations/:id/sub-actions":                       viaProtected,
	"DELETE /mitigations/:id/sub-actions/:aid":                viaProtected,
	"PATCH /mitigations/:id/sub-actions/:aid":                 viaProtected,
	"POST /mitigations/:id/sub-actions/:aid/complete":         viaProtected,
	"POST /mitigations/:id/sub-actions/:aid/revert":           viaProtected,
	"POST /mitigations/:id/transfer-owner":                    viaProtected,
	"PATCH /mitigations/:id/validate":                         viaProtected,
	"PUT /notifications/email-policy":                         viaProtected,
	"DELETE /notifications/:notificationId":                   viaProtected,
	"PATCH /notifications/:notificationId/read":               viaProtected,
	"PATCH /notifications/preferences":                        viaProtected,
	"PATCH /notifications/read-all":                           viaProtected,
	"POST /notifications/test":                                viaProtected,
	"POST /onboarding/complete":                               viaProtected,
	"POST /onboarding/starter-risks":                          viaProtected,
	"PUT /onboarding/steps/:step":                             viaProtected,
	"PUT /organization":                                       viaProtected,
	"POST /organization/deletion":                             viaProtected,
	"POST /organization/deletion/cancel":                      viaProtected,
	"POST /organization/invitations":                          viaProtected,
	"DELETE /organization/invitations/:invitationId":          viaProtected,
	"POST /organization/invitations/:invitationId/resend":     viaProtected,
	"DELETE /organization/logo":                               viaProtected,
	"PUT /organization/logo":                                  viaProtected,
	"PUT /organization/members/:memberId/role":                viaProtected,
	"PUT /organization/members/:memberId/status":              viaProtected,
	"POST /rbac/roles":                                        viaProtected,
	"DELETE /rbac/roles/:role_id":                             viaProtected,
	"PATCH /rbac/roles/:role_id":                              viaProtected,
	"DELETE /rbac/roles/:role_id/permissions":                 viaProtected,
	"POST /rbac/roles/:role_id/permissions":                   viaProtected,
	"POST /rbac/tenants":                                      viaProtected,
	"DELETE /rbac/tenants/:tenant_id":                         viaProtected,
	"PATCH /rbac/tenants/:tenant_id":                          viaProtected,
	"POST /rbac/users":                                        viaProtected,
	"DELETE /rbac/users/:user_id":                             viaProtected,
	"PATCH /rbac/users/:user_id/role":                         viaProtected,
	"POST /reports":                                           viaProtected,
	"POST /reports/board":                                     viaProtected,
	"DELETE /reports/board/:reportId":                         viaProtected,
	"PATCH /reports/board/:reportId":                          viaProtected,
	"POST /reports/board/:reportId/approve":                   viaProtected,
	"POST /reports/jobs":                                      viaProtected,
	"DELETE /reports/:reportId":                               viaProtected,
	"POST /reports/:reportId/comments":                        viaProtected,
	"POST /reports/:reportId/transition":                      viaProtected,
	"PUT /risk-acceptance/policy":                             viaProtected,
	"POST /risk-categories":                                   viaProtected,
	"DELETE /risk-categories/:id":                             viaProtected,
	"PATCH /risk-categories/:id":                              viaProtected,
	"POST /risks":                                             viaProtected,
	"POST /risks/bulk":                                        viaProtected,
	"POST /risks/bulk/add-tags":                               viaProtected,
	"POST /risks/bulk/change-status":                          viaProtected,
	"POST /risks/bulk/delete":                                 viaProtected,
	"POST /risks/bulk/preview":                                viaProtected,
	"POST /risks/bulk/remove-tags":                            viaProtected,
	"PUT /risk-scoring/weights":                               viaProtected,
	"DELETE /risks/:id":                                       viaProtected,
	"PATCH /risks/:id":                                        viaProtected,
	"POST /risks/:id/acceptance/renew":                        viaProtected,
	"POST /risks/:id/control-mappings":                        viaProtected,
	"DELETE /risks/:id/control-mappings/:mappingId":           viaProtected,
	"POST /risks/:id/mitigations":                             viaProtected,
	"POST /risks/:id/review":                                  viaProtected,
	"POST /risks/:id/simulate":                                viaProtected,
	"POST /risks/:id/smart-score/simulate":                    viaProtected,
	"POST /risks/:id/transfer-owner":                          viaProtected,
	"POST /risks/:id/transition":                              viaProtected,
	"POST /risks/import":                                      viaProtected,
	"POST /saved-views":                                       viaProtected,
	"DELETE /saved-views/:id":                                 viaProtected,
	"PATCH /saved-views/:id":                                  viaProtected,
	"DELETE /scanner/agents/:id":                              viaProtected,
	"POST /scanner/configs":                                   viaProtected,
	"DELETE /scanner/configs/:id":                             viaProtected,
	"POST /scanner/configs/:id/registration-token":            viaProtected,
	"POST /scanner/configs/:id/scan":                          viaProtected,
	"POST /scanner/jobs/:id/ignore":                           viaProtected,
	"POST /scanner/jobs/:id/import":                           viaProtected,
	"POST /scanner/mitigations/auto-complete":                 viaProtected,
	"POST /score-engine/classify":                             viaProtected,
	"POST /score-engine/compute":                              viaProtected,
	"POST /score-engine/configs":                              viaProtected,
	"PUT /score-engine/configs/:id":                           viaProtected,
	"POST /score/preview":                                     viaProtected,
	"PUT /security/mfa-policy":                                viaProtected,
	"POST /teams":                                             viaProtected,
	"DELETE /teams/:id":                                       viaProtected,
	"PATCH /teams/:id":                                        viaProtected,
	"DELETE /teams/:id/members/:userId":                       viaProtected,
	"POST /teams/:id/members/:userId":                         viaProtected,
	"PUT /telemetry":                                          viaProtected,
	"POST /users":                                             viaProtected,
	"PATCH /users/me":                                         viaProtected,
	"DELETE /users/me/avatar":                                 viaProtected,
	"PUT /users/me/avatar":                                    viaProtected,
	"POST /vendor-assessments/:id/resend":                     viaProtected,
	"POST /vendor-assessments/:id/revoke":                     viaProtected,
	"POST /vendor-questionnaire-templates":                    viaProtected,
	"PUT /vendor-questionnaire-templates/:id":                 viaProtected,
	"POST /vendor-questionnaire-templates/:id/archive":        viaProtected,
	"POST /vendors/:id/assessments":                           viaProtected,
	"POST /vendors/:id/assets":                                viaProtected,
	"DELETE /vendors/:id/assets/:linkId":                      viaProtected,
	"POST /vulnerabilities/bulk/change-status":                viaProtected,
	"POST /vulnerabilities/bulk/delete":                       viaProtected,
	"POST /vulnerabilities/bulk/preview":                      viaProtected,
	"DELETE /vulnerabilities/:id":                             viaProtected,
	"PUT /vulnerabilities/:id/asset":                          viaProtected,
	"PATCH /vulnerabilities/:id/status":                       viaProtected,
	"POST /vulnerabilities/:id/ticket":                        viaProtected,
	"POST /vulnerabilities/ingest":                            viaProtected,
	"POST /vulnerabilities/integrations":                      viaProtected,
	"DELETE /vulnerabilities/integrations/:id":                viaProtected,
	"POST /vulnerabilities/integrations/:id/pull":             viaProtected,
	"POST /vulnerabilities/scan":                              viaProtected,
	"DELETE /vulnerabilities/ticketing":                       viaProtected,
	"PUT /vulnerabilities/ticketing":                          viaProtected,
}

// mutatingRegistration is one state-changing route as main.go declares it.
type mutatingRegistration struct {
	key       string // "METHOD /path", relative to /api/v1
	afterGate bool
	body      string
}

// parseMutatingRoutes reads every state-changing registration in main.go and
// resolves it to its full path under /api/v1.
func parseMutatingRoutes(t *testing.T, src string) []mutatingRegistration {
	t.Helper()
	gate := protectedGateOffset(t, src)

	prefixes := map[string]string{"api": "", "protected": ""}
	for _, m := range regexp.MustCompile(`(\w+)\s*:?=\s*(\w+)\.Group\(\s*"([^"]*)"`).FindAllStringSubmatch(src, -1) {
		if m[1] == "api" {
			continue // the /api/v1 group itself: paths are kept relative to it
		}
		parent, ok := prefixes[m[2]]
		require.True(t, ok, "group %s derives from %s, which the parser does not know", m[1], m[2])
		_, reused := prefixes[m[1]]
		require.False(t, reused, "group variable %s is declared twice; the parser keys prefixes by name", m[1])
		prefixes[m[1]] = parent + m[3]
	}

	routers := []string{"app"}
	for name := range prefixes {
		routers = append(routers, regexp.QuoteMeta(name))
	}
	unreadable := regexp.MustCompile(`\b(?:`+strings.Join(routers, "|")+`)\.(?:Add|Route|Mount)\(|\bRegister\w*Routes\(`).FindAllString(src, -1)
	require.Empty(t, unreadable, "main.go registers routes in a form this parser cannot read; extend it before the table can vouch for them")

	var out []mutatingRegistration
	for _, r := range parseRegistrations(t, src) {
		if r.method == "GET" {
			continue
		}
		recv := r.body[:strings.Index(r.body, ".")]
		var path string
		switch {
		case recv == "app":
			require.True(t, strings.HasPrefix(r.path, "/api/v1/"), "app.%s(%q) is outside /api/v1; extend the parser", r.method, r.path)
			path = strings.TrimPrefix(r.path, "/api/v1")
		default:
			prefix, ok := prefixes[recv]
			require.True(t, ok, "%s.%s(%q): unknown receiver, so its prefix and its gate are unknown; extend the parser", recv, r.method, r.path)
			path = prefix + r.path
		}
		out = append(out, mutatingRegistration{key: r.method + " " + path, afterGate: r.offset > gate, body: r.body})
	}
	return out
}

func TestCSRFRouteTable_ListsEveryMutatingRoute(t *testing.T) {
	routes := parseMutatingRoutes(t, readMainGo(t))
	require.Greater(t, len(routes), 200, "far fewer mutating routes than expected; the parser stopped matching")

	seen := map[string]bool{}
	var missing []string
	for _, r := range routes {
		assert.False(t, seen[r.key], "%s is registered twice", r.key)
		seen[r.key] = true
		defence, ok := mutatingRouteDefences[r.key]
		if !ok {
			missing = append(missing, r.key)
			continue
		}
		// The position decides what runs ahead of the handler, so the table
		// cannot claim the gate for a route registered before it, or the reverse.
		if defence == viaProtected {
			assert.True(t, r.afterGate, "%s is listed as behind Protected but is registered before api.Use(Protected)", r.key)
		} else {
			assert.False(t, r.afterGate, "%s is registered behind Protected; list it as viaProtected", r.key)
		}
		switch defence {
		case viaMFAEnrollment:
			assert.Contains(t, r.body, "mfaEnrollmentGuard", r.key)
		case viaOptionalAuth:
			assert.Contains(t, r.body, "optionalAuth", r.key)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing,
		"state-changing routes with no CSRF decision. Add each to mutatingRouteDefences with the defence it relies on")

	var stale []string
	for key := range mutatingRouteDefences {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	assert.Empty(t, stale, "listed routes main.go no longer registers; remove them")
}

// The table's claims, exercised: each route that relies on a middleware gets a
// cookie session from another origin with no token, a forged token, and the
// real one, through that middleware exactly as main.go mounts it.
func TestCSRFRouteTable_CookieRoutesRefuseCrossOrigin(t *testing.T) {
	routes := parseMutatingRoutes(t, readMainGo(t))

	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: pk, PublicKey: &pk.PublicKey}
	tenant := uuid.New()
	access, _, err := authpkg.GenerateTypedToken(keys, uuid.New(), tenant,
		map[uuid.UUID]string{tenant: "root"}, []string{"*"}, nil, 5*time.Minute, authpkg.TokenTypeAccess)
	require.NoError(t, err)
	noBlacklist := func(string) (bool, error) { return false, nil }

	reached := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"reached": true}) }
	app := fiber.New()
	api := app.Group("/api/v1")
	optional := middleware.OptionalAuth(keys, noBlacklist)
	enrolment := middleware.MFAEnrollmentMiddleware(keys, noBlacklist)
	var gated []mutatingRegistration
	for _, r := range routes {
		method, path, _ := strings.Cut(r.key, " ")
		switch mutatingRouteDefences[r.key] {
		case viaOptionalAuth:
			api.Add(method, path, optional, reached)
		case viaMFAEnrollment:
			api.Add(method, path, enrolment, reached)
		case viaProtected:
			gated = append(gated, r)
		default:
			continue
		}
	}
	api.Use(middleware.Protected(keys, noBlacklist))
	for _, r := range gated {
		method, path, _ := strings.Cut(r.key, " ")
		api.Add(method, path, reached)
	}

	concrete := regexp.MustCompile(`:\w+\??|\*`)
	send := func(key, csrfCookie, csrfHeader string) (int, map[string]any) {
		method, path, _ := strings.Cut(key, " ")
		req := httptest.NewRequest(method, "/api/v1"+concrete.ReplaceAllString(path, uuid.NewString()), nil)
		req.Header.Set("Origin", "https://attacker.example")
		req.AddCookie(&http.Cookie{Name: middleware.AccessTokenCookie, Value: access})
		if csrfCookie != "" {
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookie, Value: csrfCookie})
		}
		if csrfHeader != "" {
			req.Header.Set(middleware.CSRFHeader, csrfHeader)
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		out := map[string]any{}
		_ = json.Unmarshal(raw, &out)
		return resp.StatusCode, out
	}

	checked := 0
	for _, r := range routes {
		switch mutatingRouteDefences[r.key] {
		case viaProtected, viaOptionalAuth, viaMFAEnrollment:
		default:
			continue
		}
		checked++
		for name, tok := range map[string][2]string{
			"no token":      {"", ""},
			"cookie only":   {"csrf-victim", ""},
			"forged header": {"csrf-victim", "csrf-attacker"},
		} {
			status, body := send(r.key, tok[0], tok[1])
			assert.Equal(t, http.StatusForbidden, status, "%s with %s", r.key, name)
			assert.Equal(t, "CSRF_FAILED", body["code"], "%s with %s", r.key, name)
			assert.Nil(t, body["reached"], "%s ran its handler with %s", r.key, name)
		}
		status, body := send(r.key, "csrf-victim", "csrf-victim")
		assert.Equal(t, http.StatusOK, status, "%s refused the SPA's own request: %v", r.key, body)
	}
	require.Greater(t, checked, 200)
	t.Logf("%d cookie-authenticated mutating routes refuse a cross-origin request without the double-submit token", checked)
}
