// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormAuthAuditLogRepository implements AuthAuditLogRepository using GORM
type GormAuthAuditLogRepository struct {
	db *gorm.DB
}

// NewGormAuthAuditLogRepository creates a new GORM auth audit log repository
func NewGormAuthAuditLogRepository(db *gorm.DB) *GormAuthAuditLogRepository {
	return &GormAuthAuditLogRepository{db: db}
}

// Create creates a new audit log entry (APPEND-ONLY - never update or delete)
func (r *GormAuthAuditLogRepository) Create(ctx context.Context, log *domain.AuthAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByUser gets audit logs for a specific user
func (r *GormAuthAuditLogRepository) GetByUser(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.AuthAuditLog, error) {
	var logs []*domain.AuthAuditLog
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	err := query.Find(&logs).Error
	return logs, err
}

// GetByTenant gets audit logs for a specific tenant
func (r *GormAuthAuditLogRepository) GetByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]*domain.AuthAuditLog, error) {
	var logs []*domain.AuthAuditLog
	query := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	err := query.Find(&logs).Error
	return logs, err
}

// GetByAction gets audit logs for a specific action
func (r *GormAuthAuditLogRepository) GetByAction(ctx context.Context, action string, limit int, offset int) ([]*domain.AuthAuditLog, error) {
	var logs []*domain.AuthAuditLog
	query := r.db.WithContext(ctx).
		Where("action = ?", action).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	err := query.Find(&logs).Error
	return logs, err
}

// GetRecent gets recent audit logs within a time range
func (r *GormAuthAuditLogRepository) GetRecent(ctx context.Context, since time.Time, limit int) ([]*domain.AuthAuditLog, error) {
	var logs []*domain.AuthAuditLog
	err := r.db.WithContext(ctx).
		Where("created_at >= ?", since).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetFailedAttempts gets failed authentication attempts for security monitoring
func (r *GormAuthAuditLogRepository) GetFailedAttempts(ctx context.Context, since time.Time, limit int) ([]*domain.AuthAuditLog, error) {
	var logs []*domain.AuthAuditLog
	err := r.db.WithContext(ctx).
		Where("success = false AND created_at >= ?", since).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
