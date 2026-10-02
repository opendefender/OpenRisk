// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #889 — an unfinished enrolment can be started over: the unverified secret's
// key is replaced in place, and a verified one is never touched.

func checkReplaceUnverified(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := NewGormMFARepository(db)
	ctx := context.Background()
	step := int64(42)
	used := time.Now()

	pending, tenant := uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: pending, TenantID: tenant, SecretEncrypted: "old",
		LastTOTPStep: &step, LastUsedAt: &used,
	}).Error)
	enrolled := uuid.New()
	require.NoError(t, db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: enrolled, TenantID: tenant, SecretEncrypted: "kept", IsVerified: true,
	}).Error)

	// Another tenant's request cannot reach the row.
	ok, err := repo.ReplaceUnverifiedMFASecret(ctx, pending, uuid.New(), "intruder")
	require.NoError(t, err)
	assert.False(t, ok, "tenant-scoped: another tenant replaces nothing")

	ok, err = repo.ReplaceUnverifiedMFASecret(ctx, pending, tenant, "new")
	require.NoError(t, err)
	assert.True(t, ok)
	var got domain.MFASecret
	require.NoError(t, db.Where("user_id = ?", pending).First(&got).Error)
	assert.Equal(t, "new", got.SecretEncrypted)
	assert.False(t, got.IsVerified)
	assert.Nil(t, got.LastTOTPStep, "the abandoned key's replay step must not carry over")
	assert.Nil(t, got.LastUsedAt)

	ok, err = repo.ReplaceUnverifiedMFASecret(ctx, enrolled, tenant, "attack")
	require.NoError(t, err)
	assert.False(t, ok, "a verified secret is never replaced")
	var kept domain.MFASecret
	require.NoError(t, db.Where("user_id = ?", enrolled).First(&kept).Error)
	assert.Equal(t, "kept", kept.SecretEncrypted)
}

func TestGormMFARepository_ReplaceUnverifiedMFASecret(t *testing.T) {
	_, db := setupMFARepo(t)
	checkReplaceUnverified(t, db)
}

func TestGormMFARepository_ReplaceUnverifiedMFASecret_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Exec(`CREATE TEMP TABLE mfa_secrets (LIKE public.mfa_secrets INCLUDING ALL) ON COMMIT DROP`).Error)
		checkReplaceUnverified(t, tx)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
