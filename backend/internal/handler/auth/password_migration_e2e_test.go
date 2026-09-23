// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

// End-to-end proof of the Argon2id migration (#484), through the real login
// route, the real GORM user repository and the real hasher: an account still
// holding the SHA-256 digest the first release wrote signs in once and leaves
// with an Argon2id hash; after the cutoff it is sent to password reset instead.

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

// newMigrationFixture seeds one account whose stored password is the unsalted
// hex SHA-256 digest the pre-Argon2id hasher wrote, and mounts the login route
// with the given cutoff.
func newMigrationFixture(t *testing.T, cutoff time.Time) *migrationFixture {
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

	digest := sha256.Sum256([]byte(migrationPassword))
	user := &domain.User{
		ID: uuid.New(), Email: "legacy@a.io", Username: "legacy",
		FullName: "Legacy Account", IsActive: true,
		Password: hex.EncodeToString(digest[:]), DefaultOrgID: &orgID,
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
		Time: 2, Memory: 19456, Threads: 1,
	})
	userRepo := repository.NewGormUserRepository(db)
	loginUC := appauth.NewLoginUseCase(userRepo, coreauth.NewTokenManager(db, keys), hasher).
		WithLegacyHashCutoff(cutoff).
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

func TestPasswordMigration_LegacyHashIsUpgradedOnSignIn(t *testing.T) {
	f := newMigrationFixture(t, migrationNow.Add(90*24*time.Hour))
	before := testutil.ToFloat64(monitoring.PasswordHashUpgradesTotal.WithLabelValues(coreauth.AlgorithmLegacySHA256))

	counts, err := f.census.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["active"][coreauth.AlgorithmLegacySHA256], "census must see the legacy account")

	status, body := f.login(t, migrationPassword)
	require.Equal(t, http.StatusOK, status, "a legacy account signs in before the cutoff: %v", body)

	stored := f.storedHash(t)
	assert.Equal(t, coreauth.AlgorithmArgon2id, coreauth.HashAlgorithm(stored), "the stored hash must now be Argon2id")
	assert.True(t, coreauth.NewArgon2idPasswordHasher().Verify(stored, migrationPassword), "the new hash must verify the same password")
	assert.Equal(t, before+1, testutil.ToFloat64(monitoring.PasswordHashUpgradesTotal.WithLabelValues(coreauth.AlgorithmLegacySHA256)))

	// The metric the migration is judged by goes down by one.
	counts, err = f.census.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), counts["active"][coreauth.AlgorithmLegacySHA256])
	assert.Equal(t, int64(1), counts["active"][coreauth.AlgorithmArgon2id])

	// And the account keeps working under its new hash.
	status, _ = f.login(t, migrationPassword)
	assert.Equal(t, http.StatusOK, status)
}

func TestPasswordMigration_WrongPasswordUpgradesNothing(t *testing.T) {
	f := newMigrationFixture(t, migrationNow.Add(90*24*time.Hour))
	legacy := f.user.Password

	status, _ := f.login(t, migrationPassword+"x")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, legacy, f.storedHash(t), "no rehash without proof of the password")
}

func TestPasswordMigration_AfterCutoffTheAccountIsSentToReset(t *testing.T) {
	f := newMigrationFixture(t, migrationNow.Add(-time.Hour))
	legacy := f.user.Password
	before := testutil.ToFloat64(monitoring.PasswordHashExpiredLoginsTotal)

	status, body := f.login(t, migrationPassword)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "password_reset_required", body["code"])
	assert.Nil(t, body["token_pair"], "no session past the cutoff")
	assert.Equal(t, legacy, f.storedHash(t), "a refused login must not upgrade the hash")
	assert.Equal(t, before+1, testutil.ToFloat64(monitoring.PasswordHashExpiredLoginsTotal))
}

// The reset-required answer is only ever given to someone who knows the
// password. A wrong guess past the cutoff gets the ordinary 401, so the
// distinct answer is no oracle for "this account is on the old algorithm".
func TestPasswordMigration_AfterCutoffAWrongPasswordLearnsNothing(t *testing.T) {
	f := newMigrationFixture(t, migrationNow.Add(-time.Hour))

	status, body := f.login(t, "not-the-password")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.NotEqual(t, "password_reset_required", body["code"])
}

func TestPasswordHashCensus_CountsDeletedAccountsSeparately(t *testing.T) {
	f := newMigrationFixture(t, migrationNow.Add(90*24*time.Hour))
	require.NoError(t, f.db.Delete(&domain.User{}, "id = ?", f.user.ID).Error)

	counts, err := f.census.Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), counts["active"][coreauth.AlgorithmLegacySHA256])
	assert.Equal(t, int64(1), counts["deleted"][coreauth.AlgorithmLegacySHA256],
		"a soft-deleted row still holds its hash and must stay visible")
}
