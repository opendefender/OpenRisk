// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
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

// #1215 — filter_query and update_data are jsonb maps. Without a serializer
// GORM wrote them but could not scan them back on Postgres, so every job with
// a filter failed to read. Runs against a migrated database; everything
// happens in an outer transaction that is rolled back.
func withBulkPG(t *testing.T, fn func(tx *gorm.DB, s *BulkOperationService)) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		fn(tx, &BulkOperationService{db: tx})
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// createBulkOpPG writes a job through GORM, the way CreateBulkOperation does,
// so the write path of the serializer is exercised too.
func createBulkOpPG(t *testing.T, tx *gorm.DB, tenant, user uuid.UUID, filter, update map[string]interface{}, createdAt time.Time) *domain.BulkOperation {
	t.Helper()
	op := &domain.BulkOperation{
		ID: uuid.New(), OperationType: domain.BulkOperationTypeUpdate,
		Status:      domain.BulkOperationStatusPending,
		FilterQuery: filter, UpdateData: update,
		TenantID: tenant, CreatedBy: user, CreatedAt: createdAt,
	}
	require.NoError(t, tx.Create(op).Error)
	return op
}

func TestGetBulkOperation_Postgres_Success(t *testing.T) {
	withBulkPG(t, func(tx *gorm.DB, s *BulkOperationService) {
		tenant, user := uuid.New(), uuid.New()
		filter := map[string]interface{}{"status": "open", "min_score": 4.5}
		update := map[string]interface{}{"status": "mitigated", "tags": []interface{}{"q4", "bulk"}}
		op := createBulkOpPG(t, tx, tenant, user, filter, update, time.Now())

		got, err := s.GetBulkOperation(op.ID, tenant)
		require.NoError(t, err)
		assert.Equal(t, filter, got.FilterQuery)
		assert.Equal(t, update, got.UpdateData)
		assert.Equal(t, tenant, got.TenantID)

		// A job without a filter or update data stores NULL and reads back nil.
		bare := createBulkOpPG(t, tx, tenant, user, nil, nil, time.Now())
		got, err = s.GetBulkOperation(bare.ID, tenant)
		require.NoError(t, err)
		assert.Nil(t, got.FilterQuery)
		assert.Nil(t, got.UpdateData)
		var nulls int64
		require.NoError(t, tx.Model(&domain.BulkOperation{}).
			Where("id = ? AND tenant_id = ? AND filter_query IS NULL AND update_data IS NULL", bare.ID, tenant).
			Count(&nulls).Error)
		assert.Equal(t, int64(1), nulls)
	})
}

func TestGetBulkOperation_Postgres_NotFound(t *testing.T) {
	withBulkPG(t, func(tx *gorm.DB, s *BulkOperationService) {
		_, err := s.GetBulkOperation(uuid.New(), uuid.New())
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

// Another tenant's job is indistinguishable from a missing one.
func TestGetBulkOperation_Postgres_Unauthorized(t *testing.T) {
	withBulkPG(t, func(tx *gorm.DB, s *BulkOperationService) {
		owner, intruder := uuid.New(), uuid.New()
		op := createBulkOpPG(t, tx, owner, uuid.New(), map[string]interface{}{"status": "open"}, nil, time.Now())

		_, err := s.GetBulkOperation(op.ID, intruder)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestListBulkOperations_Postgres_Success(t *testing.T) {
	withBulkPG(t, func(tx *gorm.DB, s *BulkOperationService) {
		tenant, other := uuid.New(), uuid.New()
		user := uuid.New()
		now := time.Now()
		older := createBulkOpPG(t, tx, tenant, user, map[string]interface{}{"status": "open"}, nil, now.Add(-time.Minute))
		newer := createBulkOpPG(t, tx, tenant, user, map[string]interface{}{"status": "closed"}, map[string]interface{}{"owner": "alice"}, now)
		createBulkOpPG(t, tx, other, user, map[string]interface{}{"status": "open"}, nil, now)        // other tenant, same user id
		createBulkOpPG(t, tx, tenant, uuid.New(), map[string]interface{}{"status": "open"}, nil, now) // same tenant, other user

		ops, err := s.ListBulkOperations(user, tenant, 50, 0)
		require.NoError(t, err)
		require.Len(t, ops, 2)
		assert.Equal(t, newer.ID, ops[0].ID)
		assert.Equal(t, map[string]interface{}{"status": "closed"}, ops[0].FilterQuery)
		assert.Equal(t, map[string]interface{}{"owner": "alice"}, ops[0].UpdateData)
		assert.Equal(t, older.ID, ops[1].ID)
		assert.Equal(t, map[string]interface{}{"status": "open"}, ops[1].FilterQuery)
		for _, op := range ops {
			assert.Equal(t, tenant, op.TenantID)
		}
	})
}

func TestListBulkOperations_Postgres_Unauthorized(t *testing.T) {
	withBulkPG(t, func(tx *gorm.DB, s *BulkOperationService) {
		owner, intruder := uuid.New(), uuid.New()
		user := uuid.New()
		createBulkOpPG(t, tx, owner, user, map[string]interface{}{"status": "open"}, nil, time.Now())

		ops, err := s.ListBulkOperations(user, intruder, 50, 0)
		require.NoError(t, err)
		assert.Empty(t, ops)
	})
}
