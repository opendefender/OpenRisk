// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
)

// The test-definition repository (#1415). It reads and writes control_tests.
// tested_by is the implementation_id column (A10.4), not an edge row. Every query
// names tenant_id, and the parent chain (implementation, then control) is
// resolved live in the tenant on every read and write, so a test of a foreign or
// deleted implementation reads as absent.

// TestRepository stores the test definitions of an implementation. The audit
// entry of a write is appended in the same transaction.
type TestRepository struct {
	db    *gorm.DB
	audit AuditAppender
}

// NewTestRepository builds the repository.
func NewTestRepository(db *gorm.DB, audit AuditAppender) *TestRepository {
	return &TestRepository{db: db, audit: audit}
}

var _ ctlapp.TestRepository = (*TestRepository)(nil)

func testNotFound() error {
	return domain.InternalControlError(domain.ErrNotFound, domain.IntCtlTestNotFound)
}

// TestToRow is the persistence shape of a canonical ControlTest (without its
// results, which have their own table, #1416).
func TestToRow(t canonical.ControlTest, deletedAt *time.Time) (ControlTestRow, error) {
	prov, err := json.Marshal(t.Provenance)
	if err != nil {
		return ControlTestRow{}, fmt.Errorf("controlchain: provenance: %w", err)
	}
	return ControlTestRow{
		ID: t.ID, TenantID: t.TenantID, Source: string(t.Source), SourceID: t.SourceID,
		SourceVersion: t.SourceVersion, Confidence: t.Confidence, Provenance: string(prov),
		ObservedAt: t.ObservedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DeletedAt: deletedAt,
		ImplementationID: t.ImplementationID, Name: t.Name, Checks: t.Checks, Method: string(t.Method),
		Procedure: t.Procedure, Collector: t.Collector, Cadence: t.Cadence, PassCriteria: t.PassCriteria,
	}, nil
}

// RowToTest is the reverse of TestToRow.
func RowToTest(row ControlTestRow) (canonical.ControlTest, error) {
	src := canonical.Source(row.Source)
	prov, err := canonical.ParseProvenance([]byte(row.Provenance), src)
	if err != nil {
		return canonical.ControlTest{}, fmt.Errorf("controlchain: test %s provenance: %w", row.ID, err)
	}
	return canonical.ControlTest{
		Envelope: canonical.Envelope{
			ID: row.ID, TenantID: row.TenantID, Source: src, SourceID: row.SourceID, SourceVersion: row.SourceVersion,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ObservedAt: row.ObservedAt,
			Confidence: row.Confidence, Provenance: prov,
		},
		ImplementationID: row.ImplementationID, Name: row.Name, Checks: row.Checks, Method: canonical.TestMethod(row.Method),
		Procedure: row.Procedure, Collector: row.Collector, Cadence: row.Cadence, PassCriteria: row.PassCriteria,
	}, nil
}

// liveTestRow loads the live test of the tenant (under row lock when lock is set
// and the engine has one) and resolves its implementation, and through it the
// control, live in the same tenant. Anything missing is the same ErrNotFound.
func liveTestRow(ctx context.Context, tx *gorm.DB, tenantID, id uuid.UUID, lock bool) (ControlTestRow, error) {
	q := tx.WithContext(ctx).Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id)
	if lock && tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row ControlTestRow
	if err := q.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, testNotFound()
		}
		return row, err
	}
	if _, err := liveImplementationRow(ctx, tx, tenantID, row.ImplementationID, false); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return row, testNotFound()
		}
		return row, err
	}
	return row, nil
}

// CreateTest stores the test and its audit entry in one transaction, after the
// implementation (and its control) resolved live in the tenant.
func (r *TestRepository) CreateTest(ctx context.Context, tenantID uuid.UUID, t canonical.ControlTest, audit *domain.AuditEvent) error {
	if tenantID == uuid.Nil || t.TenantID != tenantID {
		return domain.ErrForbidden
	}
	if audit == nil {
		return errors.New("controlchain.CreateTest: an audit entry is required")
	}
	row, err := TestToRow(t, nil)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Locked (Postgres) so a concurrent delete of the implementation, which
		// cascades to its tests, cannot leave this one undeleted under it.
		if _, err := liveImplementationRow(ctx, tx, tenantID, t.ImplementationID, true); err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		// A new test is a new in-scope signal of its control (#555).
		if err := controlstatus.MarkImplementation(ctx, tx, tenantID, t.ImplementationID); err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	return wrapApp(err, "controlchain.CreateTest")
}

// GetTest reads one live test of the tenant.
func (r *TestRepository) GetTest(ctx context.Context, tenantID, id uuid.UUID) (*canonical.ControlTest, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	row, err := liveTestRow(ctx, r.db, tenantID, id, false)
	if err != nil {
		return nil, wrapApp(err, "controlchain.GetTest")
	}
	t, err := RowToTest(row)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTests returns one page of the live tests of a live implementation of the
// tenant, by name then id.
func (r *TestRepository) ListTests(ctx context.Context, tenantID, implementationID uuid.UUID, limit, offset int) ([]canonical.ControlTest, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	if _, err := liveImplementationRow(ctx, r.db, tenantID, implementationID, false); err != nil {
		return nil, wrapApp(err, "controlchain.ListTests")
	}
	var rows []ControlTestRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND implementation_id = ? AND deleted_at IS NULL", tenantID, implementationID).
		Order("name ASC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlchain.ListTests: %w", err)
	}
	out := make([]canonical.ControlTest, len(rows))
	for i := range rows {
		if out[i], err = RowToTest(rows[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UpdateTest loads the live test under lock (tenant_id and id), lets apply
// decide, then stores the result and the audit entry in one transaction. The
// UPDATE is a map, so a cleared field (nil) is written. A nil entry from apply
// means no change: nothing is written.
func (r *TestRepository) UpdateTest(ctx context.Context, tenantID, id uuid.UUID,
	apply func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error),
) (*canonical.ControlTest, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var out canonical.ControlTest
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := liveTestRow(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		cur, err := RowToTest(row)
		if err != nil {
			return err
		}
		next, ev, err := apply(cur)
		if err != nil {
			return err
		}
		out = next
		if ev == nil {
			return nil
		}
		res := tx.Model(&ControlTestRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{
				"name": next.Name, "checks": next.Checks, "method": string(next.Method),
				"procedure": next.Procedure, "collector": next.Collector, "cadence": next.Cadence,
				"pass_criteria": next.PassCriteria, "updated_at": next.UpdatedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return testNotFound()
		}
		// The cadence decides the freshness window (#555).
		if err := controlstatus.MarkTest(ctx, tx, tenantID, id); err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, ev)
	})
	if err != nil {
		return nil, wrapApp(err, "controlchain.UpdateTest")
	}
	return &out, nil
}

// DeleteTest soft-deletes the live test (UPDATE deleted_at, tenant_id in the
// WHERE) after apply accepted it, with its audit entry, in one transaction.
func (r *TestRepository) DeleteTest(ctx context.Context, tenantID, id uuid.UUID, apply func(cur canonical.ControlTest) (*domain.AuditEvent, error)) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := liveTestRow(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		cur, err := RowToTest(row)
		if err != nil {
			return err
		}
		ev, err := apply(cur)
		if err != nil {
			return err
		}
		// Marked before the delete, while the test still resolves to its control (#555).
		if err := controlstatus.MarkTest(ctx, tx, tenantID, id); err != nil {
			return err
		}
		res := tx.Model(&ControlTestRow{}).
			Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, id).
			Updates(map[string]any{"deleted_at": ev.CreatedAt, "updated_at": ev.CreatedAt})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return testNotFound()
		}
		return r.audit.AppendTx(ctx, tx, ev)
	})
	return wrapApp(err, "controlchain.DeleteTest")
}
