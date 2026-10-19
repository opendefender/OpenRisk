// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormMFABackupCodePurgeRepository reads the ledger the universal-code purge
// writes (#1001, migrations/0078). It is a system job across tenants: each row
// carries its own tenant_id and the use case acts on that row alone.
type GormMFABackupCodePurgeRepository struct {
	db *gorm.DB
}

func NewGormMFABackupCodePurgeRepository(db *gorm.DB) *GormMFABackupCodePurgeRepository {
	return &GormMFABackupCodePurgeRepository{db: db}
}

// ClaimPending stamps up to limit unnotified rows and returns them, in one
// statement. SKIP LOCKED lets several replicas booting together split the
// work instead of mailing the same people. A database that never ran the SQL
// layer has no ledger, and nobody to tell.
func (r *GormMFABackupCodePurgeRepository) ClaimPending(ctx context.Context, limit int) ([]domain.MFABackupCodePurge, error) {
	var exists bool
	if err := r.db.WithContext(ctx).Raw(`SELECT to_regclass('mfa_backup_code_purges') IS NOT NULL`).Scan(&exists).Error; err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	var rows []struct {
		ID       uuid.UUID
		TenantID uuid.UUID
		UserID   uuid.UUID
	}
	err := r.db.WithContext(ctx).Raw(`
		UPDATE mfa_backup_code_purges
		   SET notified_at = now()
		 WHERE id IN (
		       SELECT id FROM mfa_backup_code_purges
		        WHERE notified_at IS NULL
		        ORDER BY purged_at, id
		        LIMIT ?
		          FOR UPDATE SKIP LOCKED)
		RETURNING id, tenant_id, user_id`, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.MFABackupCodePurge, len(rows))
	for i, row := range rows {
		out[i] = domain.MFABackupCodePurge{ID: row.ID, TenantID: row.TenantID, UserID: row.UserID}
	}
	return out, nil
}
