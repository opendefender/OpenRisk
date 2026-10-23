// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// #727 — the session list and "sign out other devices" through the real HTTP stack: the auth
// middleware, the session handler, the GORM session projection, the token
// manager and the refresh handler, mounted as main.go mounts them.
//
// Requests carry only what a browser would send to each route: the access
// cookie everywhere, the refresh cookie only to /api/v1/auth/refresh, where
// its Path scopes it.
// ---------------------------------------------------------------------------

type sessionStack struct {
	app    *fiber.App
	db     *gorm.DB
	tokens *coreauth.TokenManager
	keys   *authpkg.RSAKeys
	userID uuid.UUID
	orgID  uuid.UUID
	// devices signs trusted-device cookies the way main.go's signer does.
	devices *coreauth.DeviceCookieSigner
}

// usersTable reads the one column the trust check needs from the minimal
// users table, which GormUserRepository's full model does not fit.
type usersTable struct{ db *gorm.DB }

func (u usersTable) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var row struct{ DevicesRevokedAt *time.Time }
	if err := u.db.WithContext(ctx).Table("users").Select("devices_revoked_at").Where("id = ?", id).Take(&row).Error; err != nil {
		return nil, err
	}
	return &domain.User{ID: id, DevicesRevokedAt: row.DevicesRevokedAt}, nil
}

func newSessionStack(t *testing.T) *sessionStack {
	t.Helper()
	return newSessionStackWithUsers(t, nil)
}

// newSessionStackWithUsers wires the trust check to users; nil reads the
// stack's own users table.
func newSessionStackWithUsers(t *testing.T, users authhandler.UserByIDReader) *sessionStack {
	t.Helper()

	dsn := "file:sessions_others_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`).Error)
	// "Sign out other devices" also stamps users.devices_revoked_at (D-064).
	require.NoError(t, db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, devices_revoked_at DATETIME, deleted_at DATETIME)`).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	s := &sessionStack{db: db, keys: keys, userID: uuid.New(), orgID: uuid.New(),
		devices: coreauth.NewDeviceCookieSigner([]byte("test-device-key"))}
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (?)`, s.userID.String()).Error)
	s.tokens = coreauth.NewTokenManager(db, keys)
	// What main.go's org resolver derives from an active membership, reduced to
	// the one account these tests use.
	s.tokens.SetOrgSessionResolver(func(_ context.Context, _, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: "user"}}, nil
	})

	if users == nil {
		users = usersTable{db}
	}
	sessionRepo := repository.NewGormSessionRepository(db)
	sessions := authhandler.NewSessionHandler(
		appauth.NewListSessionsUseCase(sessionRepo),
		appauth.NewRevokeSessionUseCase(sessionRepo),
		appauth.NewRevokeOtherSessionsUseCase(sessionRepo),
		nil,
	).WithTrustedDevices(s.devices, users)
	blacklist := authpkg.NewTokenBlacklistManager(&memoryBlacklist{keys: map[string]string{}})
	logout := appauth.NewLogoutUseCase(s.tokens).WithAccessTokenRevocation(blacklist).WithSessionRevocation(s.tokens)
	h := authhandler.NewHandler(nil, nil, appauth.NewRefreshTokenUseCase(s.tokens), logout, nil, nil).
		WithAccessTokenRevocation(keys)

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Post("/auth/refresh", h.RefreshToken)
	api.Post("/auth/logout", h.Logout)
	// Stands in for middleware.PATMiddleware, which main.go mounts before
	// Protected: it sets the same locals and context for a PAT-authenticated
	// request, and Protected then lets the request through untouched.
	api.Use(func(c *fiber.Ctx) error {
		if c.Get("X-Test-PAT") == "" {
			return c.Next()
		}
		c.Locals("is_pat", true)
		middleware.SetContext(c, &middleware.RequestContext{UserID: s.userID, OrganizationID: s.orgID})
		return c.Next()
	})
	protected := api.Use(middleware.Protected(keys, func(string) (bool, error) { return false, nil }))
	protected.Get("/auth/sessions", sessions.ListSessions)
	protected.Delete("/auth/sessions/others", sessions.RevokeOtherSessions)
	s.app = app
	return s
}

// signIn opens a session on a named device, as login does.
func (s *sessionStack) signIn(t *testing.T, device string) *coreauth.TokenPair {
	t.Helper()
	pair, err := s.tokens.IssueSessionForOrg(context.Background(), s.userID, s.orgID, coreauth.DeviceContext{UserAgent: device})
	require.NoError(t, err)
	return pair
}

type sessionResponse struct {
	status  int
	body    map[string]any
	cookies map[string]*http.Cookie
}

func (s *sessionStack) send(t *testing.T, method, path string, cookies map[string]string, headers map[string]string) sessionResponse {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := s.app.Test(req, 10000)
	require.NoError(t, err)
	raw, _ := io.ReadAll(res.Body)
	out := sessionResponse{status: res.StatusCode, cookies: map[string]*http.Cookie{}}
	_ = json.Unmarshal(raw, &out.body)
	for _, c := range res.Cookies() {
		out.cookies[c.Name] = c
	}
	return out
}

// refresh is the browser's refresh: the refresh cookie and nothing else.
func (s *sessionStack) refresh(t *testing.T, refreshToken string) sessionResponse {
	t.Helper()
	return s.send(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{middleware.RefreshTokenCookie: refreshToken}, nil)
}

func (s *sessionStack) liveSessions(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Table("refresh_tokens").Where("user_id = ? AND rotated_at IS NULL", s.userID).Count(&n).Error)
	return n
}

// browserCookies is what the SPA's browser attaches to an authenticated API
// call: the access cookie and the CSRF cookie, whose value the SPA echoes in
// the header (see browserHeaders).
func browserCookies(pair *coreauth.TokenPair) map[string]string {
	return map[string]string{middleware.AccessTokenCookie: pair.AccessToken, middleware.CSRFCookie: "csrf-727"}
}

var browserHeaders = map[string]string{middleware.CSRFHeader: "csrf-727"}

// legacyAccessToken is an access token minted before the sid claim existed,
// still valid for up to AccessTokenTTL after the deploy that adds it.
func (s *sessionStack) legacyAccessToken(t *testing.T) *coreauth.TokenPair {
	t.Helper()
	token, _, err := authpkg.GenerateAccessToken(s.keys, s.userID, s.orgID,
		map[uuid.UUID]string{s.orgID: "user"}, nil, nil, coreauth.AccessTokenTTL)
	require.NoError(t, err)
	return &coreauth.TokenPair{AccessToken: token}
}

// rotate refreshes a device the way the browser does and returns its new pair,
// leaving a spent row behind for reuse detection.
func (s *sessionStack) rotate(t *testing.T, pair *coreauth.TokenPair) *coreauth.TokenPair {
	t.Helper()
	res := s.refresh(t, pair.RefreshToken)
	require.Equal(t, http.StatusOK, res.status, res.body)
	access, refresh := res.cookies[middleware.AccessTokenCookie], res.cookies[middleware.RefreshTokenCookie]
	require.NotNil(t, access)
	require.NotNil(t, refresh)
	return &coreauth.TokenPair{AccessToken: access.Value, RefreshToken: refresh.Value}
}

// listSessions is GET /auth/sessions as the browser sends it: access cookie
// only, never the refresh cookie.
func (s *sessionStack) listSessions(t *testing.T, pair *coreauth.TokenPair) []map[string]any {
	t.Helper()
	res := s.send(t, http.MethodGet, "/api/v1/auth/sessions",
		map[string]string{middleware.AccessTokenCookie: pair.AccessToken}, nil)
	require.Equal(t, http.StatusOK, res.status, res.body)
	raw, ok := res.body["sessions"].([]any)
	require.True(t, ok, "sessions: %v", res.body)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.(map[string]any))
	}
	return out
}

func currentDevices(sessions []map[string]any) []string {
	var devices []string
	for _, row := range sessions {
		if row["current"] == true {
			devices = append(devices, row["user_agent"].(string))
		}
	}
	return devices
}

// Criterion 1: the other device is signed out, the caller is not, and keeps
// the very session it had.
func TestRevokeOtherSessionsE2E_BrowserCallerStaysSignedIn(t *testing.T) {
	s := newSessionStack(t)
	laptop := s.rotate(t, s.signIn(t, "laptop"))
	phone := s.signIn(t, "phone")

	// The browser sends the access cookie here; the refresh cookie's Path keeps
	// it off this route. The access token's sid names the laptop's session.
	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", browserCookies(laptop), browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, false, res.body["reauthenticate"])
	assert.EqualValues(t, 1, res.body["revoked"], "the phone, and nothing else")
	assert.Nil(t, res.body["token_pair"], "the caller keeps its session; nothing is re-issued")
	assert.Nil(t, res.cookies[middleware.RefreshTokenCookie])
	assert.Equal(t, int64(1), s.liveSessions(t), "exactly one session survives: the caller's")

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, phone.RefreshToken).status,
		"the other device can no longer refresh")

	// The refresh cookie the browser already holds still works, and keeps
	// working as the lineage rotates on.
	next := s.rotate(t, laptop)
	s.rotate(t, next)
}

// Criterion 2: the list marks exactly one row, the caller's, for a browser that
// sends no refresh token, and keeps marking it across rotations.
func TestListSessionsE2E_MarksTheCallersDevice(t *testing.T) {
	s := newSessionStack(t)
	laptopAtSignIn := s.signIn(t, "laptop")
	laptop := s.rotate(t, s.rotate(t, laptopAtSignIn))
	phone := s.rotate(t, s.signIn(t, "phone"))

	sessions := s.listSessions(t, laptop)
	assert.Len(t, sessions, 2, "one row per device: spent tokens are not sessions")
	assert.Equal(t, []string{"laptop"}, currentDevices(sessions))

	assert.Equal(t, []string{"phone"}, currentDevices(s.listSessions(t, phone)))

	// An access token from before the rotations names the same session.
	assert.Equal(t, []string{"laptop"}, currentDevices(s.listSessions(t, laptopAtSignIn)))
}

// An access token minted before sid existed cannot name its session. It is
// never handed a new one (#1060): every session ends, the caller's included,
// and the caller is told to sign in again.
func TestRevokeOtherSessionsE2E_LegacyTokenSignsOutEverywhere(t *testing.T) {
	s := newSessionStack(t)
	laptop := s.signIn(t, "laptop")
	phone := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", browserCookies(s.legacyAccessToken(t)), browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, true, res.body["reauthenticate"])
	assert.EqualValues(t, 2, res.body["revoked"])
	assert.Nil(t, res.body["token_pair"], "no session is minted for a token that names none")
	if issued := res.cookies[middleware.RefreshTokenCookie]; issued != nil {
		assert.Empty(t, issued.Value, "the refresh cookie may only be cleared, never set")
	}
	assert.Equal(t, int64(0), s.liveSessions(t))

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, phone.RefreshToken).status)
	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, laptop.RefreshToken).status)
}

// An API client on a legacy token that names its session in X-Refresh-Token
// keeps that very session; nothing is re-issued and no cookie is set.
func TestRevokeOtherSessionsE2E_PresentedSessionIsKept(t *testing.T) {
	s := newSessionStack(t)
	client := s.signIn(t, "ci-runner")
	other := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", browserCookies(s.legacyAccessToken(t)),
		map[string]string{middleware.CSRFHeader: "csrf-727", "X-Refresh-Token": client.RefreshToken})
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.EqualValues(t, 1, res.body["revoked"])
	assert.Nil(t, res.body["token_pair"])
	assert.Nil(t, res.cookies[middleware.RefreshTokenCookie])

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, other.RefreshToken).status)
	assert.Equal(t, http.StatusOK, s.refresh(t, client.RefreshToken).status)
}

func TestRevokeOtherSessionsE2E_Unauthorized(t *testing.T) {
	s := newSessionStack(t)
	phone := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, res.status)
	assert.Equal(t, int64(1), s.liveSessions(t), "an anonymous call revokes nothing")
	assert.Equal(t, http.StatusOK, s.refresh(t, phone.RefreshToken).status)
}

// A personal access token never carries a refresh cookie. Re-issuing for it
// would turn a scoped, revocable token into a full 30-day browser session, so
// the route refuses PATs outright and touches nothing.
func TestRevokeOtherSessionsE2E_PersonalAccessTokenRefused(t *testing.T) {
	s := newSessionStack(t)
	phone := s.signIn(t, "phone")

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", nil, map[string]string{"X-Test-PAT": "1"})
	assert.Equal(t, http.StatusForbidden, res.status, res.body)
	assert.Nil(t, res.body["token_pair"], "a PAT must never be handed a session")
	assert.Nil(t, res.cookies[middleware.RefreshTokenCookie])
	assert.Equal(t, int64(1), s.liveSessions(t), "nothing is minted and nothing is revoked")
	assert.Equal(t, http.StatusOK, s.refresh(t, phone.RefreshToken).status)
}

// #1061: the browser sends only its access cookie to /auth/logout, never the
// refresh cookie. The session its sid names must end anyway.
func TestLogoutE2E_BrowserLogoutEndsItsSession(t *testing.T) {
	s := newSessionStack(t)
	laptop := s.rotate(t, s.signIn(t, "laptop"))
	phone := s.signIn(t, "phone")

	res := s.send(t, http.MethodPost, "/api/v1/auth/logout", browserCookies(laptop), browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)

	assert.Equal(t, http.StatusUnauthorized, s.refresh(t, laptop.RefreshToken).status,
		"the refresh token the browser held is dead, wherever a copy of it went")
	assert.Equal(t, int64(1), s.liveSessions(t), "only the phone's session is left")
	s.rotate(t, phone)
}

// The sid only ever ends a session of the token's own user.
func TestLogoutE2E_ForeignSidRevokesNothing(t *testing.T) {
	s := newSessionStack(t)
	phone := s.signIn(t, "phone")
	var family string
	require.NoError(t, s.db.Table("refresh_tokens").Select("family_id").Where("user_id = ?", s.userID).Scan(&family).Error)

	stranger, _, err := authpkg.GenerateSessionAccessToken(s.keys, uuid.New(), s.orgID, nil, nil, nil,
		coreauth.AccessTokenTTL, uuid.MustParse(family))
	require.NoError(t, err)

	res := s.send(t, http.MethodPost, "/api/v1/auth/logout",
		map[string]string{middleware.AccessTokenCookie: stranger}, nil)
	assert.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, int64(1), s.liveSessions(t))
	s.rotate(t, phone)
}

// #1118/#1119: "sign out other devices" re-issues the caller's trusted-device
// cookie only when the caller holds one the account still trusts, on both the
// sid path and the refresh-hash path.
func TestRevokeOtherSessionsE2E_TrustedDeviceReissue(t *testing.T) {
	trusted := middleware.TrustedDeviceCookieName()

	type path struct {
		cookies func(s *sessionStack) map[string]string
		headers func(s *sessionStack) map[string]string
	}
	paths := map[string]func(t *testing.T, s *sessionStack) path{
		"sid": func(t *testing.T, s *sessionStack) path {
			laptop := s.signIn(t, "laptop")
			return path{
				cookies: func(*sessionStack) map[string]string { return browserCookies(laptop) },
				headers: func(*sessionStack) map[string]string { return browserHeaders },
			}
		},
		"refresh hash": func(t *testing.T, s *sessionStack) path {
			client := s.signIn(t, "ci-runner")
			legacy := s.legacyAccessToken(t)
			return path{
				cookies: func(*sessionStack) map[string]string { return browserCookies(legacy) },
				headers: func(*sessionStack) map[string]string {
					return map[string]string{middleware.CSRFHeader: "csrf-727", "X-Refresh-Token": client.RefreshToken}
				},
			}
		},
	}
	cases := map[string]struct {
		cookie func(t *testing.T, s *sessionStack) string
		want   bool
	}{
		"a cookie the account trusts is re-issued": {
			cookie: func(_ *testing.T, s *sessionStack) string { return s.devices.Issue(s.userID) },
			want:   true,
		},
		"no cookie, none minted": {
			cookie: func(*testing.T, *sessionStack) string { return "" },
		},
		"another user's cookie is not": {
			cookie: func(_ *testing.T, s *sessionStack) string { return s.devices.Issue(uuid.New()) },
		},
		"a cookie an earlier revocation ended is not": {
			cookie: func(t *testing.T, s *sessionStack) string {
				c := s.devices.Issue(s.userID)
				time.Sleep(2 * time.Millisecond)
				require.NoError(t, s.db.Exec(`UPDATE users SET devices_revoked_at = ? WHERE id = ?`, time.Now(), s.userID.String()).Error)
				return c
			},
		},
	}
	for pname, mk := range paths {
		for cname, tc := range cases {
			t.Run(pname+"/"+cname, func(t *testing.T) {
				s := newSessionStack(t)
				p := mk(t, s)
				s.signIn(t, "phone")
				cookies := p.cookies(s)
				if v := tc.cookie(t, s); v != "" {
					cookies[trusted] = v
				}

				res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", cookies, p.headers(s))
				require.Equal(t, http.StatusOK, res.status, res.body)
				issued := res.cookies[trusted]
				if !tc.want {
					assert.True(t, issued == nil || issued.Value == "", "no trusted-device cookie may be set")
					return
				}
				require.NotNil(t, issued, "the caller's trusted-device cookie is re-issued")
				d := s.devices.Verify(issued.Value)
				require.NotNil(t, d)
				var at *time.Time
				require.NoError(t, s.db.Raw(`SELECT devices_revoked_at FROM users WHERE id = ?`, s.userID.String()).Scan(&at).Error)
				assert.True(t, d.TrustsFor(&domain.User{ID: s.userID, DevicesRevokedAt: at}),
					"the re-issued cookie survives the stamp this revocation set")
			})
		}
	}
}

type failingUsers struct{}

func (failingUsers) GetByID(context.Context, uuid.UUID) (*domain.User, error) {
	return nil, errors.New("database unavailable")
}

// The trust check fails closed: when the user cannot be read, the revocation
// still happens and no trusted-device cookie is re-issued.
func TestRevokeOtherSessionsE2E_UnreadableUserReissuesNothing(t *testing.T) {
	s := newSessionStackWithUsers(t, failingUsers{})
	laptop := s.signIn(t, "laptop")
	s.signIn(t, "phone")
	cookies := browserCookies(laptop)
	cookies[middleware.TrustedDeviceCookieName()] = s.devices.Issue(s.userID)

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", cookies, browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.EqualValues(t, 1, res.body["revoked"])
	issued := res.cookies[middleware.TrustedDeviceCookieName()]
	assert.True(t, issued == nil || issued.Value == "", "no trusted-device cookie on an unreadable user")
}

// A caller whose own session ends with the others (a legacy token naming no
// session) gets no fresh 90 days on its cookie: it signs in again, and that
// sign-in plants one.
func TestRevokeOtherSessionsE2E_ReauthenticatingCallerGetsNoCookie(t *testing.T) {
	s := newSessionStack(t)
	s.signIn(t, "laptop")
	cookies := browserCookies(s.legacyAccessToken(t))
	cookies[middleware.TrustedDeviceCookieName()] = s.devices.Issue(s.userID)

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", cookies, browserHeaders)
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, true, res.body["reauthenticate"])
	issued := res.cookies[middleware.TrustedDeviceCookieName()]
	assert.True(t, issued == nil || issued.Value == "", "no trusted-device cookie for a caller signed out too")
}

// An API client presenting a refresh token that names no live session (here,
// one already rotated) keeps nothing: it is told to sign in again, and its
// trusted-device cookie gets no fresh 90 days.
func TestRevokeOtherSessionsE2E_StalePresentedSessionReauthenticates(t *testing.T) {
	s := newSessionStack(t)
	client := s.signIn(t, "ci-runner")
	s.rotate(t, client) // client.RefreshToken is now spent
	s.signIn(t, "phone")
	cookies := browserCookies(s.legacyAccessToken(t))
	cookies[middleware.TrustedDeviceCookieName()] = s.devices.Issue(s.userID)

	res := s.send(t, http.MethodDelete, "/api/v1/auth/sessions/others", cookies,
		map[string]string{middleware.CSRFHeader: "csrf-727", "X-Refresh-Token": client.RefreshToken})
	require.Equal(t, http.StatusOK, res.status, res.body)
	assert.Equal(t, true, res.body["reauthenticate"])
	assert.Equal(t, int64(0), s.liveSessions(t), "nothing named a live session, so nothing survives")
	issued := res.cookies[middleware.TrustedDeviceCookieName()]
	assert.True(t, issued == nil || issued.Value == "", "no trusted-device cookie for a caller signed out too")
}
