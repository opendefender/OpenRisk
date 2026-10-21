// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

// #686 over HTTP: a sign-up for an address that already has an account,
// through the real register route, the real GORM user repository and the
// audit trail.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
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

type countingDuplicateMailer struct {
	mu sync.Mutex
	to []string
}

func (m *countingDuplicateMailer) SendDuplicateSignup(_ context.Context, to, _, _, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.to = append(m.to, to)
	return nil
}

func TestRegisterDuplicate_ConflictThen429AndEveryAttemptIsAudited(t *testing.T) {
	hasher := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{Time: 1, Memory: 1024, Threads: 1})
	f := newMigrationFixture(t, "unused")

	require.NoError(t, f.db.Exec(`CREATE TABLE auth_audit_logs (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(f.db, "auth_audit_logs", &domain.AuthAuditLog{}))
	audit := coreauth.NewAuditService(repository.NewGormAuthAuditLogRepository(f.db))

	mailer := &countingDuplicateMailer{}
	registerUC := appauth.NewRegisterUseCase(repository.NewGormUserRepository(f.db),
		repository.NewGormOrganizationRepository(f.db), nil, hasher).
		WithAccounts(repository.NewGormRegistrationRepository(f.db)).
		WithDuplicateGuard(authmfa.NewMemoryAttemptStore(), mailer, "http://app.test")
	app := fiber.New()
	app.Post("/api/v1/auth/register", authhandler.NewHandler(nil, registerUC, nil, nil, hasher, audit).Register)

	const password = "Ancre-Vitrail7-Cobalt"
	post := func(email string) (*http.Response, []byte) {
		t.Helper()
		raw, err := json.Marshal(map[string]string{
			"email": email, "password": password, "full_name": "Someone Else", "company_name": "Elsewhere",
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "probe/1.0")
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp, body
	}

	for i := 1; i <= appauth.RegisterMaxDuplicatesPerAddress; i++ {
		email := f.user.Email
		if i%2 == 0 {
			email = " " + strings.ToUpper(f.user.Email) // the normalised address is what counts
		}
		resp, raw := post(email)
		require.Equal(t, http.StatusConflict, resp.StatusCode, "attempt %d", i)
		var body map[string]any
		require.NoError(t, json.Unmarshal(raw, &body))
		assert.Equal(t, "EMAIL_ALREADY_REGISTERED", body["code"])
		// Nothing else about the account: no id, no name, no organisation.
		assert.ElementsMatch(t, []string{"error", "code"}, keys(body))
		assert.NotContains(t, string(raw), f.user.FullName)
	}

	resp, raw := post(f.user.Email)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	wait, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	require.NoError(t, err)
	assert.InDelta(t, appauth.RegisterDuplicateLockDuration.Seconds(), float64(wait), 2)
	var limited struct {
		Code       string `json:"code"`
		RetryAfter int    `json:"retry_after"`
	}
	require.NoError(t, json.Unmarshal(raw, &limited))
	assert.Equal(t, "REGISTER_RATE_LIMITED", limited.Code)
	assert.Equal(t, wait, limited.RetryAfter)

	// One alert for six attempts, to the owner.
	assert.Equal(t, []string{f.user.Email}, mailer.to)

	var rows []domain.AuthAuditLog
	require.NoError(t, f.db.Where("action = ?", string(coreauth.AuditActionRegister)).
		Order("created_at").Find(&rows).Error)
	require.Len(t, rows, appauth.RegisterMaxDuplicatesPerAddress+1)
	hashed := "email_sha256=" + domain.HashEmailForReset(f.user.Email)
	for _, r := range rows {
		assert.False(t, r.Success)
		assert.Equal(t, "probe/1.0", r.UserAgent)
		assert.NotEmpty(t, r.IP)
		require.NotNil(t, r.FailureReason)
		assert.Contains(t, *r.FailureReason, hashed)
		assert.NotContains(t, strings.ToLower(*r.FailureReason), f.user.Email, "never the address in clear")
	}
	assert.True(t, strings.HasPrefix(*rows[0].FailureReason, "email_already_registered "))
	assert.True(t, strings.HasPrefix(*rows[len(rows)-1].FailureReason, "register_duplicate_limited "))
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
