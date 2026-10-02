// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// #754 — DisableMFA removes the secret and the backup codes in one transaction.

func setupMFARepo(t *testing.T) (*GormMFARepository, *gorm.DB) {
	t.Helper()
	dsn := "file:mfa_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for _, m := range []struct {
		table string
		model any
	}{
		{"mfa_secrets", &domain.MFASecret{}},
		{"mfa_backup_codes", &domain.MFABackupCode{}},
	} {
		require.NoError(t, db.Exec(`CREATE TABLE `+m.table+` (id TEXT PRIMARY KEY)`).Error)
		require.NoError(t, sqliteschema.Reconcile(db, m.table, m.model))
	}
	return NewGormMFARepository(db), db
}

func seedMFA(t *testing.T, db *gorm.DB, userID, tenantID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Create(&domain.MFASecret{ID: uuid.New(), UserID: userID, TenantID: tenantID, SecretEncrypted: "x", IsVerified: true}).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&domain.MFABackupCode{ID: uuid.New(), UserID: userID, TenantID: tenantID, CodeHash: uuid.NewString()}).Error)
	}
}

// countMFA counts rows including soft-deleted ones: a tombstone is not a deletion.
func countMFA(t *testing.T, db *gorm.DB, userID uuid.UUID) (secrets, codes int64) {
	t.Helper()
	require.NoError(t, db.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", userID).Count(&secrets).Error)
	require.NoError(t, db.Model(&domain.MFABackupCode{}).Where("user_id = ?", userID).Count(&codes).Error)
	return secrets, codes
}

func TestGormMFARepository_DisableMFA_DeletesSecretAndCodes(t *testing.T) {
	repo, db := setupMFARepo(t)
	user, tenant, other := uuid.New(), uuid.New(), uuid.New()
	seedMFA(t, db, user, tenant)
	seedMFA(t, db, other, tenant)

	require.NoError(t, repo.DisableMFA(context.Background(), user, tenant))

	secrets, codes := countMFA(t, db, user)
	assert.Zero(t, secrets, "the secret must be gone, not soft-deleted")
	assert.Zero(t, codes)
	secrets, codes = countMFA(t, db, other)
	assert.EqualValues(t, 1, secrets, "another user's factor is untouched")
	assert.EqualValues(t, 3, codes)
}

func TestGormMFARepository_DisableMFA_IsTenantScoped(t *testing.T) {
	repo, db := setupMFARepo(t)
	user, tenant := uuid.New(), uuid.New()
	seedMFA(t, db, user, tenant)

	require.NoError(t, repo.DisableMFA(context.Background(), user, uuid.New()))

	secrets, codes := countMFA(t, db, user)
	assert.EqualValues(t, 1, secrets)
	assert.EqualValues(t, 3, codes)
}

func TestGormMFARepository_DisableMFA_RollsBackWhenTheSecondWriteFails(t *testing.T) {
	repo, db := setupMFARepo(t)
	user, tenant := uuid.New(), uuid.New()
	seedMFA(t, db, user, tenant)

	// Fail the backup-code delete, which runs after the secret delete.
	boom := errors.New("backup code delete failed")
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_codes", func(tx *gorm.DB) {
		if tx.Statement.Table == "mfa_backup_codes" {
			_ = tx.AddError(boom)
		}
	}))

	err := repo.DisableMFA(context.Background(), user, tenant)
	require.ErrorIs(t, err, boom)

	secrets, codes := countMFA(t, db, user)
	assert.EqualValues(t, 1, secrets, "the secret delete must roll back")
	assert.EqualValues(t, 3, codes)

	var s domain.MFASecret
	require.NoError(t, db.Where("user_id = ?", user).First(&s).Error, "the secret is still live, not a tombstone")
}

func TestGormMFARepository_DisableMFA_AllowsReenrolment(t *testing.T) {
	repo, db := setupMFARepo(t)
	user, tenant := uuid.New(), uuid.New()
	seedMFA(t, db, user, tenant)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX ux_mfa_user ON mfa_secrets (user_id)`).Error)

	require.NoError(t, repo.DisableMFA(context.Background(), user, tenant))

	// Production's mfa_secrets.user_id is UNIQUE with no deleted_at clause.
	require.NoError(t, repo.CreateMFASecret(context.Background(),
		&domain.MFASecret{ID: uuid.New(), UserID: user, TenantID: tenant, SecretEncrypted: "y"}))
}
