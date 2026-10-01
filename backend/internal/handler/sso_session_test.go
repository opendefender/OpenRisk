// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// These cover the exit of every SSO sign-in (#803): the browser must leave with
// the session in HttpOnly cookies and a redirect to the SPA, never with a JSON
// body holding the tokens.

const ssoTestBase = "https://app.test"

// recordingAuditRepo keeps what the audit service writes.
type recordingAuditRepo struct {
	mu   sync.Mutex
	logs []*domain.AuthAuditLog
}

func (r *recordingAuditRepo) Create(_ context.Context, l *domain.AuthAuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, l)
	return nil
}

func (r *recordingAuditRepo) GetByUser(context.Context, uuid.UUID, int, int) ([]*domain.AuthAuditLog, error) {
	return nil, nil
}

func (r *recordingAuditRepo) GetByTenant(context.Context, uuid.UUID, int, int) ([]*domain.AuthAuditLog, error) {
	return nil, nil
}

func (r *recordingAuditRepo) count(action coreauth.AuditAction) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, l := range r.logs {
		if l.Action == string(action) && l.Success {
			n++
		}
	}
	return n
}

// ssoFixture is a user with a tenant, and the SSO globals pointed at a real
// TokenManager over sqlite.
type ssoFixture struct {
	db    *gorm.DB
	user  *domain.User
	orgID uuid.UUID
	audit *recordingAuditRepo
}

// setupSSOSession wires ssoTokenManager, ssoAudit, database.DB and the SPA base,
// and restores all of them afterwards. resolverErr, when set, makes session
// issuance fail the way a vanished membership does.
func setupSSOSession(t *testing.T, resolverErr error) *ssoFixture {
	t.Helper()

	prevTM, prevAudit, prevRepo := ssoTokenManager, ssoAudit, ssoUserRepo
	prevDB, prevBase := database.DB, oauthAppBaseURL
	t.Cleanup(func() {
		ssoTokenManager, ssoAudit, ssoUserRepo = prevTM, prevAudit, prevRepo
		database.DB, oauthAppBaseURL = prevDB, prevBase
	})

	db := setupSwitchDB(t)

	userID, orgID := uuid.New(), uuid.New()
	seedMember(t, db, userID, orgID, "Acme", "acme", true)
	require.NoError(t, db.Exec(`INSERT INTO users (id, email, username, full_name, is_active, default_org_id) VALUES (?,?,?,?,1,?)`,
		userID.String(), "member@opendefender.io", "member", "Member", orgID.String()).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tm := coreauth.NewTokenManager(db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey})
	tm.SetSessionResolver(func(_ context.Context, uid uuid.UUID) (*coreauth.SessionClaims, error) {
		if resolverErr != nil {
			return nil, resolverErr
		}
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: "admin"}}, nil
	})

	audit := &recordingAuditRepo{}
	ssoTokenManager = tm
	ssoAudit = coreauth.NewAuditService(audit)
	ssoUserRepo = nil
	database.DB = db
	oauthAppBaseURL = ssoTestBase

	return &ssoFixture{
		db:    db,
		user:  &domain.User{ID: userID, Email: "member@opendefender.io", Username: "member", IsActive: true, DefaultOrgID: &orgID},
		orgID: orgID,
		audit: audit,
	}
}

// runIssueSSOSession calls issueSSOSession the way both callbacks do.
func runIssueSSOSession(t *testing.T, user *domain.User, returnTo string) *http.Response {
	t.Helper()
	app := fiber.New()
	app.Get("/cb", func(c *fiber.Ctx) error {
		return issueSSOSession(c, user, "google", returnTo, "en")
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/cb", nil), -1)
	require.NoError(t, err)
	return resp
}

func cookiesByName(resp *http.Response) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, ck := range resp.Cookies() {
		out[ck.Name] = ck
	}
	return out
}

// requireSessionCookiesWithoutLeak is the core of AC1/AC2: three cookies, a
// redirect, and neither token in the body or the Location.
func requireSessionCookiesWithoutLeak(t *testing.T, resp *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusFound, resp.StatusCode)

	cookies := cookiesByName(resp)
	access, refresh, csrf := cookies[middleware.AccessTokenCookie], cookies[middleware.RefreshTokenCookie], cookies[middleware.CSRFCookie]
	require.NotNil(t, access, "access cookie missing")
	require.NotNil(t, refresh, "refresh cookie missing")
	require.NotNil(t, csrf, "csrf cookie missing")
	require.True(t, access.HttpOnly)
	require.True(t, refresh.HttpOnly)
	require.Equal(t, "/api/v1/auth/refresh", refresh.Path)
	require.NotEmpty(t, access.Value)
	require.NotEmpty(t, refresh.Value)

	// Same lifetimes as password login (IssueSessionCookies), within a minute.
	require.WithinDuration(t, time.Now().Add(coreauth.AccessTokenTTL), access.Expires, time.Minute)
	require.WithinDuration(t, time.Now().Add(coreauth.RefreshTokenTTL), refresh.Expires, time.Minute)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	loc := resp.Header.Get("Location")
	for _, leak := range []string{"access_token", "refresh_token", access.Value, refresh.Value} {
		require.NotContains(t, string(body), leak)
		require.NotContains(t, loc, leak)
	}
}

func requireInternalFailure(t *testing.T, resp *http.Response, provider string) {
	t.Helper()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, ssoTestBase+"/login", loc.Scheme+"://"+loc.Host+loc.Path)
	require.Equal(t, "internal", loc.Query().Get("error"))
	require.Equal(t, provider, loc.Query().Get("provider"))

	cookies := cookiesByName(resp)
	require.Nil(t, cookies[middleware.AccessTokenCookie], "a failed sign-in must not set a session")
	require.Nil(t, cookies[middleware.RefreshTokenCookie])
}

func TestIssueSSOSession_Success(t *testing.T) {
	fx := setupSSOSession(t, nil)

	resp := runIssueSSOSession(t, fx.user, "")

	requireSessionCookiesWithoutLeak(t, resp)
	require.Equal(t, ssoTestBase+"/auth/sso/complete", resp.Header.Get("Location"))
	require.Equal(t, 1, fx.audit.count(coreauth.AuditActionLogin), "the login must still be audited")
}

func TestIssueSSOSession_ReturnToIsHonoured(t *testing.T) {
	fx := setupSSOSession(t, nil)

	resp := runIssueSSOSession(t, fx.user, "/risks?focus=42&tab=score")

	requireSessionCookiesWithoutLeak(t, resp)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/auth/sso/complete", loc.Path)
	require.Equal(t, "/risks?focus=42&tab=score", loc.Query().Get("next"))
}

func TestIssueSSOSession_OffSiteReturnToIsIgnored(t *testing.T) {
	fx := setupSSOSession(t, nil)

	for _, target := range []string{"//evil.example", "https://evil.example/x", `/\evil.example`, "evil.example"} {
		t.Run(target, func(t *testing.T) {
			resp := runIssueSSOSession(t, fx.user, target)
			require.Equal(t, http.StatusFound, resp.StatusCode)
			require.Equal(t, ssoTestBase+"/auth/sso/complete", resp.Header.Get("Location"))
		})
	}
}

// Unwired token manager: the deployment cannot mint sessions at all.
func TestIssueSSOSession_Unauthorized(t *testing.T) {
	fx := setupSSOSession(t, nil)
	ssoTokenManager = nil

	requireInternalFailure(t, runIssueSSOSession(t, fx.user, "/risks"), "google")
	require.Zero(t, fx.audit.count(coreauth.AuditActionLogin))
}

// The session resolver cannot find the user's membership.
func TestIssueSSOSession_NotFound(t *testing.T) {
	fx := setupSSOSession(t, domain.NewNotFoundError("membership", "member@opendefender.io"))

	requireInternalFailure(t, runIssueSSOSession(t, fx.user, ""), "google")
	require.Zero(t, fx.audit.count(coreauth.AuditActionLogin))
}

func TestIssueSSOSession_OnboardingFailureRedirects(t *testing.T) {
	setupSSOSession(t, nil)

	// A user with no tenant, against a database where onboarding cannot write.
	broken, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	database.DB = broken
	orphan := &domain.User{ID: uuid.New(), Email: "new@opendefender.io", IsActive: true}

	resp := runIssueSSOSession(t, orphan, "")

	requireInternalFailure(t, resp, "google")
	body, _ := io.ReadAll(resp.Body)
	require.NotContains(t, string(body), "no such table", "the onboarding error must not reach the browser")
}

// ---------------------------------------------------------------------------
// Through the real callbacks
// ---------------------------------------------------------------------------

// linkedUserRepo resolves every identity to the fixture user through an
// existing provider link, the shortest path through the resolver.
type linkedUserRepo struct{ user *domain.User }

func (r linkedUserRepo) GetByEmail(context.Context, string) (*domain.User, error) { return r.user, nil }
func (r linkedUserRepo) GetByID(context.Context, uuid.UUID) (*domain.User, error) { return r.user, nil }

type linkedRepo struct{ userID uuid.UUID }

func (l linkedRepo) FindByProviderSubject(context.Context, string, string) (*domain.OAuthProvider, error) {
	return &domain.OAuthProvider{ID: uuid.New(), UserID: l.userID}, nil
}
func (l linkedRepo) ListByUser(context.Context, uuid.UUID) ([]domain.OAuthProvider, error) {
	return nil, nil
}
func (l linkedRepo) Create(context.Context, *domain.OAuthProvider) error    { return nil }
func (l linkedRepo) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }

// oauthSignIn drives /login?return_to=… then /callback for a linked user.
func oauthSignIn(t *testing.T, fx *ssoFixture, returnTo string) *http.Response {
	t.Helper()
	p := newStubProvider(t)
	p.userInfo = map[string]any{"id": "google-sub-1", "email": fx.user.Email, "verified_email": true, "name": "Member"}
	app := newOAuthTestApp(t, "google", p)
	oauthAppBaseURL = ssoTestBase

	prevResolver := oauthResolver
	t.Cleanup(func() { oauthResolver = prevResolver })
	oauthResolver = appauth.NewResolveOAuthIdentityUseCase(linkedUserRepo{fx.user}, linkedRepo{fx.user.ID})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth2/login/google?return_to="+url.QueryEscape(returnTo), nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	authURL, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	state := authURL.Query().Get("state")
	code := "auth-code-" + state
	registerChallengeForCode(t, p, "google", state, code)

	return callbackWithCookie(t, app, "google", "state="+state+"&code="+code, state)
}

func TestOAuthCallback_Success_SetsCookiesAndReturnsToTheStartingPage(t *testing.T) {
	fx := setupSSOSession(t, nil)

	resp := oauthSignIn(t, fx, "/risks?focus=7")

	requireSessionCookiesWithoutLeak(t, resp)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, ssoTestBase+"/auth/sso/complete", loc.Scheme+"://"+loc.Host+loc.Path)
	require.Equal(t, "/risks?focus=7", loc.Query().Get("next"))
	require.Equal(t, 1, fx.audit.count(coreauth.AuditActionLogin))
}

func TestOAuthCallback_Success_OffSiteReturnToLandsHome(t *testing.T) {
	fx := setupSSOSession(t, nil)

	resp := oauthSignIn(t, fx, "//evil.example/phish")

	requireSessionCookiesWithoutLeak(t, resp)
	require.Equal(t, ssoTestBase+"/auth/sso/complete", resp.Header.Get("Location"))
}

// The SAML ACS hands its user to issueSSOSession with no return target, from a
// POST the IdP's form submits. An ACS-level test needs a signed-assertion
// fixture; until then this pins the exit the ACS calls.
func samlExit(t *testing.T, user *domain.User) *http.Response {
	t.Helper()
	app := fiber.New()
	app.Post("/api/v1/auth/saml2/acs", func(c *fiber.Ctx) error {
		return issueSSOSession(c, user, "saml2", "", oauthLocale(c))
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/auth/saml2/acs", nil), -1)
	require.NoError(t, err)
	return resp
}

func TestIssueSSOSession_SAMLExitSetsCookiesAndLandsHome(t *testing.T) {
	fx := setupSSOSession(t, nil)

	resp := samlExit(t, fx.user)

	requireSessionCookiesWithoutLeak(t, resp)
	require.Equal(t, ssoTestBase+"/auth/sso/complete", resp.Header.Get("Location"))
	require.Equal(t, 1, fx.audit.count(coreauth.AuditActionLogin))
}

func TestIssueSSOSession_SAMLExitFailureRedirectsToLogin(t *testing.T) {
	fx := setupSSOSession(t, errors.New("resolver down"))

	requireInternalFailure(t, samlExit(t, fx.user), "saml2")
}
