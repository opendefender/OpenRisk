// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlexception persists control exceptions (#1428, #555 slice 2).
//
// The row type here is the persistence shape of domain.ControlException. Every
// constraint migration 0087 declares is also declared as a check: or index tag,
// because a schema built by AutoMigrate alone (a compose boot never runs the SQL
// layer) must refuse the same rows.
//
// The table lives beside the control chain, not in controlchain: the chain's six
// tables are the ADR 0008 canonical records reconciled from the legacy tables,
// and controlchain.Tables() says so. An exception is a decision recorded about a
// control, not a canonical record.
package controlexception

import (
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// TableControlExceptions is the table of migration 0087.
const TableControlExceptions = "control_exceptions"

// Models lists the row types for AutoMigrate.
func Models() []interface{} { return []interface{}{&ExceptionRow{}} }

// ExceptionRow is domain.ControlException, persisted. Ids are assigned in Go.
// Time columns carry no type tag: gorm builds timestamptz on PostgreSQL and a
// datetime on sqlite, which the handler tests use.
type ExceptionRow struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID   uuid.UUID  `gorm:"type:uuid;not null;index:idx_control_exceptions_control,priority:1"`
	ControlID  uuid.UUID  `gorm:"type:uuid;not null;index:idx_control_exceptions_control,priority:2"`
	OwnerID    uuid.UUID  `gorm:"type:uuid;not null"`
	Reason     string     `gorm:"type:text;not null;check:chk_control_exceptions_reason,length(trim(reason)) > 0"`
	ApprovedBy uuid.UUID  `gorm:"type:uuid;not null"`
	StartsAt   time.Time  `gorm:"not null;check:chk_control_exceptions_window,expires_at > starts_at"`
	ExpiresAt  time.Time  `gorm:"not null;index:idx_control_exceptions_control,priority:3"`
	RevokedAt  *time.Time `gorm:"check:chk_control_exceptions_revocation,(revoked_at IS NULL) = (revoked_by IS NULL)"`
	RevokedBy  *uuid.UUID `gorm:"type:uuid"`
	// RevokeReason is empty, never NULL: a revocation without a reason is allowed.
	RevokeReason string `gorm:"type:text;not null;default:''"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName implements gorm's Tabler.
func (ExceptionRow) TableName() string { return TableControlExceptions }

func rowOf(e *domain.ControlException) *ExceptionRow {
	return &ExceptionRow{
		ID: e.ID, TenantID: e.TenantID, ControlID: e.ControlID, OwnerID: e.OwnerID,
		Reason: e.Reason, ApprovedBy: e.ApprovedBy, StartsAt: e.StartsAt, ExpiresAt: e.ExpiresAt,
		RevokedAt: e.RevokedAt, RevokedBy: e.RevokedBy, RevokeReason: e.RevokeReason,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func (r ExceptionRow) entity() domain.ControlException {
	return domain.ControlException{
		ID: r.ID, TenantID: r.TenantID, ControlID: r.ControlID, OwnerID: r.OwnerID,
		Reason: r.Reason, ApprovedBy: r.ApprovedBy, StartsAt: r.StartsAt, ExpiresAt: r.ExpiresAt,
		RevokedAt: r.RevokedAt, RevokedBy: r.RevokedBy, RevokeReason: r.RevokeReason,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
