// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

// End-to-end proof of the password hashing rules (#484, D-048), through the
// real login route, the real GORM user repository and the real hasher:
//   - an account still holding the SHA-256 digest the first release wrote is
//     refused, even with the right password, and stays counted until it resets;
//   - an Argon2id hash written at a lower cost is rewritten at today's cost on
//     the next successful sign-in.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
	"github.com/opendefender/openrisk/pkg/monitoring"
)

const migrationPassword = "Ancre-Vitrail7-Cobalt"

var migrationNow = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

type migrationFixture struct {
	db     *gorm.DB
	app    *fiber.App
	user   *domain.User
	census *repository.PasswordHashCensus
}

// newMigrationFixture seeds one account with the given stored hash and mounts
// the login route over a hasher writing Argon2id at t=3.
func newMigrationFixture(t *testing.T, stored string) *migrationFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	for _, m := range []struct {
		table string
		model any
	}{
		{"users", &domain.User{}},
		{"organization_members", &domain.OrganizationMember{}},
		{"organizations", &domain.Organization{}},
	} {
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}
	require.NoError(t, db.Exec(`
		CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, tenant_id TEXT NOT NULL,
			family_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			device_fingerprint TEXT, ip_address TEXT, user_agent TEXT,
			expires_at DATETIME NOT NULL, rotated_at DATETIME, last_used_at DATETIME,
			created_at DATETIME, updated_at DATETIME
		)`).Error)

	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.Organization{
		ID: orgID, Name: "Banque Atlantique", Slug: "banque-atlantique", IsActive: true, Plan: domain.PlanStarter,
	}).Error)

	user := &domain.User{
		ID: uuid.New(), Email: "legacy@a.io", Username: "legacy",
		FullName: "Legacy Account", IsActive: true,
		Password: stored, DefaultOrgID: &orgID,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{
		ID: uuid.New(), UserID: user.ID, OrganizationID: orgID, Role: domain.RoleUser,
		IsActive: true, Status: domain.MembershipActive, JoinedAt: migrationNow,
	}).Error)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &authpkg.RSAKeys{PrivateKey: priv, PublicKey: &priv.PublicKey}

	// The product's real hasher, at its floor cost so the suite stays fast. The
	// cost is irrelevant to what is proven here; the algorithm is not.
	hasher := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 3, Memory: 19456, Threads: 1,
	})
	userRepo := repository.NewGormUserRepository(db)
	loginUC := appauth.NewLoginUseCase(userRepo, coreauth.NewTokenManager(db, keys), hasher).
		WithClock(func() time.Time { return migrationNow })

	app := fiber.New()
	app.Post("/api/v1/auth/login", authhandler.NewHandler(loginUC, nil, nil, nil, hasher, nil).Login)

	return &migrationFixture{
		db: db, app: app, user: user,
		census: repository.NewPasswordHashCensus(db, coreauth.HashAlgorithm),
	}
}

func (f *migrationFixture) login(t *testing.T, password string) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"email": f.user.Email, "password": password})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return resp.StatusCode, out
}

func (f *migrationFixture) storedHash(t *testing.T) string {
	t.Helper()
	var u domain.User
	require.NoError(t, f.db.First(&u, "id = ?", f.user.ID).Error)
	return u.Password
}

func legacyDigest(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

// D-048: no transparent migration. The right password on a SHA-256 account
// gets the same 401 as a wrong one, nothing is rewritten, and the account stays
// in the census until it resets its password.
func TestPasswordMigration_LegacyHashIsRefused(t *testing.T) {
	f := newMigrationFixture(t, legacyDigest(migrationPassword))
	legacy := f.user.Password

	status, body := f.login(t, migrationPassword)
	assert.Equal(t, http.StatusUnauthorized, status, "a legacy digest must not buy a session: %v", body)
	assert.Nil(t, body["token_pair"])
	assert.Equal(t, legacy, f.storedHash(t), "nothing is rewritten for a hash that cannot verify")

	counts, err := f.census.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["active"][coreauth.AlgorithmLegacySHA256], "the account still has to reset")
}

func TestPasswordMigration_LowerCostArgon2idIsUpgradedOnSignIn(t *testing.T) {
	older := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 2, Memory: 19456, Threads: 1,
	})
	stored, err := older.Hash(migrationPassword)
	require.NoError(t, err)
	f := newMigrationFixture(t, stored)
	before := testutil.ToFloat64(monitoring.PasswordHashUpgradesTotal.WithLabelValues(coreauth.AlgorithmArgon2id))

	status, body := f.login(t, migrationPassword)
	require.Equal(t, http.StatusOK, status, "%v", body)

	upgraded := f.storedHash(t)
	assert.True(t, strings.HasPrefix(upgraded, "$argon2id$v=19$m=19456,t=3,p=1$"), "rewritten at today's cost: %q", upgraded)
	assert.Equal(t, before+1, testutil.ToFloat64(monitoring.PasswordHashUpgradesTotal.WithLabelValues(coreauth.AlgorithmArgon2id)))

	status, _ = f.login(t, migrationPassword)
	assert.Equal(t, http.StatusOK, status, "the account keeps working under its new hash")
}

func TestPasswordMigration_WrongPasswordUpgradesNothing(t *testing.T) {
	older := coreauth.NewArgon2idPasswordHasherWithParams(coreauth.Argon2idParams{
		Time: 2, Memory: 19456, Threads: 1,
	})
	stored, err := older.Hash(migrationPassword)
	require.NoError(t, err)
	f := newMigrationFixture(t, stored)

	status, _ := f.login(t, migrationPassword+"x")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, stored, f.storedHash(t), "no rehash without proof of the password")
}

func TestPasswordHashCensus_CountsDeletedAccountsSeparately(t *testing.T) {
	f := newMigrationFixture(t, legacyDigest(migrationPassword))
	require.NoError(t, f.db.Delete(&domain.User{}, "id = ?", f.user.ID).Error)

	counts, err := f.census.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), counts["active"][coreauth.AlgorithmLegacySHA256])
	assert.Equal(t, int64(1), counts["deleted"][coreauth.AlgorithmLegacySHA256],
		"a soft-deleted row still holds its hash and must stay visible")
}
