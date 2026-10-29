// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial

package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// An account signs in with a password or with a provider, never both (#1150).

// unlinkedRepo has no link for any identity and records every link written.
type unlinkedRepo struct{ created *int }

func (unlinkedRepo) FindByProviderSubject(context.Context, string, string) (*domain.OAuthProvider, error) {
	return nil, nil
}
func (unlinkedRepo) ListByUser(context.Context, uuid.UUID) ([]domain.OAuthProvider, error) {
	return nil, nil
}
func (r unlinkedRepo) Create(context.Context, *domain.OAuthProvider) error {
	*r.created++
	return nil
}
func (unlinkedRepo) TouchLogin(context.Context, uuid.UUID, time.Time) error { return nil }

func TestOAuthCallback_PasswordAccountIsRefusedAndNotLinked(t *testing.T) {
	user := &domain.User{
		ID: uuid.New(), Email: "member@opendefender.io", Username: "member",
		Password: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA", IsActive: true,
	}
	p := newStubProvider(t)
	p.userInfo = map[string]any{"id": "google-sub-9", "email": user.Email, "verified_email": true, "name": "Member"}
	app := newOAuthTestApp(t, "google", p)

	created := 0
	prevResolver := oauthResolver
	t.Cleanup(func() { oauthResolver = prevResolver })
	oauthResolver = appauth.NewResolveOAuthIdentityUseCase(linkedUserRepo{user}, unlinkedRepo{&created})

	state, _ := startFlow(t, app, "google")
	code := "auth-code-" + state
	registerChallengeForCode(t, p, "google", state, code)

	resp := callbackWithCookie(t, app, "google", "state="+state+"&code="+code, state)

	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, "/login", loc.Path)
	require.Equal(t, "password_account", loc.Query().Get("error"))
	require.Equal(t, "google", loc.Query().Get("provider"))
	require.Zero(t, created, "no provider link may be written on a password account")
	for _, ck := range resp.Cookies() {
		require.NotContains(t, []string{"or_access", "or_refresh", "or_csrf"}, ck.Name, "a refused sign-in must not set a session")
	}
}
