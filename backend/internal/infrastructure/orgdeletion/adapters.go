// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package orgdeletion (infrastructure) adapts existing services to the danger-zone
// ports: the tenant purge to the deletion Purger, and admin fan-out notifications
// to the AdminNotifier.
package orgdeletion

import (
	"context"

	"github.com/google/uuid"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// Purger adapts a tenant-delete function (e.g. TenantService.DeleteTenant) to the
// orgdeletion.Purger port.
type Purger struct {
	deleter func(ctx context.Context, tenant uuid.UUID) error
}

func NewPurger(deleter func(ctx context.Context, tenant uuid.UUID) error) *Purger {
	return &Purger{deleter: deleter}
}

func (p *Purger) PurgeTenant(ctx context.Context, tenant uuid.UUID) error {
	return p.deleter(ctx, tenant)
}

// AdminNotifier fans an in-app notification out to every active admin/root member
// of a tenant. Best-effort — a failed notification never blocks the flow.
type AdminNotifier struct {
	db    *gorm.DB
	notif *notificationapp.UseCase
}

func NewAdminNotifier(db *gorm.DB, notif *notificationapp.UseCase) *AdminNotifier {
	return &AdminNotifier{db: db, notif: notif}
}

func (n *AdminNotifier) NotifyAdmins(ctx context.Context, tenant uuid.UUID, subject, body string) error {
	if n.notif == nil {
		return nil
	}
	var members []domain.OrganizationMember
	if err := n.db.WithContext(ctx).
		Where("organization_id = ? AND is_active = ? AND role IN ?", tenant, true, []string{"admin", "root"}).
		Find(&members).Error; err != nil {
		return err
	}
	for _, m := range members {
		_ = n.notif.NotifyInApp(m.UserID, tenant, domain.NotificationType("org_deletion"), subject, body, nil, "organization")
	}
	return nil
}
