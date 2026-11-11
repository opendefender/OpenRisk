// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// ---------------------------------------------------------------------------
// #1032 — regenerating backup codes through the real HTTP stack: RS256 auth
// middleware, MFA guard, handler, use case, GORM repository and the auth audit
// log. New codes take a current authenticator code, plus the password when the
// account has one.
// ---------------------------------------------------------------------------

const regeneratePath = "/api/v1/auth/mfa/backup-codes/regenerate"

// sessionThenEnrolWithSecret opens a session, then enrols the member with a
// real, verified secret and eight backup codes. It returns the token and the
// plain secret.
func (f *deferredFixture) sessionThenEnrolWithSecret(t *testing.T, m *domain.OrganizationMember) (token, plain string) {
	t.Helper()
	_, token = f.login(t, m.User.Email)
	require.NotEmpty(t, token)
	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, deferredTOTPKey)
	require.NoError(t, err)
	require.NoError(t, f.db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, SecretEncrypted: enc, IsVerified: true,
	}).Error)
	for i := 0; i < 8; i++ {
		require.NoError(t, f.db.Create(&domain.MFABackupCode{
			ID: uuid.New(), UserID: m.UserID, TenantID: m.OrganizationID, CodeHash: "old-" + uuid.NewString(),
		}).Error)
	}
	return token, plain
}

func (f *deferredFixture) codeHashes(t *testing.T, m *domain.OrganizationMember) []string {
	t.Helper()
	var hashes []string
	require.NoError(t, f.db.Model(&domain.MFABackupCode{}).
		Where("user_id = ? AND tenant_id = ?", m.UserID, m.OrganizationID).
		Pluck("code_hash", &hashes).Error)
	return hashes
}

func (f *deferredFixture) regenerateAudit(t *testing.T, m *domain.OrganizationMember) []domain.AuthAuditLog {
	t.Helper()
	var rows []domain.AuthAuditLog
	require.NoError(t, f.db.Where("user_id = ? AND action = ?", m.UserID, "mfa_backup_codes_regenerate").
		Order("created_at").Find(&rows).Error)
	return rows
}

func TestRegenerateBackupCodesE2E_Success(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.memberA)

	status, resp := f.do(t, http.MethodPost, regeneratePath, token,
		jsonBody{"password": deferredPassword, "code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status, "%v", resp)

	raw, ok := resp["backup_codes"].([]any)
	require.True(t, ok, "%v", resp)
	require.Len(t, raw, 8)

	hashes := f.codeHashes(t, f.memberA)
	require.Len(t, hashes, 8, "the old codes are gone, the new ones stored")
	for _, h := range hashes {
		assert.NotContains(t, h, "old-", "an old code survived")
	}
	// Every code returned is one that is stored, hashed.
	for _, c := range raw {
		matched := false
		for _, h := range hashes {
			if bcrypt.CompareHashAndPassword([]byte(h), []byte(c.(string))) == nil {
				matched = true
				break
			}
		}
		assert.True(t, matched, "returned code %v is not stored", c)
	}

	logs := f.regenerateAudit(t, f.memberA)
	require.Len(t, logs, 1)
	assert.True(t, logs[0].Success)
}

func TestRegenerateBackupCodesE2E_Unauthorized(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.memberA)

	cases := []struct {
		name string
		body jsonBody
		want string
	}{
		{"wrong password", jsonBody{"password": "guess-1234", "code": liveCode(t, plain)}, "wrong_password"},
		{"missing password", jsonBody{"code": liveCode(t, plain)}, "wrong_password"},
		{"missing code", jsonBody{"password": deferredPassword}, "wrong_code"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := f.do(t, http.MethodPost, regeneratePath, token, tc.body)
			assert.Equal(t, http.StatusUnauthorized, status, "%v", resp)
			assert.Equal(t, tc.want, resp["code"])
			assert.Nil(t, resp["backup_codes"])
			assert.Len(t, f.codeHashes(t, f.memberA), 8)
		})
	}
	for _, l := range f.regenerateAudit(t, f.memberA) {
		assert.False(t, l.Success)
		require.NotNil(t, l.FailureReason)
	}
	assert.Len(t, f.regenerateAudit(t, f.memberA), 3, "every refusal is audited")
}

func TestRegenerateBackupCodesE2E_ReplayedCodeIsRefused(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.memberA)
	code := liveCode(t, plain)

	status, resp := f.do(t, http.MethodPost, regeneratePath, token, jsonBody{"password": deferredPassword, "code": code})
	require.Equal(t, http.StatusOK, status, "%v", resp)
	first := f.codeHashes(t, f.memberA)

	status, resp = f.do(t, http.MethodPost, regeneratePath, token, jsonBody{"password": deferredPassword, "code": code})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "wrong_code", resp["code"])
	assert.ElementsMatch(t, first, f.codeHashes(t, f.memberA), "a replay changes nothing")
}

func TestRegenerateBackupCodesE2E_NotFound(t *testing.T) {
	f := newDeferredFixture(t)
	_, token := f.login(t, f.memberA.User.Email)

	status, resp := f.do(t, http.MethodPost, regeneratePath, token, jsonBody{"password": deferredPassword, "code": "123456"})
	assert.Equal(t, http.StatusNotFound, status, "%v", resp)
	assert.Equal(t, "not_enrolled", resp["code"])
}

func TestRegenerateBackupCodesE2E_AccountWithoutPasswordNeedsOnlyTheCode(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.ssoSessionThenEnrol(t, f.memberA)

	status, resp := f.do(t, http.MethodPost, regeneratePath, token, jsonBody{"code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status, "%v", resp)
	assert.Len(t, f.codeHashes(t, f.memberA), 8)
}

// The point of #1032: a role that may not turn MFA off can still get new codes.
func TestRegenerateBackupCodesE2E_PrivilegedRoleCanRegenerate(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.adminA)
	f.resolver.Invalidate(f.adminA.UserID, f.tenantA)

	status, resp := f.do(t, http.MethodPost, regeneratePath, token,
		jsonBody{"password": deferredPassword, "code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status, "%v", resp)
}

func TestRegenerateBackupCodesE2E_AnotherTenantsCodesAreOutOfReach(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.memberA)
	f.enrol(t, f.adminB)
	before := f.codeHashes(t, f.adminB)

	status, _ := f.do(t, http.MethodPost, regeneratePath, token,
		jsonBody{"password": deferredPassword, "code": liveCode(t, plain)})
	require.Equal(t, http.StatusOK, status)

	assert.ElementsMatch(t, before, f.codeHashes(t, f.adminB))
}

// Regenerate and disable share one per-account budget: both take the password.
func TestRegenerateBackupCodesE2E_SharesTheDisableBudget(t *testing.T) {
	f := newDeferredFixture(t)
	token, plain := f.sessionThenEnrolWithSecret(t, f.memberA)

	for i := 0; i < 3; i++ {
		status, _ := f.do(t, http.MethodPost, disablePath, token, jsonBody{"password": "guess-" + uuid.NewString()})
		require.Equal(t, http.StatusUnauthorized, status)
	}
	for i := 0; i < 2; i++ {
		status, _ := f.do(t, http.MethodPost, regeneratePath, token, jsonBody{"password": "guess-" + uuid.NewString(), "code": "000000"})
		require.Equal(t, http.StatusUnauthorized, status)
	}
	status, resp := f.do(t, http.MethodPost, regeneratePath, token,
		jsonBody{"password": deferredPassword, "code": liveCode(t, plain)})
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "too_many_attempts", resp["code"])
	assert.Len(t, f.codeHashes(t, f.memberA), 8)
	for _, h := range f.codeHashes(t, f.memberA) {
		assert.Contains(t, h, "old-")
	}
}
