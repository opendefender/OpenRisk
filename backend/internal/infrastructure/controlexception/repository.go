// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
)

// listLimit bounds one control's exception list. A control has a handful of
// exceptions over its life; the bound only stops a runaway.
const listLimit = 500

// AuditAppender appends an audit entry on the caller's transaction. Satisfied by
// repository.GormAuditChainRepository.
type AuditAppender interface {
	AppendTx(ctx context.Context, tx *gorm.DB, e *domain.AuditEvent) error
}

// Repository stores control exceptions. Every query names tenant_id, and the
// audit entry of a write is appended in the same transaction as the write: if the
// append fails the exception is not stored (or not revoked).
type Repository struct {
	db    *gorm.DB
	audit AuditAppender
}

// NewRepository builds the repository.
func NewRepository(db *gorm.DB, audit AuditAppender) *Repository {
	return &Repository{db: db, audit: audit}
}

func (r *Repository) supportsRowLock() bool { return r.db.Dialector.Name() == "postgres" }

// ControlExists reports whether the canonical control exists, undeleted, in the
// tenant. A control of another tenant reads as absent.
func (r *Repository) ControlExists(ctx context.Context, tenantID, controlID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, domain.ErrForbidden
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&controlchain.ControlRow{}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, controlID).
		Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("controlexception.ControlExists: %w", err)
	}
	return n > 0, nil
}

// Create stores the exception and its audit entry in one transaction.
func (r *Repository) Create(ctx context.Context, ex *domain.ControlException, audit *domain.AuditEvent) error {
	if ex == nil || ex.TenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rowOf(ex)).Error; err != nil {
			return err
		}
		// An exception starts or ends a waiver of the control's status (#555).
		if err := controlstatus.MarkControls(ctx, tx, ex.TenantID, ex.ControlID); err != nil {
			return err
		}
		return r.audit.AppendTx(ctx, tx, audit)
	})
	if err != nil {
		return fmt.Errorf("controlexception.Create: %w", err)
	}
	return nil
}

// ListByControl returns the control's exceptions in the tenant, newest first.
func (r *Repository) ListByControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.ControlException, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var rows []ExceptionRow
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND control_id = ?", tenantID, controlID).
		Order("created_at DESC, id DESC").Limit(listLimit).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("controlexception.ListByControl: %w", err)
	}
	out := make([]domain.ControlException, len(rows))
	for i := range rows {
		out[i] = rows[i].entity()
	}
	return out, nil
}

// Revoke loads the exception under lock (tenant_id and id), lets apply decide and
// mutate it, then stores the result and the audit entry apply returned in one
// transaction. An id that does not exist in the tenant, including one that
// exists in another, is ErrNotFound. An error from apply rolls everything back.
func (r *Repository) Revoke(ctx context.Context, tenantID, id uuid.UUID,
	apply func(current *domain.ControlException) (*domain.AuditEvent, error),
) (*domain.ControlException, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrForbidden
	}
	var out domain.ControlException
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("tenant_id = ? AND id = ?", tenantID, id)
		if r.supportsRowLock() {
			// Two concurrent revokes: the second waits, then reads the revoked row.
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var row ExceptionRow
		if err := q.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ControlExceptionError(domain.ErrNotFound, domain.CtlExcNotFound)
			}
			return err
		}
		cur := row.entity()
		ev, err := apply(&cur)
		if err != nil {
			return err
		}
		res := tx.Model(&ExceptionRow{}).
			Where("tenant_id = ? AND id = ?", tenantID, id).
			Updates(map[string]any{
				"revoked_at":    cur.RevokedAt,
				"revoked_by":    cur.RevokedBy,
				"revoke_reason": cur.RevokeReason,
				"updated_at":    cur.UpdatedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ControlExceptionError(domain.ErrNotFound, domain.CtlExcNotFound)
		}
		if err := controlstatus.MarkControls(ctx, tx, tenantID, cur.ControlID); err != nil {
			return err
		}
		if err := r.audit.AppendTx(ctx, tx, ev); err != nil {
			return err
		}
		out = cur
		return nil
	})
	if err != nil {
		var app *domain.AppError
		if errors.As(err, &app) {
			return nil, err
		}
		return nil, fmt.Errorf("controlexception.Revoke: %w", err)
	}
	return &out, nil
}
