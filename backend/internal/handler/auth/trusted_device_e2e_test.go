// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

// D-064 (#879) over HTTP: the cookie planted by a sign-in gets that browser
// past a lock someone else set; a browser without it stays locked; revoking the
// sessions untrusts it.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

func TestTrustedDevice_GetsPastALockOthersStayOut(t *testing.T) {
	hasher := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{Time: 1, Memory: 1024, Threads: 1})
	stored, err := hasher.Hash(migrationPassword)
	require.NoError(t, err)
	f := newMigrationFixture(t, stored)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tm := coreauth.NewTokenManager(f.db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey})
	loginUC := appauth.NewLoginUseCase(repository.NewGormUserRepository(f.db), tm, hasher).
		WithClock(func() time.Time { return migrationNow }).
		WithAttemptLimits(authmfa.NewMemoryAttemptStore())
	h := authhandler.NewHandler(loginUC, nil, nil, nil, hasher, nil).
		WithTrustedDevices(coreauth.NewDeviceCookieSigner([]byte("test-device-key")))
	app := fiber.New()
	app.Post("/api/v1/auth/login", h.Login)

	post := func(password, cookie string) *http.Response {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"email": f.user.Email, "password": password})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: middleware.TrustedDeviceCookieName(), Value: cookie})
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp
	}

	// The owner signs in once: the browser is now trusted.
	first := post(migrationPassword, "")
	require.Equal(t, http.StatusOK, first.StatusCode)
	var cookie *http.Cookie
	for _, c := range first.Cookies() {
		if c.Name == middleware.TrustedDeviceCookieName() {
			cookie = c
		}
	}
	require.NotNil(t, cookie, "a full sign-in plants the cookie")
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, "/api/v1/auth", cookie.Path)

	// Someone else locks the address.
	for i := 1; i <= appauth.LoginMaxFailuresPerAddress; i++ {
		require.Equal(t, http.StatusUnauthorized, post("wrong-"+strconv.Itoa(i), "").StatusCode)
	}
	require.Equal(t, http.StatusTooManyRequests, post(migrationPassword, "").StatusCode, "a browser without the cookie stays locked")

	// The owner's browser gets in.
	require.Equal(t, http.StatusOK, post(migrationPassword, cookie.Value).StatusCode)

	// A password reset or change revokes the sessions, and the device with them.
	require.NoError(t, tm.RevokeAllUserTokens(context.Background(), f.user.ID))
	assert.Equal(t, http.StatusTooManyRequests, post(migrationPassword, cookie.Value).StatusCode)
}
