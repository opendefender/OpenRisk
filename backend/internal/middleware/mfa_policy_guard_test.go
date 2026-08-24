// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
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

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeMFAStatus is an MFAStatusProvider returning a fixed decision (or error),
// so the guard is exercised over the real HTTP stack without a database.
type fakeMFAStatus struct {
	decision domain.MFADecision
	err      error
	calls    int
}

func (f *fakeMFAStatus) Resolve(context.Context, uuid.UUID, uuid.UUID, string) (domain.MFADecision, error) {
	f.calls++
	return f.decision, f.err
}

// guardApp mounts the guard behind a middleware that publishes an identity, the
// way the real auth gate does.
func guardApp(provider MFAStatusProvider, authenticated bool) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if authenticated {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: uuid.New()})
		}
		return c.Next()
	})
	app.Use(MFAPolicyGuard(provider))

	reached := func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"reached": true}) }
	app.Get("/api/v1/risks", reached)
	app.Post("/api/v1/risks", reached)
	app.Get("/api/v1/auth/me", reached)
	app.Post("/api/v1/auth/mfa/setup", reached)
	app.Post("/api/v1/auth/mfa/verify", reached)
	app.Post("/api/v1/auth/logout", reached)
	return app
}

func do(t *testing.T, app *fiber.App, method, path string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil), -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	return resp.StatusCode, body
}

func requiredDecision() domain.MFADecision {
	deadline := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	return domain.MFADecision{
		State: domain.MFAStateRequired, Required: true, Privileged: true,
		Deadline: &deadline, GraceDays: 7,
	}
}

func TestMFAPolicyGuard_BlocksProtectedRoutesOnceEnrolmentIsMandatory(t *testing.T) {
	// The API-level bypass test: no frontend involved, just a session whose
	// grace period has expired hitting a business route directly.
	app := guardApp(&fakeMFAStatus{decision: requiredDecision()}, true)

	status, body := do(t, app, http.MethodGet, "/api/v1/risks")
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Equal(t, MFAEnrollmentRequiredCode, body["code"])
	require.NotNil(t, body["mfa"], "the client is told why, and by when it was due")

	status, _ = do(t, app, http.MethodPost, "/api/v1/risks")
	assert.Equal(t, fiber.StatusForbidden, status, "writes are blocked too, not just reads")
}

func TestMFAPolicyGuard_LetsGraceAndConfiguredThrough(t *testing.T) {
	for _, d := range []domain.MFADecision{
		{State: domain.MFAStateRecommended},
		{State: domain.MFAStateGraceActive, Privileged: true, GraceActive: true},
		{State: domain.MFAStateGraceExpiring, Privileged: true, GraceActive: true},
		{State: domain.MFAStateConfigured, Configured: true},
	} {
		app := guardApp(&fakeMFAStatus{decision: d}, true)
		status, body := do(t, app, http.MethodGet, "/api/v1/risks")
		assert.Equal(t, fiber.StatusOK, status, "state %s must not block", d.State)
		assert.Equal(t, true, body["reached"])
	}
}

func TestMFAPolicyGuard_LeavesTheRemedyReachable(t *testing.T) {
	// A requirement you cannot satisfy is a lockout, not a control: a blocked
	// admin must still be able to enrol, read why, and sign out.
	app := guardApp(&fakeMFAStatus{decision: requiredDecision()}, true)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/auth/mfa/setup"},
		{http.MethodPost, "/api/v1/auth/mfa/verify"},
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/logout"},
	} {
		status, _ := do(t, app, tc.method, tc.path)
		assert.Equal(t, fiber.StatusOK, status, "%s must stay reachable", tc.path)
	}
}

func TestMFAPolicyGuard_FailsClosedWhenTheStatusCannotBeResolved(t *testing.T) {
	// An unverifiable guard blocks. A database problem must not be a way to be
	// treated as compliant — but the code says "could not check", not "enrol",
	// because sending someone to scan a QR code that will not help is worse
	// than an honest error.
	app := guardApp(&fakeMFAStatus{err: errors.New("database down")}, true)

	status, body := do(t, app, http.MethodGet, "/api/v1/risks")
	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, MFAStatusUnavailableCode, body["code"])
	assert.Nil(t, body["reached"])
}

func TestMFAPolicyGuard_DefersToTheAuthGateForAnonymousRequests(t *testing.T) {
	// No identity published: refusing here would turn every 401 into a confusing
	// MFA message. The auth middleware has already refused, or will.
	provider := &fakeMFAStatus{decision: requiredDecision()}
	app := guardApp(provider, false)

	status, _ := do(t, app, http.MethodGet, "/api/v1/risks")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 0, provider.calls, "no identity, nothing to resolve")
}

func TestMFAPolicyGuard_UnwiredIsANoOp(t *testing.T) {
	// A deployment that has not enabled MFA at all is unaffected.
	app := guardApp(nil, true)
	status, _ := do(t, app, http.MethodGet, "/api/v1/risks")
	assert.Equal(t, fiber.StatusOK, status)
}

func TestMFAPolicyGuard_ExemptListIsSuffixMatchedAndShort(t *testing.T) {
	// Pinning the list: every entry that is not the remedy or the way out is a
	// route an account with no second factor can still use.
	assert.ElementsMatch(t, []string{
		"/auth/mfa/setup",
		"/auth/mfa/verify",
		"/auth/mfa/challenge",
		"/auth/me",
		"/auth/logout",
	}, mfaGuardExemptSuffixes)

	assert.True(t, isMFAGuardExempt("/api/v1/auth/me"))
	assert.False(t, isMFAGuardExempt("/api/v1/auth/mefa"))
	assert.False(t, isMFAGuardExempt("/api/v1/risks"))
	assert.False(t, isMFAGuardExempt("/api/v1/auth/pat"),
		"minting API tokens is exactly what a privileged account must not do without MFA")
}
