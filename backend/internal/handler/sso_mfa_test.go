// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// GHSA-rv5c-pr27-gpxh: an SSO sign-in passes the same MFA gate as a password
// sign-in. These pin the exit (no session while a factor is owed), the hand-off
// to the SPA (an HttpOnly cookie read once), and the gate itself against a real
// LoginUseCase.

// fixedSecondFactor answers every user with the same outcome.
type fixedSecondFactor struct {
	out *appauth.SecondFactorOutcome
	err error
}

func (f fixedSecondFactor) SSOSecondFactor(context.Context, uuid.UUID) (*appauth.SecondFactorOutcome, error) {
	return f.out, f.err
}

// requireNoSession: an SSO exit that owes a factor sets no session cookie.
func requireNoSession(t *testing.T, resp *http.Response) {
	t.Helper()
	cookies := cookiesByName(resp)
	require.Nil(t, cookies[middleware.AccessTokenCookie], "no session while a second factor is owed")
	require.Nil(t, cookies[middleware.RefreshTokenCookie], "no session while a second factor is owed")
}

// requireMFAHandOff: redirected to the completion page with mfa=1, the token in
// the scoped HttpOnly cookie and nowhere else. Returns the cookie value.
func requireMFAHandOff(t *testing.T, resp *http.Response, wantNext string) string {
	t.Helper()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	requireNoSession(t, resp)

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, ssoTestBase+ssoCompletePath, loc.Scheme+"://"+loc.Host+loc.Path)
	require.Equal(t, "1", loc.Query().Get("mfa"))
	require.Equal(t, wantNext, loc.Query().Get("next"))

	ck := cookiesByName(resp)[middleware.SSOMFACookie]
	require.NotNil(t, ck, "the MFA step must reach the SPA")
	require.True(t, ck.HttpOnly)
	require.Equal(t, middleware.SSOMFACookiePath, ck.Path)
	require.NotEmpty(t, ck.Value)

	body, _ := io.ReadAll(resp.Body)
	token := ck.Value[strings.Index(ck.Value, ":")+1:]
	require.NotContains(t, resp.Header.Get("Location"), token, "the MFA token must stay out of the URL")
	require.NotContains(t, string(body), token)
	return ck.Value
}

func TestIssueSSOSession_EnrolledAuthenticatorStopsAtChallenge(t *testing.T) {
	fx := setupSSOSession(t, nil)
	ssoSecondFactor = fixedSecondFactor{out: &appauth.SecondFactorOutcome{Step: appauth.SecondFactorChallenge, MFAToken: "challenge-token"}}

	resp := runIssueSSOSession(t, fx.user, "/risks")

	require.Equal(t, "challenge:challenge-token", requireMFAHandOff(t, resp, "/risks"))
	require.Equal(t, 1, fx.audit.count(coreauth.AuditActionLogin), "the first leg is audited like a password login's")
}

func TestIssueSSOSession_RoleMandateStopsAtEnrolment(t *testing.T) {
	fx := setupSSOSession(t, nil)
	ssoSecondFactor = fixedSecondFactor{out: &appauth.SecondFactorOutcome{Step: appauth.SecondFactorEnrollment, MFAToken: "enrol-token"}}

	resp := samlExit(t, fx.user)

	require.Equal(t, "enrollment:enrol-token", requireMFAHandOff(t, resp, ""))
}

// A deployment that forgot to wire the gate must not read as "no MFA".
func TestIssueSSOSession_UnwiredGateRefuses(t *testing.T) {
	fx := setupSSOSession(t, nil)
	ssoSecondFactor = nil

	requireInternalFailure(t, runIssueSSOSession(t, fx.user, ""), "google")
	require.Zero(t, fx.audit.count(coreauth.AuditActionLogin))
}

func TestIssueSSOSession_GateFailureRefuses(t *testing.T) {
	fx := setupSSOSession(t, nil)
	ssoSecondFactor = fixedSecondFactor{err: errors.New("mfa store down")}

	requireInternalFailure(t, runIssueSSOSession(t, fx.user, ""), "google")
}

// runSSOMFAStep posts to the claim endpoint with the given cookie value.
func runSSOMFAStep(t *testing.T, cookie string) (*http.Response, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Post(middleware.SSOMFACookiePath, SSOMFAStep)
	req := httptest.NewRequest(http.MethodPost, middleware.SSOMFACookiePath, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: middleware.SSOMFACookie, Value: cookie})
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp, body
}

func requireMFACookieCleared(t *testing.T, resp *http.Response) {
	t.Helper()
	ck := cookiesByName(resp)[middleware.SSOMFACookie]
	require.NotNil(t, ck, "the step must be spent on first read")
	require.Empty(t, ck.Value)
	require.Equal(t, middleware.SSOMFACookiePath, ck.Path)
}

func TestSSOMFAStep_Success(t *testing.T) {
	resp, body := runSSOMFAStep(t, "challenge:tok.en.value")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, body["mfa_required"])
	require.Equal(t, "tok.en.value", body["mfa_token"])
	require.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	requireMFACookieCleared(t, resp)

	resp, body = runSSOMFAStep(t, "enrollment:enrol.tok")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, body["mfa_enrollment_required"])
	require.Equal(t, "enrol.tok", body["mfa_token"])
	requireMFACookieCleared(t, resp)
}

func TestSSOMFAStep_NotFound(t *testing.T) {
	for _, cookie := range []string{"", "challenge:", "session:abc", "abc"} {
		t.Run(cookie, func(t *testing.T) {
			resp, body := runSSOMFAStep(t, cookie)
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
			require.Equal(t, "SSO_MFA_NOT_FOUND", body["code"])
			require.NotContains(t, body, "mfa_token")
		})
	}
}

// ---------------------------------------------------------------------------
// Against the real gate
// ---------------------------------------------------------------------------

// secretOnlyMFARepo answers GetMFASecret from a map; nothing else is called on
// the sign-in path.
type secretOnlyMFARepo struct {
	repository.MFARepository
	secrets map[uuid.UUID]*domain.MFASecret
}

func (r secretOnlyMFARepo) GetMFASecret(_ context.Context, userID, _ uuid.UUID) (*domain.MFASecret, error) {
	return r.secrets[userID], nil
}

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return p, nil }
func (plainHasher) Verify(h, p string) bool       { return h == p }

// realGate wires the production LoginUseCase as the SSO gate, with admin as a
// privileged role, and returns the MFA store it reads.
func realGate(t *testing.T, fx *ssoFixture, now time.Time) secretOnlyMFARepo {
	t.Helper()
	mfa := secretOnlyMFARepo{secrets: map[uuid.UUID]*domain.MFASecret{}}
	ssoSecondFactor = appauth.NewLoginUseCase(repository.NewGormUserRepository(fx.db), fx.tm, plainHasher{}).
		WithMFA(mfa).
		RequireMFAForRoles([]string{"admin", "root"}, nil).
		WithClock(func() time.Time { return now })
	return mfa
}

// claimToken reads the step the SPA would collect and checks its token type.
func claimToken(t *testing.T, fx *ssoFixture, cookie, wantType string) {
	t.Helper()
	_, body := runSSOMFAStep(t, cookie)
	tok, _ := body["mfa_token"].(string)
	require.NotEmpty(t, tok)
	claims, err := authpkg.ValidateToken(fx.keys, tok, nil)
	require.NoError(t, err)
	require.Equal(t, wantType, claims.Type)
	require.Empty(t, claims.Permissions, "an MFA token carries no permission")
}

// The advisory's case: a user who enrolled an OpenRisk authenticator signs in
// with Google and is asked for it.
func TestOAuthCallback_EnrolledAuthenticatorIsChallenged(t *testing.T) {
	fx := setupSSOSession(t, nil)
	mfa := realGate(t, fx, time.Now())
	mfa.secrets[fx.user.ID] = &domain.MFASecret{UserID: fx.user.ID, TenantID: fx.orgID, IsVerified: true}

	resp := oauthSignIn(t, fx, "/risks?focus=7")

	cookie := requireMFAHandOff(t, resp, "/risks?focus=7")
	claimToken(t, fx, cookie, authpkg.TokenTypeMFARequired)
}

// An unconfirmed secret is not a factor: an ordinary member signs in as before.
func TestOAuthCallback_UnverifiedSecretIsNoFactor(t *testing.T) {
	fx := setupSSOSession(t, nil)
	require.NoError(t, fx.db.Exec(`UPDATE organization_members SET role = 'user' WHERE user_id = ?`, fx.user.ID.String()).Error)
	mfa := realGate(t, fx, time.Now())
	mfa.secrets[fx.user.ID] = &domain.MFASecret{UserID: fx.user.ID, TenantID: fx.orgID, IsVerified: false}

	requireSessionCookiesWithoutLeak(t, oauthSignIn(t, fx, ""))
}

// The role policy: an admin with nothing enrolled, past the grace window, is
// sent to enrolment instead of being signed in.
func TestOAuthCallback_RolePolicyPastGraceRequiresEnrolment(t *testing.T) {
	fx := setupSSOSession(t, nil)
	require.NoError(t, fx.db.Exec(`UPDATE organization_members SET role = 'admin', joined_at = ? WHERE user_id = ?`,
		time.Now().AddDate(0, 0, -60), fx.user.ID.String()).Error)
	realGate(t, fx, time.Now())

	resp := oauthSignIn(t, fx, "")

	cookie := requireMFAHandOff(t, resp, "")
	claimToken(t, fx, cookie, authpkg.TokenTypeMFAEnrollment)
}

// Inside the grace window the admin still gets in, as with a password.
func TestOAuthCallback_RolePolicyInsideGraceSignsIn(t *testing.T) {
	fx := setupSSOSession(t, nil)
	require.NoError(t, fx.db.Exec(`UPDATE organization_members SET role = 'admin', joined_at = ? WHERE user_id = ?`,
		time.Now(), fx.user.ID.String()).Error)
	realGate(t, fx, time.Now())

	requireSessionCookiesWithoutLeak(t, oauthSignIn(t, fx, ""))
}
