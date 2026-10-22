// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// ---------------------------------------------------------------------------
// #1123 — a personal access token carries its scopes and dies when revoked. A
// route that mints a session out of the caller's credential would turn any PAT
// into a full, refreshable session of its owner. The two such routes behind
// `protected` must refuse a PAT before minting anything.
//
// The stack is the production chain: PATMiddleware with a real token service,
// then the RS256 gate, then the real handlers and use cases over the real token
// manager, so "nothing was minted" is read from the refresh_tokens table.
// ---------------------------------------------------------------------------

const mintTestPassword = "Old-Password-Still-Long-1"

type mintPATRepo struct{ rows []*domain.PersonalAccessToken }

func (r *mintPATRepo) Create(_ context.Context, t *domain.PersonalAccessToken) error {
	t.ID = uuid.New()
	r.rows = append(r.rows, t)
	return nil
}
func (r *mintPATRepo) GetByTokenHash(_ context.Context, h string) (*domain.PersonalAccessToken, error) {
	for _, t := range r.rows {
		if t.TokenHash == h {
			return t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *mintPATRepo) ListByOwner(context.Context, uuid.UUID, uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	return nil, nil
}
func (r *mintPATRepo) UpdateLastUsed(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (r *mintPATRepo) DeleteByOwner(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

// mintMembers makes the user an active member of every organization asked
// about, so a refusal can only come from the credential, never the membership.
type mintMembers struct{}

func (mintMembers) ListActiveMemberships(context.Context, uuid.UUID) ([]*domain.OrganizationMember, error) {
	return nil, nil
}
func (mintMembers) GetOrganizationMember(_ context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error) {
	return &domain.OrganizationMember{UserID: userID, OrganizationID: orgID, Role: domain.MemberRole("admin"), IsActive: true}, nil
}
func (mintMembers) GetByID(context.Context, uuid.UUID) (*domain.User, error) { return nil, nil }

type mintUsers struct{ u *domain.User }

func (m *mintUsers) GetByEmail(context.Context, string) (*domain.User, error) { return nil, nil }
func (m *mintUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if m.u.ID == id {
		return m.u, nil
	}
	return nil, nil
}
func (m *mintUsers) Update(_ context.Context, u *domain.User) error { m.u = u; return nil }

type mintHasher struct{}

func (mintHasher) Hash(p string) (string, error)  { return "h:" + p, nil }
func (mintHasher) Verify(hash, plain string) bool { return hash == "h:"+plain }

type mintStack struct {
	*sessionStack
	app   *fiber.App
	users *mintUsers
	pat   string
}

func newMintStack(t *testing.T) *mintStack {
	t.Helper()
	s := newSessionStack(t)

	svc := coreauth.NewPersonalAccessTokenService(&mintPATRepo{})
	_, raw, err := svc.CreateToken(context.Background(), s.userID, s.orgID, "CI", "", []string{"risks:read"}, nil)
	require.NoError(t, err)
	resolve := func(_ context.Context, _, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		return &coreauth.SessionClaims{TenantID: orgID, Permissions: []string{"*"}}, nil
	}

	users := &mintUsers{u: &domain.User{ID: s.userID, Email: "a@x.io", IsActive: true, Password: "h:" + mintTestPassword}}
	switcher := authhandler.NewSwitchHandler(nil, appauth.NewSwitchOrganizationUseCase(mintMembers{}, s.tokens), nil)
	password := authhandler.NewPasswordHandler(nil, nil, pwpolicy.New(), "", nil).
		WithChangePassword(appauth.NewChangePasswordUseCase(users, mintHasher{}, pwpolicy.New(), s.tokens, nil))

	app := fiber.New()
	api := app.Group("/api/v1")
	api.Use(middleware.PATMiddleware(svc, resolve))
	protected := api.Use(middleware.Protected(s.keys, func(string) (bool, error) { return false, nil }))
	protected.Post("/auth/switch-org", switcher.SwitchOrganization)
	protected.Post("/auth/password/change", password.ChangePassword)

	return &mintStack{sessionStack: s, app: app, users: users, pat: raw}
}

func (m *mintStack) post(t *testing.T, path, bearer string, body any) (*http.Response, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	res, err := m.app.Test(req, -1)
	require.NoError(t, err)
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	var parsed map[string]any
	_ = json.Unmarshal(out, &parsed)
	return res, parsed
}

func (m *mintStack) sessions(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, m.db.Table("refresh_tokens").Count(&n).Error)
	return n
}

// assertNothingMinted checks the three places a session could leak out: the
// cookies, the body and the session store.
func (m *mintStack) assertNothingMinted(t *testing.T, res *http.Response, body map[string]any, before int64) {
	t.Helper()
	assert.Equal(t, fiber.StatusForbidden, res.StatusCode, "body: %v", body)
	msg, _ := body["error"].(string)
	assert.True(t, strings.HasPrefix(msg, "personal access tokens cannot "), "error = %q", msg)
	assert.Empty(t, res.Header.Values("Set-Cookie"), "a PAT must not receive session cookies")
	assert.NotContains(t, body, "token_pair")
	assert.NotContains(t, body, "csrf_token")
	assert.Equal(t, before, m.sessions(t), "a PAT must not open a session")
}

func TestSwitchOrganization_PATIsRefused(t *testing.T) {
	m := newMintStack(t)
	before := m.sessions(t)

	res, body := m.post(t, "/api/v1/auth/switch-org", m.pat, map[string]string{"organization_id": uuid.NewString()})

	m.assertNothingMinted(t, res, body, before)
}

// The refusal is about the credential: a real session still switches.
func TestSwitchOrganization_SessionStillSwitches(t *testing.T) {
	m := newMintStack(t)
	pair := m.signIn(t, "laptop")
	before := m.sessions(t)

	res, body := m.post(t, "/api/v1/auth/switch-org", pair.AccessToken, map[string]string{"organization_id": uuid.NewString()})

	require.Equal(t, fiber.StatusOK, res.StatusCode, "body: %v", body)
	assert.Contains(t, body, "token_pair")
	assert.NotEmpty(t, res.Header.Values("Set-Cookie"))
	assert.Equal(t, before+1, m.sessions(t))
}

func TestChangePassword_PATIsRefused(t *testing.T) {
	m := newMintStack(t)
	before := m.sessions(t)

	res, body := m.post(t, "/api/v1/auth/password/change", m.pat, map[string]string{
		"current_password": mintTestPassword,
		"new_password":     "Violet-Kilimanjaro-Anchor-2026!",
	})

	m.assertNothingMinted(t, res, body, before)
	assert.Equal(t, "h:"+mintTestPassword, m.users.u.Password, "a PAT must not change the password")
}
