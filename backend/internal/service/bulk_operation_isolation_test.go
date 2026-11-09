// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
)

// setupBulkTestDB spins up an in-memory sqlite with a minimal risks table (only
// the columns the bulk filter/count touch) plus the bulk_operations table, and
// points the global DB at it (the service reads database.DB).
func setupBulkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:bulk_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Hand-built (AutoMigrate can't build this on sqlite: gen_random_uuid() default
	// + jsonb columns). jsonb → TEXT; only the columns the lookups touch are needed.
	if err := db.Exec(`CREATE TABLE bulk_operations (
		id TEXT PRIMARY KEY, operation_type TEXT, status TEXT,
		filter_query TEXT, resource_count INTEGER, processed_count INTEGER,
		update_data TEXT, export_format TEXT, result_url TEXT,
		error_message TEXT, error_count INTEGER,
		tenant_id TEXT, created_by TEXT, created_at DATETIME,
		started_at DATETIME, completed_at DATETIME, estimated_time INTEGER
	);`).Error; err != nil {
		t.Fatalf("create bulk_operations: %v", err)
	}
	// Minimal risks table — Count only needs the columns referenced in WHERE.
	if err := db.Exec(`CREATE TABLE risks (
		id TEXT PRIMARY KEY, tenant_id TEXT, status TEXT, score REAL, deleted_at DATETIME
	);`).Error; err != nil {
		t.Fatalf("create risks: %v", err)
	}
	database.DB = db
	return db
}

// insertOp writes a bulk_operations row via raw SQL (NULL jsonb columns) so the
// test does not depend on GORM serializing map[string]interface{} on sqlite.
func insertOp(t *testing.T, db *gorm.DB, tenant, user uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(`INSERT INTO bulk_operations
		(id, operation_type, status, tenant_id, created_by, created_at)
		VALUES (?, 'export', 'pending', ?, ?, ?)`,
		id.String(), tenant.String(), user.String(), time.Now()).Error; err != nil {
		t.Fatalf("insert op: %v", err)
	}
	return id
}

func seedRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID, status string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO risks (id, tenant_id, status, score) VALUES (?, ?, ?, ?)`,
		uuid.New().String(), tenant.String(), status, 5.0).Error; err != nil {
		t.Fatalf("seed risk: %v", err)
	}
}

// The core RULE #2 guarantee: a bulk filter must only ever see the caller's tenant.
func TestBulkOperation_CountResourcesIsTenantScoped(t *testing.T) {
	db := setupBulkTestDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	seedRisk(t, db, tenantA, "open")
	seedRisk(t, db, tenantA, "open")
	seedRisk(t, db, tenantB, "open") // must NOT be visible to tenant A

	s := NewBulkOperationService()
	var count int64
	if err := s.countResourcesByFilter(&count, tenantA, map[string]interface{}{"status": "open"}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 risks for tenant A, got %d (cross-tenant leak if 3)", count)
	}
}

// GetBulkOperation must not return another tenant's operation by UUID.
func TestBulkOperation_GetIsTenantScoped(t *testing.T) {
	db := setupBulkTestDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	author := uuid.New()

	opID := insertOp(t, db, tenantB, author)

	s := NewBulkOperationService()
	if _, err := s.GetBulkOperation(opID, author, tenantA); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant A must NOT be able to read tenant B's bulk operation, got %v", err)
	}
	if _, err := s.GetBulkOperation(opID, author, tenantB); err != nil {
		t.Fatalf("tenant B must read its own operation: %v", err)
	}
}

// #1219 — a colleague in the same tenant who holds the UUID gets the same
// not-found as for a job that does not exist.
func TestBulkOperation_GetIsAuthorScoped(t *testing.T) {
	db := setupBulkTestDB(t)
	tenant, author, colleague := uuid.New(), uuid.New(), uuid.New()

	opID := insertOp(t, db, tenant, author)

	s := NewBulkOperationService()
	_, errColleague := s.GetBulkOperation(opID, colleague, tenant)
	if !errors.Is(errColleague, gorm.ErrRecordNotFound) {
		t.Fatalf("a colleague must NOT read the author's bulk operation, got %v", errColleague)
	}
	_, errMissing := s.GetBulkOperation(uuid.New(), author, tenant)
	if !errors.Is(errMissing, gorm.ErrRecordNotFound) {
		t.Fatalf("a missing job must read as not found, got %v", errMissing)
	}
	got, err := s.GetBulkOperation(opID, author, tenant)
	if err != nil {
		t.Fatalf("the author must read their own operation: %v", err)
	}
	if got.ID != opID || got.CreatedBy != author {
		t.Fatalf("expected op %s by %s, got %s by %s", opID, author, got.ID, got.CreatedBy)
	}
}

// ListBulkOperations must only return the caller's tenant's operations.
func TestBulkOperation_ListIsTenantScoped(t *testing.T) {
	db := setupBulkTestDB(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	userA := uuid.New()

	for _, spec := range []struct {
		tenant uuid.UUID
		user   uuid.UUID
	}{{tenantA, userA}, {tenantA, userA}, {tenantB, uuid.New()}} {
		insertOp(t, db, spec.tenant, spec.user)
	}

	s := NewBulkOperationService()
	ops, err := s.ListBulkOperations(userA, tenantA, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops for tenant A, got %d", len(ops))
	}
	for _, op := range ops {
		if op.TenantID != tenantA {
			t.Fatalf("leaked op from tenant %s", op.TenantID)
		}
	}
}
