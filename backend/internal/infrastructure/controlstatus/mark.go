// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// The stale marks (ADR 0012 point 5). Every write that can change an input of a
// control's status calls one of these inside its own transaction, with that
// transaction's handle, so a commit and its invalidation cannot be separated by a
// crash. Each is one UPDATE that binds the tenant on every tenant-scoped table it
// reads or writes. A control with no row is not marked: the claim creates a row
// for it before it computes, so a write after the claim finds one.
//
// A deleting write calls its mark before it removes the rows the mark resolves
// through (edges, the soft-deleted test): the mark resolves controls on rows that
// are still there. A mark that matches no row is not an error.
//
// Write paths that mark (one test each, TestControlStatusMarks_EveryWritePathMarksItsControlsOnly
// in repository/control_status_engine_test.go):
//
//	RecordResult, AttachEvidence            -> MarkTest
//	CreateTest, UpdateTest, DeleteTest      -> MarkImplementation / MarkTest
//	CreateImplementation                    -> MarkControls
//	UpdateImplementation, DeleteImplementation -> MarkImplementation
//	UpdateControl, DeleteControl            -> MarkControls
//	exception Create, Revoke                -> MarkControls
//	evidence Update, Delete                 -> MarkEvidence
//	evidence Link, Unlink (the mirror)      -> MarkControls
//
// Known unmarked writer: controlchain/reconcile.go. The boot reconciler inserts
// control-level evidenced_by edges (~line 449) and backfills mitigated_by edges
// without marking any row, so a control whose edge only the reconciler wrote keeps a
// stale cache until another write or its next_change_at. Only the cache is affected:
// the status endpoint computes live. Tracked in a follow-up issue.

// The edge vocabulary of ADR 0008 D5, bound as values (never glued into the SQL).
var (
	evidencedBy  = string(canonical.VerbEvidencedBy)
	evidenceType = string(canonical.TypeEvidence)
	controlType  = string(canonical.TypeControl)
	testType     = string(canonical.TypeControlTest)
)

func needTenant(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.NewForbiddenError("tenant_id is required")
	}
	return nil
}

// MarkControls marks the rows of the given controls of the tenant. On PostgreSQL
// the rows are locked first, in control_id order, so two transactions that mark
// the same controls in opposite order wait for each other instead of deadlocking
// (40P01 would fail the evidence write that carried the mark). Every statement
// binds the tenant.
func MarkControls(ctx context.Context, tx *gorm.DB, tenantID uuid.UUID, controlIDs ...uuid.UUID) error {
	if err := needTenant(tenantID); err != nil {
		return err
	}
	if len(controlIDs) == 0 {
		return nil
	}
	if tx.Dialector.Name() == "postgres" {
		var locked []uuid.UUID
		if err := tx.WithContext(ctx).Raw(`SELECT control_id FROM control_status_evaluations
			WHERE tenant_id = ? AND control_id IN ? ORDER BY control_id FOR UPDATE`, tenantID, controlIDs).Scan(&locked).Error; err != nil {
			return fmt.Errorf("controlstatus.MarkControls: lock: %w", err)
		}
	}
	if err := tx.WithContext(ctx).Table(TableControlStatusEvaluations).
		Where("tenant_id = ? AND control_id IN ?", tenantID, controlIDs).
		Update("stale", true).Error; err != nil {
		return fmt.Errorf("controlstatus.MarkControls: %w", err)
	}
	return nil
}

// MarkImplementation marks the control of an implementation of the tenant.
func MarkImplementation(ctx context.Context, tx *gorm.DB, tenantID, implementationID uuid.UUID) error {
	if err := needTenant(tenantID); err != nil {
		return err
	}
	err := tx.WithContext(ctx).Exec(`UPDATE control_status_evaluations SET stale = ?
		WHERE tenant_id = ? AND control_id IN (
			SELECT i.control_id FROM control_implementations i WHERE i.tenant_id = ? AND i.id = ?)`,
		true, tenantID, tenantID, implementationID).Error
	if err != nil {
		return fmt.Errorf("controlstatus.MarkImplementation: %w", err)
	}
	return nil
}

// MarkTest marks the control of a test of the tenant, through the test's
// implementation. The soft-deleted rows are resolved too: a delete marks before
// or after its UPDATE alike.
func MarkTest(ctx context.Context, tx *gorm.DB, tenantID, testID uuid.UUID) error {
	if err := needTenant(tenantID); err != nil {
		return err
	}
	err := tx.WithContext(ctx).Exec(`UPDATE control_status_evaluations SET stale = ?
		WHERE tenant_id = ? AND control_id IN (
			SELECT i.control_id FROM control_tests t JOIN control_implementations i ON i.id = t.implementation_id AND i.tenant_id = t.tenant_id
			WHERE t.tenant_id = ? AND i.tenant_id = ? AND t.id = ?)`,
		true, tenantID, tenantID, tenantID, testID).Error
	if err != nil {
		return fmt.Errorf("controlstatus.MarkTest: %w", err)
	}
	return nil
}

// MarkEvidence marks every control an evidence of the tenant reaches through an
// evidenced_by edge: the control it is attached to, and the control of each test
// it backs through that test's implementation. A deleting write calls it before it
// removes the edges. The controls are resolved first and marked through
// MarkControls, so the rows are locked in a fixed order.
func MarkEvidence(ctx context.Context, tx *gorm.DB, tenantID, evidenceID uuid.UUID) error {
	if err := needTenant(tenantID); err != nil {
		return err
	}
	var ids []uuid.UUID
	err := tx.WithContext(ctx).Raw(`SELECT e.from_id FROM canonical_edges e
		WHERE e.tenant_id = ? AND e.verb = ? AND e.to_type = ? AND e.to_id = ? AND e.from_type = ?
		UNION
		SELECT i.control_id FROM canonical_edges e
		JOIN control_tests t ON t.id = e.from_id AND t.tenant_id = e.tenant_id
		JOIN control_implementations i ON i.id = t.implementation_id AND i.tenant_id = t.tenant_id
		WHERE e.tenant_id = ? AND e.verb = ? AND e.to_type = ? AND e.to_id = ? AND e.from_type = ?
		  AND t.tenant_id = ? AND i.tenant_id = ?`,
		tenantID, evidencedBy, evidenceType, evidenceID, controlType,
		tenantID, evidencedBy, evidenceType, evidenceID, testType,
		tenantID, tenantID).Scan(&ids).Error
	if err != nil {
		return fmt.Errorf("controlstatus.MarkEvidence: %w", err)
	}
	return MarkControls(ctx, tx, tenantID, ids...)
}
