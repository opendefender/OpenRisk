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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// #754 — the sqlite tests fail the second write with a GORM callback; this one
// makes Postgres itself refuse it, with a trigger, and checks the secret delete
// is rolled back by the database. Temporary tables shadow the real ones for this
// connection only, and the outer transaction is rolled back at the end.
func TestGormMFARepository_DisableMFA_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, sql := range []string{
			`CREATE TEMP TABLE mfa_secrets (LIKE public.mfa_secrets INCLUDING ALL) ON COMMIT DROP`,
			`CREATE TEMP TABLE mfa_backup_codes (LIKE public.mfa_backup_codes INCLUDING ALL) ON COMMIT DROP`,
			`CREATE FUNCTION pg_temp.refuse_code_delete() RETURNS trigger LANGUAGE plpgsql AS
			   $$ BEGIN IF current_setting('or754.refuse', true) = 'on' THEN
			        RAISE EXCEPTION 'backup code delete refused'; END IF; RETURN OLD; END $$`,
			`CREATE TRIGGER refuse BEFORE DELETE ON pg_temp.mfa_backup_codes
			   FOR EACH ROW EXECUTE FUNCTION pg_temp.refuse_code_delete()`,
		} {
			require.NoError(t, tx.Exec(sql).Error, sql)
		}
		repo := NewGormMFARepository(tx)
		ctx := context.Background()
		user, tenant := uuid.New(), uuid.New()
		seed := func() {
			require.NoError(t, tx.Create(&domain.MFASecret{ID: uuid.New(), UserID: user, TenantID: tenant, SecretEncrypted: "x", IsVerified: true}).Error)
			for range 3 {
				require.NoError(t, tx.Create(&domain.MFABackupCode{ID: uuid.New(), UserID: user, TenantID: tenant, CodeHash: uuid.NewString()}).Error)
			}
		}
		count := func() (secrets, codes int64) {
			// Tenant-agnostic on purpose: proves no row survives under ANY tenant.
			require.NoError(t, tx.Unscoped().Model(&domain.MFASecret{}).Where("user_id = ?", user).Count(&secrets).Error)
			// Same: every tenant's backup codes for this user.
			require.NoError(t, tx.Model(&domain.MFABackupCode{}).Where("user_id = ?", user).Count(&codes).Error)
			return
		}
		seed()

		// Postgres refuses the second write: nothing may be deleted.
		require.NoError(t, tx.Exec(`SET LOCAL or754.refuse = 'on'`).Error)
		require.Error(t, repo.DisableMFA(ctx, user, tenant))
		s, c := count()
		assert.EqualValues(t, 1, s, "the secret delete must roll back with the failed code delete")
		assert.EqualValues(t, 3, c)

		// Another tenant's id deletes nothing.
		require.NoError(t, tx.Exec(`SET LOCAL or754.refuse = 'off'`).Error)
		require.NoError(t, repo.DisableMFA(ctx, user, uuid.New()))
		s, c = count()
		assert.EqualValues(t, 1, s)
		assert.EqualValues(t, 3, c)

		// The real call removes both, and the unique user_id lets the user enrol again.
		require.NoError(t, repo.DisableMFA(ctx, user, tenant))
		s, c = count()
		assert.Zero(t, s, "hard delete, no tombstone")
		assert.Zero(t, c)
		seed()
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
