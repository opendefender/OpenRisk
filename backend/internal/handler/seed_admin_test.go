// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// Regression tests for #485: the first boot used to seed admin@opendefender.io
// with "admin123" whenever INITIAL_ADMIN_PASSWORD was unset and APP_ENV was not
// "production", which the Helm chart never set.

func newSeedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	// One connection: every pooled connection to ":memory:" is its own database.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	for _, tbl := range []struct {
		ddl   string
		name  string
		model any
	}{
		{`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, username TEXT UNIQUE NOT NULL)`, "users", &domain.User{}},
		{`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL)`, "organizations", &domain.Organization{}},
		{`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`, "organization_members", &domain.OrganizationMember{}},
	} {
		require.NoError(t, db.Exec(tbl.ddl).Error)
		require.NoError(t, sqliteschema.Reconcile(db, tbl.name, tbl.model))
	}
	return db
}

// fastHasher is the production hasher at the cheapest parameters, so the tests
// prove a real Argon2id round trip without paying 64 MiB per hash.
func fastHasher() *auth.Argon2idPasswordHasher {
	return auth.NewArgon2idPasswordHasherWithParams(auth.Argon2idParams{Time: 1, Memory: 1024, Threads: 1})
}

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func readSecret(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSuffix(string(raw), "\n")
}

func TestSeedInitialAdmin_Success_GeneratesPasswordFile(t *testing.T) {
	db := newSeedDB(t)
	hasher := fastHasher()
	file := filepath.Join(t.TempDir(), "secrets", "initial_admin_password")

	seeded, err := seedInitialAdmin(db, hasher, envOf(map[string]string{
		"INITIAL_ADMIN_PASSWORD_FILE": file,
	}))
	require.NoError(t, err)
	require.NotNil(t, seeded)
	assert.Equal(t, file, seeded.PasswordFile)

	info, err := os.Stat(file)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the generated password must be readable by its owner only")

	password := readSecret(t, file)
	assert.Len(t, password, generatedPasswordLength)

	var admin domain.User
	require.NoError(t, db.Where("email = ?", InitialAdminEmail).First(&admin).Error)
	assert.True(t, hasher.Verify(admin.Password, password), "the file must hold the password the account was created with")
	assert.False(t, hasher.Verify(admin.Password, retiredDefaultAdminPassword))
	assert.True(t, admin.IsActive)

	// The account must be able to sign in: default organisation plus a root membership.
	require.NotNil(t, admin.DefaultOrgID)
	var org domain.Organization
	require.NoError(t, db.First(&org, "id = ?", *admin.DefaultOrgID).Error)
	assert.Equal(t, admin.ID, org.OwnerID)
	var member domain.OrganizationMember
	require.NoError(t, db.Where("organization_id = ? AND user_id = ?", org.ID, admin.ID).First(&member).Error)
	assert.Equal(t, domain.RoleRoot, member.Role)
	assert.True(t, member.IsActive)
	assert.Equal(t, uuid.Nil, admin.RoleID, "authority comes from the root membership, not a legacy role row")
}

func TestSeedInitialAdmin_UsesProvidedPassword(t *testing.T) {
	db := newSeedDB(t)
	hasher := fastHasher()
	file := filepath.Join(t.TempDir(), "initial_admin_password")

	seeded, err := seedInitialAdmin(db, hasher, envOf(map[string]string{
		"INITIAL_ADMIN_PASSWORD":      "Operator-Chose-This-9",
		"INITIAL_ADMIN_PASSWORD_FILE": file,
	}))
	require.NoError(t, err)
	require.NotNil(t, seeded)
	assert.Empty(t, seeded.PasswordFile)
	assert.NoFileExists(t, file, "a supplied password is never copied to disk")

	var admin domain.User
	require.NoError(t, db.Where("email = ?", InitialAdminEmail).First(&admin).Error)
	assert.True(t, hasher.Verify(admin.Password, "Operator-Chose-This-9"))
}

func TestSeedInitialAdmin_SkipsWhenUsersExist(t *testing.T) {
	db := newSeedDB(t)
	require.NoError(t, db.Create(&domain.User{ID: uuid.New(), Email: "someone@example.test", Username: "someone"}).Error)
	file := filepath.Join(t.TempDir(), "initial_admin_password")

	seeded, err := seedInitialAdmin(db, fastHasher(), envOf(map[string]string{
		"INITIAL_ADMIN_PASSWORD_FILE": file,
	}))
	require.NoError(t, err)
	assert.Nil(t, seeded)
	assert.NoFileExists(t, file)

	var count int64
	require.NoError(t, db.Model(&domain.User{}).Where("email = ?", InitialAdminEmail).Count(&count).Error)
	assert.Zero(t, count)
}

func TestSeedInitialAdmin_RefusesWhenPasswordFileIsUnwritable(t *testing.T) {
	db := newSeedDB(t)
	// A regular file where the parent directory should be: MkdirAll fails.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	seeded, err := seedInitialAdmin(db, fastHasher(), envOf(map[string]string{
		"INITIAL_ADMIN_PASSWORD_FILE": filepath.Join(blocker, "initial_admin_password"),
	}))
	require.Error(t, err)
	assert.Nil(t, seeded)
	var fileErr *passwordFileError
	assert.ErrorAs(t, err, &fileErr, "this is the failure SeedAdminUser turns into a fatal boot error")
	assert.Contains(t, err.Error(), "INITIAL_ADMIN_PASSWORD")

	var count int64
	require.NoError(t, db.Model(&domain.User{}).Count(&count).Error)
	assert.Zero(t, count, "no administrator may exist whose password nobody can read")
}

func TestSeedInitialAdmin_RollsBackAndRemovesFileOnFailure(t *testing.T) {
	db := newSeedDB(t)
	// The organisation insert fails, after the user insert succeeded.
	require.NoError(t, db.Exec(`DROP TABLE organizations`).Error)
	file := filepath.Join(t.TempDir(), "initial_admin_password")

	seeded, err := seedInitialAdmin(db, fastHasher(), envOf(map[string]string{
		"INITIAL_ADMIN_PASSWORD_FILE": file,
	}))
	require.Error(t, err)
	assert.Nil(t, seeded)
	var fileErr *passwordFileError
	assert.False(t, strings.Contains(err.Error(), "cannot write"), "a database failure is not a file failure")
	assert.NotErrorAs(t, err, &fileErr)

	assert.NoFileExists(t, file, "a password for an account that does not exist must not be left behind")
	var users int64
	require.NoError(t, db.Model(&domain.User{}).Count(&users).Error)
	assert.Zero(t, users, "the user insert must be rolled back")
}

func TestUsesRetiredDefaultPassword(t *testing.T) {
	hasher := fastHasher()

	t.Run("flags an administrator still on the old default", func(t *testing.T) {
		db := newSeedDB(t)
		hash, err := hasher.Hash(retiredDefaultAdminPassword)
		require.NoError(t, err)
		require.NoError(t, db.Create(&domain.User{ID: uuid.New(), Email: InitialAdminEmail, Username: "admin", Password: hash}).Error)
		assert.True(t, usesRetiredDefaultPassword(db, hasher))
	})

	t.Run("stays quiet once the password was changed", func(t *testing.T) {
		db := newSeedDB(t)
		hash, err := hasher.Hash("Something-Else-Entirely-4")
		require.NoError(t, err)
		require.NoError(t, db.Create(&domain.User{ID: uuid.New(), Email: InitialAdminEmail, Username: "admin", Password: hash}).Error)
		assert.False(t, usesRetiredDefaultPassword(db, hasher))
	})

	t.Run("stays quiet when there is no initial administrator", func(t *testing.T) {
		assert.False(t, usesRetiredDefaultPassword(newSeedDB(t), hasher))
	})
}

func TestGenerateInitialAdminPassword(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		p, err := generateInitialAdminPassword()
		require.NoError(t, err)
		require.Len(t, p, generatedPasswordLength)
		for _, r := range p {
			require.True(t, strings.ContainsRune(generatedPasswordAlphabet, r), "unexpected character %q", r)
		}
		require.False(t, seen[p], "two draws produced the same password")
		seen[p] = true
	}
}

func TestWriteSecretFile_TightensAStaleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "initial_admin_password")
	require.NoError(t, os.WriteFile(path, []byte("stale\n"), 0o644))

	require.NoError(t, writeSecretFile(path, "fresh"))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Equal(t, "fresh", readSecret(t, path))

	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file may be left next to the secret")
}
