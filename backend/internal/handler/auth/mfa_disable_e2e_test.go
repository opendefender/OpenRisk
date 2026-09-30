// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// ---------------------------------------------------------------------------
// #754 — disabling MFA through the real HTTP stack: RS256 auth middleware, MFA
// guard, audit middleware, handler, use case and GORM repository. A live
// session is not enough to remove the second factor.
// ---------------------------------------------------------------------------

type capturedTrail struct {
	mu     sync.Mutex
	events []*domain.AuditEvent
}

func (c *capturedTrail) Append(_ context.Context, e *domain.AuditEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func (c *capturedTrail) pathCount(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, e := range c.events {
		if e.Path == path {
			n++
		}
	}
	return n
}

const disablePath = "/api/v1/auth/mfa/disable"

// enrol gives a member a verified secret and backup codes, as a completed
// enrolment would.
func (f *deferredFixture) enrol(t *testing.T, m *domain.OrganizationMember) {
	t.Helper()
	require.NoError(t, f.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, SecretEncrypted: "x", IsVerified: true,
	}).Error)
	for i := 0; i < 8; i++ {
		require.NoError(t, f.db.Create(&domain.MFABackupCode{
			ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, CodeHash: uuid.NewString(),
		}).Error)
	}
}

func (f *deferredFixture) factorRows(t *testing.T, m *domain.OrganizationMember) (secrets, codes int64) {
	t.Helper()
	require.NoError(t, f.db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", m.UserID).Count(&secrets).Error)
	require.NoError(t, f.db.Model(&domain.MFABackupCode{}).Where("user_id = ?", m.UserID).Count(&codes).Error)
	return secrets, codes
}

// A session opened before enrolment, so the login itself asks for no code.
func (f *deferredFixture) sessionThenEnrol(t *testing.T, m *domain.OrganizationMember) string {
	t.Helper()
	_, token := f.login(t, m.User.Email)
	require.NotEmpty(t, token)
	f.enrol(t, m)
	return token
}

func TestMFADisableE2E_LiveSessionWithoutPasswordCannotDisable(t *testing.T) {
	f := newDeferredFixture(t)
	token := f.sessionThenEnrol(t, f.memberA)

	for name, body := range map[string]any{
		"no body":        nil,
		"empty password": jsonBody{"password": ""},
		"wrong password": jsonBody{"password": "guess-1234"},
	} {
		t.Run(name, func(t *testing.T) {
			status, resp := f.do(t, http.MethodPost, disablePath, token, body)
			if body == nil {
				// No JSON at all is refused before the password is even read.
				assert.Equal(t, http.StatusBadRequest, status)
			} else {
				assert.Equal(t, http.StatusUnauthorized, status)
				assert.Equal(t, "wrong_password", resp["code"])
				// Must not read as a dead session to the SPA.
				assert.NotContains(t, []any{"TOKEN_EXPIRED", "TOKEN_REVOKED", "TOKEN_INVALID", "UNAUTHORIZED"}, resp["code"])
			}
			secrets, codes := f.factorRows(t, f.memberA)
			assert.EqualValues(t, 1, secrets, "the secret must survive")
			assert.EqualValues(t, 8, codes, "the backup codes must survive")
		})
	}
	assert.Zero(t, f.trail.pathCount(disablePath), "a refused disable changes nothing and is not chained")
}

func TestMFADisableE2E_CorrectPasswordDisablesAndIsChained(t *testing.T) {
	f := newDeferredFixture(t)
	token := f.sessionThenEnrol(t, f.memberA)

	status, resp := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": deferredPassword})
	require.Equal(t, http.StatusOK, status, "%v", resp)

	secrets, codes := f.factorRows(t, f.memberA)
	assert.Zero(t, secrets)
	assert.Zero(t, codes)
	assert.Equal(t, 1, f.trail.pathCount(disablePath), "the deactivation is on the chained trail")

	// Disabling twice finds nothing to disable.
	status, resp = f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": deferredPassword})
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "not_enrolled", resp["code"])
}

// ssoSessionThenEnrol opens a session, then turns the member into an
// identity-provider account (no local password) enrolled with a real secret.
func (f *deferredFixture) ssoSessionThenEnrol(t *testing.T, m *domain.OrganizationMember) (token, plain string) {
	t.Helper()
	_, token = f.login(t, m.User.Email)
	require.NotEmpty(t, token)
	require.NoError(t, f.db.Model(&domain.User{}).Where("id = ?", m.UserID).Update("password", "").Error)
	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, deferredTOTPKey)
	require.NoError(t, err)
	require.NoError(t, f.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, SecretEncrypted: enc, IsVerified: true,
	}).Error)
	return token, plain
}

func TestMFADisableE2E_AccountWithoutPasswordConfirmsWithACode(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.ssoSessionThenEnrol(t, f.memberA)

	status, me := f.do(t, http.MethodGet, "/api/v1/auth/me", token, nil)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, me["has_password"], "the dialog must know to ask for a code")

	status, resp := f.do(t, http.MethodPost, disablePath, token, jsonBody{"code": "000000"})
	if status != http.StatusOK { // "000000" could, once in a million, be the live code
		assert.Equal(t, http.StatusUnauthorized, status)
		assert.Equal(t, "wrong_code", resp["code"])
		secrets, _ := f.factorRows(t, f.memberA)
		assert.EqualValues(t, 1, secrets, "a wrong code removes nothing")
	}

	code, err := totp.GenerateCode(plain, time.Now())
	require.NoError(t, err)
	status, resp = f.do(t, http.MethodPost, disablePath, token, jsonBody{"code": code})
	require.Equal(t, http.StatusOK, status, "%v", resp)
	secrets, codes := f.factorRows(t, f.memberA)
	assert.Zero(t, secrets)
	assert.Zero(t, codes)
}

func TestMFADisableE2E_MeSaysAPasswordAccountHasAPassword(t *testing.T) {
	f := newDeferredFixture(t)
	token := f.sessionThenEnrol(t, f.memberA)

	status, me := f.do(t, http.MethodGet, "/api/v1/auth/me", token, nil)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, me["has_password"])
}

func TestMFADisableE2E_PrivilegedRoleIsRefused(t *testing.T) {
	f := newDeferredFixture(t)
	// Inside the grace window the admin holds a session without a code.
	token := f.sessionThenEnrol(t, f.adminA)
	f.resolver.Invalidate(f.adminA.UserID, f.tenantA)

	status, resp := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": deferredPassword})
	assert.Equal(t, http.StatusForbidden, status, "%v", resp)
	assert.Equal(t, "mfa_required_by_role", resp["code"])

	secrets, codes := f.factorRows(t, f.adminA)
	assert.EqualValues(t, 1, secrets)
	assert.EqualValues(t, 8, codes)
}

func TestMFADisableE2E_AnotherTenantsFactorIsOutOfReach(t *testing.T) {
	f := newDeferredFixture(t)
	token := f.sessionThenEnrol(t, f.memberA)
	f.enrol(t, f.adminB)

	status, _ := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": deferredPassword})
	require.Equal(t, http.StatusOK, status)

	secrets, codes := f.factorRows(t, f.adminB)
	assert.EqualValues(t, 1, secrets)
	assert.EqualValues(t, 8, codes)
}

func TestMFADisableE2E_AttemptsAreLimitedPerAccount(t *testing.T) {
	f := newDeferredFixture(t)
	token := f.sessionThenEnrol(t, f.memberA)

	for i := 0; i < 5; i++ {
		status, _ := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": "guess-" + uuid.NewString()})
		require.Equal(t, http.StatusUnauthorized, status, "attempt %d", i+1)
	}
	// The sixth try is refused even with the right password: the budget is the
	// account's, and a guesser who finally hits it must still wait.
	status, resp := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": deferredPassword})
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "too_many_attempts", resp["code"])

	secrets, codes := f.factorRows(t, f.memberA)
	assert.EqualValues(t, 1, secrets)
	assert.EqualValues(t, 8, codes)

	// Another account's budget is untouched.
	other := f.seedMember(t, f.tenantA, "other@a.io", domain.RoleUser, "", f.now)
	otherToken := f.sessionThenEnrol(t, other)
	status, _ = f.do(t, http.MethodPost, disablePath, otherToken, jsonBody{"password": deferredPassword})
	assert.Equal(t, http.StatusOK, status)
}
