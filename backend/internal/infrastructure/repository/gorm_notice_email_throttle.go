// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormNoticeEmailThrottle keeps the notice e-mail throttle in Postgres
// (notice_email_throttle, #997), so every replica behind the HPA sees the same
// claims. tenant_id is part of the key on every statement.
type GormNoticeEmailThrottle struct {
	db *gorm.DB
}

func NewGormNoticeEmailThrottle(db *gorm.DB) *GormNoticeEmailThrottle {
	return &GormNoticeEmailThrottle{db: db}
}

// Claim takes the slot in one statement: a new key is inserted, a key whose
// last e-mail is older than window is restamped, and a recent one is left
// alone and returns no row. Two replicas racing on the same key cannot both
// get a row back.
func (r *GormNoticeEmailThrottle) Claim(ctx context.Context, key domain.NoticeEmailClaim, window time.Duration) (*domain.NoticeEmailClaim, error) {
	var stamps []time.Time
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO notice_email_throttle (tenant_id, user_id, type, resource_type, resource_id, sent_at)
		VALUES (?, ?, ?, ?, ?, now())
		ON CONFLICT (tenant_id, user_id, type, resource_type, resource_id)
		DO UPDATE SET sent_at = EXCLUDED.sent_at
		 WHERE notice_email_throttle.sent_at < now() - make_interval(secs => ?)
		RETURNING sent_at`,
		key.TenantID, key.UserID, string(key.Type), key.ResourceType, key.ResourceID, window.Seconds(),
	).Scan(&stamps).Error
	if err != nil {
		return nil, err
	}
	if len(stamps) == 0 {
		return nil, nil
	}
	claim := key
	claim.SentAt = stamps[0]
	return &claim, nil
}

// Release removes the claim, and only that claim: a row restamped since then
// belongs to a later e-mail and stays.
func (r *GormNoticeEmailThrottle) Release(ctx context.Context, claim domain.NoticeEmailClaim) error {
	return r.db.WithContext(ctx).Exec(`
		DELETE FROM notice_email_throttle
		 WHERE tenant_id = ? AND user_id = ? AND type = ? AND resource_type = ? AND resource_id = ?
		   AND sent_at = ?`,
		claim.TenantID, claim.UserID, string(claim.Type), claim.ResourceType, claim.ResourceID, claim.SentAt,
	).Error
}
