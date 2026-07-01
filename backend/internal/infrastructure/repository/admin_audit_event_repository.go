// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// AdminAuditEventRepository handles append-only admin audit trail operations
type AdminAuditEventRepository interface {
	// Log creates a new admin audit event (append-only operation)
	Log(ctx context.Context, event *domain.AdminAuditEvent) error

	// GetByID retrieves an audit event by ID (read-only)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.AdminAuditEvent, error)

	// ListByAdminUser lists all audit events for a specific admin user
	ListByAdminUser(ctx context.Context, adminUserID uuid.UUID, limit, offset int) ([]*domain.AdminAuditEvent, error)

	// ListByResource lists all audit events for a specific resource
	ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit, offset int) ([]*domain.AdminAuditEvent, error)

	// ListByAction lists all audit events for a specific action type
	ListByAction(ctx context.Context, action string, limit, offset int) ([]*domain.AdminAuditEvent, error)

	// Count returns total number of audit events
	Count(ctx context.Context) (int64, error)
}
