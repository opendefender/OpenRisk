// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// #1392 — the in-session MFA lock through the real HTTP stack. The D-061
// budget (5 tries per 15 minutes) stops ten failures from arriving through HTTP
// in one test, so nine are seeded in the store under the use case's own key.
// ---------------------------------------------------------------------------

func (f *deferredFixture) seedInSessionFailures(t *testing.T, m *domain.OrganizationMember, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := f.inSession.Increment(context.Background(), "insession:fail:"+m.UserID.String(), appauth.MFAInSessionFailureWindow)
		require.NoError(t, err)
	}
}

// post is f.do with the response headers kept.
func (f *deferredFixture) post(t *testing.T, path, token string, body jsonBody) (int, http.Header, jsonBody) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	out := jsonBody{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, resp.Header, out
}

func (f *deferredFixture) auditReasons(t *testing.T, m *domain.OrganizationMember, action string) []string {
	t.Helper()
	var rows []domain.AuthAuditLog
	require.NoError(t, f.db.Where("user_id = ? AND action = ?", m.UserID, action).Order("created_at").Find(&rows).Error)
	reasons := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.FailureReason != nil {
			reasons = append(reasons, *r.FailureReason)
		} else {
			reasons = append(reasons, "success")
		}
	}
	return reasons
}

func assertLocked(t *testing.T, status int, h http.Header, body jsonBody) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, status, "%v", body)
	assert.Equal(t, "mfa_locked", body["code"])
	secs, err := strconv.Atoi(h.Get("Retry-After"))
	require.NoError(t, err, "Retry-After %q", h.Get("Retry-After"))
	assert.Greater(t, secs, int((23 * time.Hour).Seconds()))
	assert.LessOrEqual(t, secs, int((24*time.Hour).Seconds())+1)
}

// Criteria 2 and 3 over HTTP: the 10th failure locks both endpoints, the
// correct code is then refused, the lock is audited once and each refusal too.
func TestMFAInSessionLockE2E_TenthFailureLocksBothEndpoints(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.ssoSessionThenEnrol(t, f.memberA)
	f.seedInSessionFailures(t, f.memberA, appauth.MFAInSessionMaxFailures-1)

	status, h, body := f.post(t, disablePath, token, jsonBody{"code": "00000x"})
	assertLocked(t, status, h, body)

	status, h, body = f.post(t, disablePath, token, jsonBody{"code": liveCode(t, plain)})
	assertLocked(t, status, h, body)
	status, h, body = f.post(t, regeneratePath, token, jsonBody{"code": liveCode(t, plain)})
	assertLocked(t, status, h, body)

	var secrets int64
	require.NoError(t, f.db.Model(&domain.MFASecret{}).Where("user_id = ?", f.memberA.UserID).Count(&secrets).Error)
	assert.Equal(t, int64(1), secrets, "nothing changes under the lock")

	assert.Equal(t, []string{"mfa_lock_set", "mfa_locked"}, f.auditReasons(t, f.memberA, "mfa_disable"))
	assert.Equal(t, []string{"mfa_locked"}, f.auditReasons(t, f.memberA, "mfa_backup_codes_regenerate"))
}

// The lock is checked before the D-061 budget: past five requests a locked
// account is still told it is locked, for 24 h, not "a few minutes".
func TestMFAInSessionLockE2E_LockOutranksTheShortBudget(t *testing.T) {
	f := newDeferredFixture(t)
	token, _ := f.ssoSessionThenEnrol(t, f.memberA)
	f.seedInSessionFailures(t, f.memberA, appauth.MFAInSessionMaxFailures-1)

	for i := 0; i < 8; i++ {
		path := disablePath
		if i%2 == 1 {
			path = regeneratePath
		}
		status, h, body := f.post(t, path, token, jsonBody{"code": "00000x"})
		assertLocked(t, status, h, body)
	}
}

// Below the limit a wrong code is just a wrong code.
func TestMFAInSessionLockE2E_BelowTheLimitIsAPlainFailure(t *testing.T) {
	f := newDeferredFixture(t)
	token, _ := f.ssoSessionThenEnrol(t, f.memberA)
	f.seedInSessionFailures(t, f.memberA, appauth.MFAInSessionMaxFailures-2)

	status, _, body := f.post(t, regeneratePath, token, jsonBody{"code": "00000x"})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "wrong_code", body["code"])
}
