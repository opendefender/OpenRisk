// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// This exercises organization switching across the full stack — handler →
// use case → GormUserRepository → sqlite → TokenManager (org resolver) — and
// proves the cross-tenant boundary: a user cannot switch into an org they are
// not an active member of, no matter what org id they present in the body.

func setupSwitchDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Unique named in-memory DB per test (isolated), with a single connection so
	// every query sees the same schema.
	dsn := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	// Hand-written DDL: the domain models carry Postgres gen_random_uuid() defaults
	// that sqlite cannot evaluate. Ids are assigned in Go below.
	require.NoError(t, db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY, email TEXT, username TEXT, full_name TEXT, password TEXT,
		is_active INTEGER, default_org_id TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE organizations (
		id TEXT PRIMARY KEY, name TEXT, slug TEXT, logo_url TEXT, is_active INTEGER,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (
		id TEXT PRIMARY KEY, organization_id TEXT, user_id TEXT, role TEXT, business_role TEXT,
		profile_id TEXT, is_active INTEGER, joined_at DATETIME, created_at DATETIME, updated_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE refresh_tokens (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL, family_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE, device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
		expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
		created_at DATETIME, updated_at DATETIME)`).Error)
	return db
}

// seedMember inserts an org and a membership for a user.
func seedMember(t *testing.T, db *gorm.DB, userID, orgID uuid.UUID, name, slug string, active bool) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, is_active) VALUES (?,?,?,1)`,
		orgID.String(), name, slug).Error)
	require.NoError(t, db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role, is_active) VALUES (?,?,?,?,?)`,
		uuid.NewString(), orgID.String(), userID.String(), "admin", boolToInt(active)).Error)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func buildSwitchApp(t *testing.T, db *gorm.DB, actingUser uuid.UUID) *fiber.App {
	t.Helper()
	userRepo := repository.NewGormUserRepository(db)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tm := coreauth.NewTokenManager(db, &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey})
	// Same org resolver contract as production: active membership required.
	tm.SetOrgSessionResolver(func(ctx context.Context, uid, orgID uuid.UUID) (*coreauth.SessionClaims, error) {
		m, err := userRepo.GetOrganizationMember(ctx, uid, orgID)
		if err != nil {
			return nil, err
		}
		if m == nil || !m.IsActive {
			return nil, domain.NewForbiddenError("not an active member")
		}
		return &coreauth.SessionClaims{TenantID: orgID, OrgRoles: map[uuid.UUID]string{orgID: string(m.Role)}, Permissions: m.EffectivePermissions()}, nil
	})

	h := authhandler.NewSwitchHandler(
		appauth.NewListOrganizationsUseCase(userRepo),
		appauth.NewSwitchOrganizationUseCase(userRepo, tm),
		nil,
	)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		middleware.SetContext(c, &middleware.RequestContext{UserID: actingUser})
		return c.Next()
	})
	app.Get("/auth/organizations", h.ListOrganizations)
	app.Post("/auth/switch-org", h.SwitchOrganization)
	return app
}

func TestE2E_Switch_ListsOnlyOwnActiveOrgs(t *testing.T) {
	db := setupSwitchDB(t)
	user := uuid.New()
	orgA, orgB, orgC := uuid.New(), uuid.New(), uuid.New()
	seedMember(t, db, user, orgA, "Alpha", "alpha", true)
	seedMember(t, db, user, orgB, "Bravo", "bravo", true)
	seedMember(t, db, user, orgC, "Charlie", "charlie", false) // deactivated → not listed
	// An org belonging to somebody else must never appear.
	seedMember(t, db, uuid.New(), uuid.New(), "Foreign", "foreign", true)

	app := buildSwitchApp(t, db, user)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/organizations", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Organizations []struct {
			OrganizationID string `json:"organization_id"`
			Name           string `json:"name"`
		} `json:"organizations"`
		Total int `json:"total"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, 2, body.Total, "only active memberships of the acting user")
	names := map[string]bool{}
	for _, o := range body.Organizations {
		names[o.Name] = true
	}
	require.True(t, names["Alpha"])
	require.True(t, names["Bravo"])
	require.False(t, names["Charlie"], "deactivated membership must not be switchable")
	require.False(t, names["Foreign"], "another user's org must never leak")
}

func TestE2E_Switch_ToMemberOrg_Succeeds(t *testing.T) {
	db := setupSwitchDB(t)
	user := uuid.New()
	orgA, orgB := uuid.New(), uuid.New()
	seedMember(t, db, user, orgA, "Alpha", "alpha", true)
	seedMember(t, db, user, orgB, "Bravo", "bravo", true)

	app := buildSwitchApp(t, db, user)
	resp, err := app.Test(postJSON("/auth/switch-org", `{"organization_id":"`+orgB.String()+`"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		TokenPair *coreauth.TokenPair `json:"token_pair"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotNil(t, body.TokenPair)
	require.NotEmpty(t, body.TokenPair.AccessToken)

	// The minted access token must carry orgB as the tenant — proving the switch
	// actually re-scoped the session on the server, not just echoed a body value.
	require.Equal(t, orgB.String(), tenantFromJWT(t, body.TokenPair.AccessToken))
}

// tenantFromJWT decodes (without verifying) the tenant_id claim of an RS256 JWT.
// Verification is covered by pkg/auth; here we only assert the switch re-scoped
// the session to the target org.
func tenantFromJWT(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		TenantID string `json:"tenant_id"`
	}
	require.NoError(t, json.Unmarshal(raw, &claims))
	return claims.TenantID
}

func TestE2E_Switch_ToNonMemberOrg_Forbidden(t *testing.T) {
	db := setupSwitchDB(t)
	user := uuid.New()
	orgA := uuid.New()
	seedMember(t, db, user, orgA, "Alpha", "alpha", true)
	// orgB exists but the user is NOT a member.
	orgB := uuid.New()
	seedMember(t, db, uuid.New(), orgB, "Bravo", "bravo", true)

	app := buildSwitchApp(t, db, user)
	resp, err := app.Test(postJSON("/auth/switch-org", `{"organization_id":"`+orgB.String()+`"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"a user must not switch into an org they are not a member of, whatever id they present")
}

func TestE2E_Switch_ToDeactivatedMembership_Forbidden(t *testing.T) {
	db := setupSwitchDB(t)
	user := uuid.New()
	orgA, orgB := uuid.New(), uuid.New()
	seedMember(t, db, user, orgA, "Alpha", "alpha", true)
	seedMember(t, db, user, orgB, "Bravo", "bravo", false) // membership deactivated

	app := buildSwitchApp(t, db, user)
	resp, err := app.Test(postJSON("/auth/switch-org", `{"organization_id":"`+orgB.String()+`"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"a deactivated membership must be rejected identically to a non-membership")
}

func postJSON(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
