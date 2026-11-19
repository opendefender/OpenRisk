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
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func twoControls() []*domain.ComplianceControl {
	return []*domain.ComplianceControl{
		{ID: uuid.New(), ReferenceCode: "A.1", Name: "One"},
		{ID: uuid.New(), ReferenceCode: "A.2", Name: "Two"},
	}
}

func tableCount(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

func TestCreateFrameworkWithControls_Success(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	fw := &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "F", Version: "1"}

	// A caller-set foreign tenant or status on a control must be overwritten.
	ctls := twoControls()
	ctls[0].TenantID = uuid.New()
	ctls[0].Status = domain.ControlStatusImplemented
	require.NoError(t, repo.CreateFrameworkWithControls(ctx, fw, ctls))

	got, err := repo.ListControlsByFramework(ctx, tenant, fw.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		assert.Equal(t, tenant, c.TenantID)
		assert.Equal(t, domain.ControlStatusNotImplemented, c.Status)
	}
	assert.EqualValues(t, 2, tableCount(t, repo.db, controlchain.TableControls), "each control is mirrored onto the chain")
	other, err := repo.ListControlsByFramework(ctx, uuid.New(), fw.ID)
	require.NoError(t, err)
	assert.Empty(t, other, "another tenant must not see the controls")
}

func TestCreateFrameworkWithControls_Unauthorized(t *testing.T) {
	repo := setupComplianceRepo(t)
	err := repo.CreateFrameworkWithControls(context.Background(),
		&domain.ComplianceFramework{ID: uuid.New(), Name: "F"}, twoControls())
	require.Error(t, err)
	assert.Zero(t, tableCount(t, repo.db, "compliance_frameworks"))
}

func TestCreateFrameworkWithControls_Conflict(t *testing.T) {
	repo := setupComplianceRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	require.NoError(t, repo.CreateFrameworkWithControls(ctx, &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "F", Version: "1"}, twoControls()))

	err := repo.CreateFrameworkWithControls(ctx, &domain.ComplianceFramework{ID: uuid.New(), TenantID: tenant, Name: "F", Version: "1"}, twoControls())
	require.ErrorIs(t, err, domain.ErrConflict)
	assert.EqualValues(t, 1, tableCount(t, repo.db, "compliance_frameworks"))
	assert.EqualValues(t, 2, tableCount(t, repo.db, "compliance_controls"))

	// Another tenant may hold the same name and version.
	require.NoError(t, repo.CreateFrameworkWithControls(ctx, &domain.ComplianceFramework{ID: uuid.New(), TenantID: uuid.New(), Name: "F", Version: "1"}, twoControls()))
}

func TestCreateFrameworkWithControls_RollsBackOnInjectedFailure(t *testing.T) {
	repo := setupComplianceRepo(t)
	db := repo.db
	boom := errors.New("injected write failure")
	// Fail the second control insert: the framework, the first control and its
	// control-chain mirror are already written inside the transaction.
	calls := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_mid_import", func(tx *gorm.DB) {
		if tx.Statement.Table == "compliance_controls" {
			calls++
			if calls == 2 {
				_ = tx.AddError(boom)
			}
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:fail_mid_import") })

	err := repo.CreateFrameworkWithControls(context.Background(),
		&domain.ComplianceFramework{ID: uuid.New(), TenantID: uuid.New(), Name: "F", Version: "1"}, twoControls())
	require.ErrorIs(t, err, boom)
	assert.Equal(t, 2, calls, "the failure must have hit mid-import")
	assert.Zero(t, tableCount(t, db, "compliance_frameworks"), "no framework may remain")
	assert.Zero(t, tableCount(t, db, "compliance_controls"), "no control may remain")
	assert.Zero(t, tableCount(t, db, controlchain.TableControls), "no control-chain mirror may remain")
	assert.Zero(t, tableCount(t, db, controlchain.TableCanonicalEdges), "no control-chain edge may remain")
}
