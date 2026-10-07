// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// #714 — POST /auth/mfa/setup through the handler, the use case and the GORM
// repository: started over it answers 200 each time, and a store failure
// answers 500 with no database text.

const setupPath = "/api/v1/auth/mfa/setup"

func newSetupApp(t *testing.T) (*deferredFixture, *fiber.App) {
	t.Helper()
	f := newDeferredFixture(t)
	// Production's unique user_id (domain.MFASecret), which the upsert targets.
	require.NoError(t, f.db.Exec(`CREATE UNIQUE INDEX idx_mfa_secrets_user_id ON mfa_secrets (user_id)`).Error)

	setupUC := appauth.NewSetupMFAUseCase(repository.NewGormMFARepository(f.db), deferredTOTPKey)
	h := authhandler.NewMFAHandler(setupUC, nil, nil, nil, nil, repository.NewGormUserRepository(f.db), nil)

	app := fiber.New()
	// What the session middleware leaves behind for an authenticated caller.
	app.Post(setupPath, func(c *fiber.Ctx) error {
		c.Locals("user_id", f.adminA.UserID.String())
		c.Locals("tenant_id", f.tenantA.String())
		return c.Next()
	}, h.Setup)
	return f, app
}

func postSetup(t *testing.T, app *fiber.App) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, setupPath, nil)
	req.Header.Set("Accept-Language", "en")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestMFASetupHTTP_StartedOverAnswers200(t *testing.T) {
	f, app := newSetupApp(t)

	var secrets []string
	for range 3 {
		status, body := postSetup(t, app)
		require.Equal(t, http.StatusOK, status, body)
		var out appauth.SetupMFAOutput
		require.NoError(t, json.Unmarshal([]byte(body), &out))
		secrets = append(secrets, out.Secret)
	}
	assert.NotEqual(t, secrets[0], secrets[2], "each attempt gets a new key")

	var rows, codes int64
	require.NoError(t, f.db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", f.adminA.UserID).Count(&rows).Error)
	require.NoError(t, f.db.Model(&domain.MFABackupCode{}).Where("user_id = ? AND tenant_id = ?", f.adminA.UserID, f.tenantA).Count(&codes).Error)
	assert.EqualValues(t, 1, rows)
	assert.EqualValues(t, 8, codes, "only the latest set of codes")
}

func TestMFASetupHTTP_StoreFailureIs500WithoutItsText(t *testing.T) {
	f, app := newSetupApp(t)
	const dbText = `pq: insert on table "mfa_backup_codes" violates constraint`
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("test:fail_codes", func(tx *gorm.DB) {
		if tx.Statement.Table == "mfa_backup_codes" {
			_ = tx.AddError(errors.New(dbText))
		}
	}))

	status, body := postSetup(t, app)

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.JSONEq(t, `{"error":"Something went wrong. Please try again shortly."}`, body)
	for _, leak := range []string{"mfa_", "constraint", "pq:", "failed to"} {
		assert.False(t, strings.Contains(body, leak), "the body must not carry %q: %s", leak, body)
	}
	var rows int64
	require.NoError(t, f.db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", f.adminA.UserID).Count(&rows).Error)
	assert.Zero(t, rows, "no secret without its backup codes")
}
