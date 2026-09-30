// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmfa

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/pkg/crypto"
	"github.com/opendefender/openrisk/pkg/otp"
)

// #849 — the step-up gate accepts a code once, like login does.

var gateKey = []byte("0123456789abcdef0123456789abcdef")

func newGate(t *testing.T) (*Gate, uuid.UUID, uuid.UUID, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:gate_"+uuid.NewString()+"?mode=memory&cache=private"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE mfa_secrets (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "mfa_secrets", &domain.MFASecret{}))

	plain, err := otp.GenerateTOTPSecret()
	require.NoError(t, err)
	enc, err := crypto.EncryptAES256GCM(plain, gateKey)
	require.NoError(t, err)
	user, tenant := uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.MFASecret{
		ID: uuid.New(), UserID: user, TenantID: tenant, SecretEncrypted: enc, IsVerified: true,
	}).Error)
	return NewGate(repository.NewGormMFARepository(db), gateKey), user, tenant, plain
}

func TestGate_ReplayedCodeIsRefused(t *testing.T) {
	gate, user, tenant, plain := newGate(t)
	code, err := totp.GenerateCode(plain, time.Now())
	require.NoError(t, err)

	require.NoError(t, gate.VerifyRequired(context.Background(), user, tenant, code))
	assert.ErrorIs(t, gate.VerifyRequired(context.Background(), user, tenant, code), ErrInvalidCode,
		"one code confirms one sensitive action")
}

func TestGate_CodeIsScopedToItsTenant(t *testing.T) {
	gate, user, tenant, plain := newGate(t)
	code, err := totp.GenerateCode(plain, time.Now())
	require.NoError(t, err)

	// Another tenant id finds no secret, so it is exempt and consumes nothing...
	require.NoError(t, gate.VerifyRequired(context.Background(), user, uuid.New(), code))
	// ...and the code is still good for its own tenant.
	assert.NoError(t, gate.VerifyRequired(context.Background(), user, tenant, code))
}
