// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

// #688 over HTTP: the per-address lock through the real login route, the real
// GORM user repository, the real hasher and the audit trail.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/authmfa"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

func TestLoginThrottle_LockedAddressAnswers429AndEveryFailureIsAudited(t *testing.T) {
	hasher := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{Time: 1, Memory: 1024, Threads: 1})
	stored, err := hasher.Hash(migrationPassword)
	require.NoError(t, err)
	f := newMigrationFixture(t, stored)

	require.NoError(t, f.db.Exec(`CREATE TABLE auth_audit_logs (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(f.db, "auth_audit_logs", &domain.AuthAuditLog{}))
	audit := coreauth.NewAuditService(repository.NewGormAuthAuditLogRepository(f.db))

	loginUC := appauth.NewLoginUseCase(repository.NewGormUserRepository(f.db), coreauth.NewTokenManager(f.db, nil), hasher).
		WithAttemptLimits(authmfa.NewMemoryAttemptStore())
	f.app = newLoginApp(authhandler.NewHandler(loginUC, nil, nil, nil, hasher, audit))

	for i := 1; i <= appauth.LoginMaxFailuresPerAddress; i++ {
		status, _ := f.login(t, "wrong-"+strconv.Itoa(i))
		require.Equal(t, http.StatusUnauthorized, status, "failure %d", i)
	}

	// Correct password, but the address is locked.
	resp := postLogin(t, f, strings.ToUpper(f.user.Email), migrationPassword)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	wait, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	require.NoError(t, err)
	assert.InDelta(t, appauth.LoginLockDuration.Seconds(), float64(wait), 2)
	var body struct {
		Code       string `json:"code"`
		RetryAfter int    `json:"retry_after"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "LOGIN_LOCKED", body.Code)
	assert.Equal(t, wait, body.RetryAfter)

	var reasons []string
	require.NoError(t, f.db.Model(&domain.AuthAuditLog{}).Where("success = ?", false).
		Order("created_at").Pluck("failure_reason", &reasons).Error)
	require.Len(t, reasons, appauth.LoginMaxFailuresPerAddress+1)
	hashed := "email_sha256=" + domain.HashEmailForReset(f.user.Email)
	for _, r := range reasons {
		assert.Contains(t, r, hashed)
		assert.NotContains(t, r, f.user.Email, "never the address in clear")
	}
	assert.True(t, strings.HasPrefix(reasons[len(reasons)-1], "login_locked "))
}

func newLoginApp(h *authhandler.Handler) *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/auth/login", h.Login)
	return app
}

func postLogin(t *testing.T, f *migrationFixture, email, password string) *http.Response {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"email": email, "password": password})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
