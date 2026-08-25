// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// OR26-03 — the deferrable-MFA journey through the REAL HTTP stack.
//
// Real login use case, real login handler, real RS256 auth middleware, real MFA
// policy guard, real policy repository, real GORM models. Nothing about the
// requirement is stubbed: the only thing substituted for production here is the
// database engine.
//
// It exists because the claims that matter are all about what happens BETWEEN
// components — a session minted while the window was open being refused by the
// guard afterwards, a policy written by one tenant not moving another's, a
// client that says it is compliant being ignored. Unit tests cannot hold those.
// ---------------------------------------------------------------------------

type deferredFixture struct {
	app        *fiber.App
	db         *gorm.DB
	keys       *authpkg.RSAKeys
	policyRepo *repository.GormMFAPolicyRepository
	resolver   *appauth.MFAStatusResolver

	now time.Time

	tenantA, tenantB uuid.UUID
	adminA, adminB   *domain.OrganizationMember
	memberA          *domain.OrganizationMember
}

type deferredHasher struct{}

func (deferredHasher) Hash(p string) (string, error)  { return "hashed:" + p, nil }
func (deferredHasher) Verify(hash, plain string) bool { return hash == "hashed:"+plain }
func (deferredHasher) NeedsRehash(string) bool        { return false }

const deferredPassword = "Ancre-Vitrail7-Cobalt"

func newDeferredFixture(t *testing.T) *deferredFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	// users / organization_members / organizations carry Postgres-only column
	// defaults, so they are created minimally and reconciled against the structs.
	// Reconcile keeps the fixture in step with the models rather than with a
	// comment — the hand-written variant of this DDL has drifted twice already.
	for _, ddl := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY)`,
		`CREATE TABLE mfa_secrets (id TEXT PRIMARY KEY)`,
		`CREATE TABLE mfa_backup_codes (id TEXT PRIMARY KEY)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	for _, m := range []struct {
		table string
		model any
	}{
		{"users", &domain.User{}},
		{"organization_members", &domain.OrganizationMember{}},
		{"organizations", &domain.Organization{}},
		{"mfa_secrets", &domain.MFASecret{}},
		{"mfa_backup_codes", &domain.MFABackupCode{}},
	} {
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}
	// MFAPolicy is the one model here AutoMigrate can build on sqlite unaided:
	// it carries no Postgres-only column default, which was a deliberate choice
	// when it was written (see domain/mfa_policy.go).
	require.NoError(t, db.AutoMigrate(&domain.MFAPolicy{}))
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	f := &deferredFixture{
		db: db, keys: keys,
		now:     time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC),
		tenantA: uuid.New(), tenantB: uuid.New(),
	}
	f.policyRepo = repository.NewGormMFAPolicyRepository(db)

	for _, org := range []struct {
		id   uuid.UUID
		name string
		slug string
	}{{f.tenantA, "Banque Atlantique", "banque-atlantique"}, {f.tenantB, "Other Corp", "other-corp"}} {
		require.NoError(t, db.Create(&domain.Organization{
			ID: org.id, Name: org.name, Slug: org.slug, IsActive: true, Plan: domain.PlanStarter,
		}).Error)
	}

	// Anchored today: a fresh privileged account starts with its whole window
	// ahead of it, which is the state the reported bug was about.
	f.adminA = f.seedMember(t, f.tenantA, "admin@a.io", domain.RoleAdmin, "", f.now)
	f.adminB = f.seedMember(t, f.tenantB, "admin@b.io", domain.RoleAdmin, "", f.now)
	f.memberA = f.seedMember(t, f.tenantA, "member@a.io", domain.RoleUser, "", f.now.Add(-3650*24*time.Hour))

	userRepo := repository.NewGormUserRepository(db)
	tokens := coreauth.NewTokenManager(db, keys)
	mfaRepo := repository.NewGormMFARepository(db)
	orgRoles, businessRoles := domain.DefaultMFAPrivilegeRoles()

	loginUC := appauth.NewLoginUseCase(userRepo, tokens, deferredHasher{}).
		WithMFA(mfaRepo).
		RequireMFAForRoles(orgRoles, businessRoles).
		WithMFAPolicies(f.policyRepo).
		WithClock(func() time.Time { return f.now })

	f.resolver = appauth.NewMFAStatusResolver(mfaRepo, userRepo, orgRoles, businessRoles).
		WithPolicies(f.policyRepo).
		WithClock(func() time.Time { return f.now })

	h := authhandler.NewHandler(loginUC, nil, nil, nil, deferredHasher{}, nil).
		WithUserLookup(userRepo).
		WithMFAStatus(f.resolver)

	policyHandler := authhandler.NewMFAPolicyHandler(
		appauth.NewGetMFAPolicyUseCase(f.policyRepo, orgRoles, businessRoles),
		appauth.NewUpdateMFAPolicyUseCase(f.policyRepo, orgRoles, businessRoles).WithCacheInvalidator(f.resolver),
	)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/login", h.Login)

	// The same mount order main.go uses: the auth gate publishes the identity,
	// then the MFA guard reads it, then everything else.
	protected := api.Use(middleware.Protected(keys, nil))
	protected.Use(middleware.MFAPolicyGuard(f.resolver))

	protected.Get("/auth/me", h.Me)
	protected.Get("/security/mfa-policy", policyHandler.Get)
	protected.Put("/security/mfa-policy", middleware.RequireRole("admin", "root"), policyHandler.Update)
	// Stand-ins for every business route: they exist only to be reached, or not.
	ok := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }
	protected.Get("/risks", ok)
	protected.Post("/risks", ok)
	protected.Post("/auth/pat", ok)
	protected.Post("/auth/mfa/setup", ok)

	f.app = app
	return f
}

func (f *deferredFixture) seedMember(
	t *testing.T, tenant uuid.UUID, email string,
	role domain.MemberRole, business domain.BusinessRoleKey, anchor time.Time,
) *domain.OrganizationMember {
	t.Helper()
	org := tenant
	user := &domain.User{
		ID: uuid.New(), Email: email, Username: strings.Split(email, "@")[0],
		FullName: "Test " + email, IsActive: true,
		Password: "hashed:" + deferredPassword, DefaultOrgID: &org,
	}
	require.NoError(t, f.db.Create(user).Error)

	m := &domain.OrganizationMember{
		ID: uuid.New(), UserID: user.ID, OrganizationID: tenant, Role: role,
		BusinessRole: business, IsActive: true, Status: domain.MembershipActive,
		JoinedAt: anchor, MFAGraceStartedAt: &anchor,
	}
	require.NoError(t, f.db.Create(m).Error)
	m.User = user
	return m
}

// backdate moves a member's grace anchor into the past, as elapsed time would,
// and drops the cached decision the way a real state change does.
func (f *deferredFixture) backdate(t *testing.T, m *domain.OrganizationMember, days int) {
	t.Helper()
	at := f.now.Add(-time.Duration(days) * 24 * time.Hour)
	require.NoError(t, f.db.Model(&domain.OrganizationMember{}).
		Where("id = ?", m.ID).Update("mfa_grace_started_at", at).Error)
	f.resolver.Invalidate(m.UserID, m.OrganizationID)
}

type jsonBody map[string]any

func (f *deferredFixture) do(t *testing.T, method, path, token string, body any) (int, jsonBody) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = strings.NewReader(string(raw))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := jsonBody{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

// login returns the raw login response — the exact payload the reported bug was
// about — plus the access token when one was issued.
func (f *deferredFixture) login(t *testing.T, email string) (jsonBody, string) {
	t.Helper()
	status, body := f.do(t, http.MethodPost, "/api/v1/auth/login", "", jsonBody{
		"email": email, "password": deferredPassword,
	})
	require.Equal(t, http.StatusOK, status, "login should answer 200: %v", body)

	token := ""
	if pair, ok := body["token_pair"].(map[string]any); ok {
		token, _ = pair["access_token"].(string)
	}
	return body, token
}

func mfaBlock(t *testing.T, body jsonBody) map[string]any {
	t.Helper()
	block, ok := body["mfa"].(map[string]any)
	require.True(t, ok, "the response must carry the resolved mfa block: %v", body)
	return block
}

func statusOf(status int, _ jsonBody) int { return status }

// ---------------------------------------------------------------------------
// 1. The bug: a first login must not be a wall
// ---------------------------------------------------------------------------

func TestDeferredMFA_FreshAdminSignsInAndReachesTheProduct(t *testing.T) {
	f := newDeferredFixture(t)

	body, token := f.login(t, "admin@a.io")

	// THE REGRESSION FENCE. Before OR26-03 this was true with no token pair, and
	// the evaluator met a QR code before a single screen.
	assert.Nil(t, body["mfa_enrollment_required"], "a fresh admin must not be walled at enrolment")
	require.NotEmpty(t, token, "the first login must produce a real session")

	block := mfaBlock(t, body)
	assert.Equal(t, string(domain.MFAStateGraceActive), block["state"])
	assert.Equal(t, false, block["required"])
	assert.Equal(t, true, block["privileged"])
	assert.Equal(t, float64(7), block["grace_days"])
	assert.NotEmpty(t, block["deadline"], "a privileged account must be told when the window closes")

	// And the product is genuinely reachable, not merely "logged in".
	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))
	assert.Equal(t, http.StatusOK,
		statusOf(f.do(t, http.MethodPost, "/api/v1/risks", token, jsonBody{"name": "First risk"})),
		"creating the first risk is the value the wall stood in front of")
}

func TestDeferredMFA_OrdinaryMemberIsOnlyEverInvited(t *testing.T) {
	f := newDeferredFixture(t)

	body, token := f.login(t, "member@a.io")
	block := mfaBlock(t, body)

	// Ten years old, no authenticator, and still nothing but a recommendation.
	assert.Equal(t, string(domain.MFAStateRecommended), block["state"])
	assert.Equal(t, false, block["required"])
	assert.Equal(t, false, block["privileged"])
	assert.Nil(t, block["deadline"], "a member the mandate does not apply to has no deadline to show")

	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))
}

func TestDeferredMFA_SessionContractCarriesNoSecret(t *testing.T) {
	f := newDeferredFixture(t)
	body, token := f.login(t, "admin@a.io")

	raw, err := json.Marshal(mfaBlock(t, body))
	require.NoError(t, err)
	assert.NotRegexp(t, `(?i)secret|qr_code|backup|totp`, string(raw))

	_, me := f.do(t, http.MethodGet, "/api/v1/auth/me", token, nil)
	rawMe, err := json.Marshal(me)
	require.NoError(t, err)
	assert.NotRegexp(t, `(?i)secret_encrypted|qr_code|backup_code`, string(rawMe))
}

func TestDeferredMFA_MeReportsTheSameStateAsLogin(t *testing.T) {
	// One decision, two readers. If these could differ, the banner and the guard
	// would disagree and the user would believe the wrong one.
	f := newDeferredFixture(t)
	body, token := f.login(t, "admin@a.io")

	status, me := f.do(t, http.MethodGet, "/api/v1/auth/me", token, nil)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, mfaBlock(t, body)["state"], mfaBlock(t, me)["state"])
}

// ---------------------------------------------------------------------------
// 2. Enforcement: the window has to actually close
// ---------------------------------------------------------------------------

func TestDeferredMFA_PastTheDeadlineLoginStopsAtEnrolment(t *testing.T) {
	f := newDeferredFixture(t)
	f.backdate(t, f.adminA, 8)

	body, token := f.login(t, "admin@a.io")

	assert.Equal(t, true, body["mfa_enrollment_required"])
	assert.Empty(t, token, "no session may be issued once the window has closed")
	assert.NotEmpty(t, body["mfa_token"], "the enrolment token is the way forward")
	assert.Equal(t, string(domain.MFAStateRequired), mfaBlock(t, body)["state"])
}

func TestDeferredMFA_ALiveSessionIsRefusedOnceTheWindowCloses(t *testing.T) {
	// THE POINT OF THE REQUEST-TIME GUARD. Enforcing only at login would let an
	// admin who signed in on day one keep working forever by never signing out.
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")

	require.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)),
		"inside the window the session works")

	// Time passes, on the SAME session.
	f.backdate(t, f.adminA, 8)

	status, body := f.do(t, http.MethodGet, "/api/v1/risks", token, nil)
	assert.Equal(t, http.StatusForbidden, status,
		"the session must stop working, not merely stop being renewable")
	assert.Equal(t, middleware.MFAEnrollmentRequiredCode, body["code"])
	assert.Equal(t, true, mfaBlock(t, body)["required"])
}

func TestDeferredMFA_TheRemedyStaysReachableWhileBlocked(t *testing.T) {
	// A requirement you cannot satisfy is a lockout, not a control.
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	f.backdate(t, f.adminA, 8)

	require.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))

	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/auth/me", token, nil)),
		"the user must be able to learn WHY they were refused")
	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodPost, "/api/v1/auth/mfa/setup", token, jsonBody{})),
		"enrolment must remain reachable")
}

func TestDeferredMFA_EnrolmentRestoresAccess(t *testing.T) {
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	f.backdate(t, f.adminA, 8)
	require.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))

	// A verified authenticator now exists, and the cached decision is dropped
	// the way the verify handler drops it.
	require.NoError(t, f.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: f.adminA.UserID, TenantID: f.tenantA,
		SecretEncrypted: "encrypted", IsVerified: true,
	}).Error)
	f.resolver.Invalidate(f.adminA.UserID, f.tenantA)

	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)),
		"enrolling must restore access on the very next request")
}

func TestDeferredMFA_RSSIIsCoveredByTheMandate(t *testing.T) {
	// The issue names "Admin/RSSI". The security officer holds org role `user`,
	// so an org-role-only check would exempt exactly the account it is written
	// for — and the JWT says `user`, which is why the guard must not let the
	// token's role NARROW the privileged set.
	f := newDeferredFixture(t)
	rssi := f.seedMember(t, f.tenantA, "rssi@a.io", domain.RoleUser, domain.BusinessRoleRSSI,
		f.now.Add(-9*24*time.Hour))

	body, token := f.login(t, "rssi@a.io")
	assert.Equal(t, true, body["mfa_enrollment_required"], "the RSSI is privileged through the preset")
	assert.Empty(t, token)

	// And inside the window, they work like anyone else.
	f.backdate(t, rssi, 1)
	body, token = f.login(t, "rssi@a.io")
	require.NotEmpty(t, token)
	assert.Equal(t, string(domain.MFAStateGraceActive), mfaBlock(t, body)["state"])
	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))
}

// ---------------------------------------------------------------------------
// 3. The policy
// ---------------------------------------------------------------------------

func TestDeferredMFA_PolicyIsReadableBoundedAndAdminWritable(t *testing.T) {
	f := newDeferredFixture(t)
	_, adminToken := f.login(t, "admin@a.io")
	_, memberToken := f.login(t, "member@a.io")

	status, body := f.do(t, http.MethodGet, "/api/v1/security/mfa-policy", adminToken, nil)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, float64(7), body["grace_days"])
	assert.Equal(t, false, body["configured"], "an untouched tenant is on the default, not a chosen value")
	assert.Equal(t, float64(0), body["min_days"])
	assert.Equal(t, float64(90), body["max_days"])

	// Everyone subject to a deadline may read it.
	assert.Equal(t, http.StatusOK,
		statusOf(f.do(t, http.MethodGet, "/api/v1/security/mfa-policy", memberToken, nil)))

	// Bounds are the server's, not only the form's.
	for _, bad := range []any{-1, 91, 100000} {
		assert.Equal(t, http.StatusBadRequest,
			statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", adminToken, jsonBody{"grace_days": bad})),
			"grace_days=%v must be refused", bad)
	}
	assert.Equal(t, http.StatusBadRequest,
		statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", adminToken, jsonBody{})),
		"a partial save must not silently mean zero")

	status, saved := f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", adminToken, jsonBody{"grace_days": 3})
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, float64(3), saved["grace_days"])
	assert.Equal(t, true, saved["configured"])

	// A non-admin cannot move it.
	assert.Equal(t, http.StatusForbidden,
		statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", memberToken, jsonBody{"grace_days": 90})))
}

func TestDeferredMFA_PolicyIsTenantScoped(t *testing.T) {
	// Tenant A at 1 day, tenant B untouched. Neither reads nor moves the other,
	// and B's administrator keeps the full default window.
	f := newDeferredFixture(t)
	_, tokenA := f.login(t, "admin@a.io")
	_, tokenB := f.login(t, "admin@b.io")

	require.Equal(t, http.StatusOK,
		statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", tokenA, jsonBody{"grace_days": 1})))

	_, policyB := f.do(t, http.MethodGet, "/api/v1/security/mfa-policy", tokenB, nil)
	assert.Equal(t, float64(7), policyB["grace_days"], "tenant B must not inherit tenant A's window")

	_, policyA := f.do(t, http.MethodGet, "/api/v1/security/mfa-policy", tokenA, nil)
	assert.Equal(t, float64(1), policyA["grace_days"])

	// Two days into A's one-day window, A's admin is refused and B's is not.
	f.backdate(t, f.adminA, 2)
	f.backdate(t, f.adminB, 2)

	assert.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", tokenA, nil)))
	assert.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", tokenB, nil)),
		"one tenant's policy must never decide another tenant's access")
}

func TestDeferredMFA_ShorteningTheWindowBitesImmediately(t *testing.T) {
	// An administrator who sets 0 days and watches their colleagues keep working
	// concludes the setting is decorative.
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	require.Equal(t, http.StatusOK, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))

	require.Equal(t, http.StatusOK,
		statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", token, jsonBody{"grace_days": 0})))

	assert.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)),
		"the new window applies on the next request, not at the end of a cache TTL")
}

// ---------------------------------------------------------------------------
// 4. Bypass attempts — all at the API, no browser involved
// ---------------------------------------------------------------------------

func TestDeferredMFA_ClientCannotTalkItsWayOut(t *testing.T) {
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	f.backdate(t, f.adminA, 8)
	require.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))

	// A body that claims compliance. The server resolves the state itself;
	// nothing it reads comes from the request.
	assert.Equal(t, http.StatusForbidden,
		statusOf(f.do(t, http.MethodPost, "/api/v1/risks", token, jsonBody{
			"name": "bypass", "mfa_configured": true,
			"mfa": jsonBody{"state": "configured", "required": false},
		})))

	// Widening the window back out is itself a protected write, so a blocked
	// account cannot restore its own access.
	assert.Equal(t, http.StatusForbidden,
		statusOf(f.do(t, http.MethodPut, "/api/v1/security/mfa-policy", token, jsonBody{"grace_days": 90})))

	// Minting a personal access token — the standing-exemption route — is
	// refused too. A PAT issued now would outlive the block.
	assert.Equal(t, http.StatusForbidden,
		statusOf(f.do(t, http.MethodPost, "/api/v1/auth/pat", token, jsonBody{"name": "bypass"})))

	// And the token itself carries no MFA claim to tamper with in the first place.
	claims, err := authpkg.ValidateAccessToken(f.keys, token, nil)
	require.NoError(t, err)
	raw, err := json.Marshal(claims)
	require.NoError(t, err)
	assert.NotRegexp(t, `(?i)mfa`, string(raw))
}

func TestDeferredMFA_AnUnverifiedSecretDoesNotCountAsEnrolment(t *testing.T) {
	// A half-finished enrolment — secret generated, code never confirmed — is not
	// protection, and must not read as compliance.
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	require.NoError(t, f.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: f.adminA.UserID, TenantID: f.tenantA,
		SecretEncrypted: "encrypted", IsVerified: false,
	}).Error)
	f.backdate(t, f.adminA, 8)

	assert.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)))
}

func TestDeferredMFA_AMissingAnchorFailsClosedForAPrivilegedAccount(t *testing.T) {
	// A NULL anchor on a privileged membership is the state somebody would try
	// to produce to buy unlimited grace.
	f := newDeferredFixture(t)
	_, token := f.login(t, "admin@a.io")
	require.NoError(t, f.db.Model(&domain.OrganizationMember{}).
		Where("id = ?", f.adminA.ID).
		Updates(map[string]any{"mfa_grace_started_at": nil, "joined_at": nil, "created_at": nil}).Error)
	f.resolver.Invalidate(f.adminA.UserID, f.tenantA)

	assert.Equal(t, http.StatusForbidden, statusOf(f.do(t, http.MethodGet, "/api/v1/risks", token, nil)),
		"an unresolvable countdown must not read as infinite grace")
}
