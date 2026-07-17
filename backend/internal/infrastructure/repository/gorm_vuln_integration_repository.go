// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormVulnIntegrationRepository is the Postgres-backed store for vulnerability
// scanner connector configs and the tenant ITSM/ticketing config.
// ABSOLUTE RULE: every tenant-owned query filters by tenant_id. The single
// exception is GetIntegrationByWebhookToken, where the opaque token IS the
// tenant credential (documented on the interface).
type GormVulnIntegrationRepository struct {
	db *gorm.DB
}

func NewGormVulnIntegrationRepository(db *gorm.DB) *GormVulnIntegrationRepository {
	return &GormVulnIntegrationRepository{db: db}
}

var _ domain.VulnIntegrationRepository = (*GormVulnIntegrationRepository)(nil)

// withDerived sets the non-persisted HasCredentials flag from the ciphertext.
func withDerived(in *domain.VulnIntegration) {
	if in != nil {
		in.HasCredentials = in.EncryptedCredentials != ""
	}
}

// UpsertIntegration inserts or updates by (tenant, source): a tenant has at most
// one config per scanner source.
func (r *GormVulnIntegrationRepository) UpsertIntegration(ctx context.Context, in *domain.VulnIntegration) error {
	var existing domain.VulnIntegration
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source = ?", in.TenantID, in.Source).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(in).Error
	}
	if err != nil {
		return err
	}
	in.ID = existing.ID
	in.CreatedAt = existing.CreatedAt
	// Preserve last-pull telemetry unless the caller explicitly set it.
	if in.LastPullAt == nil {
		in.LastPullAt = existing.LastPullAt
		in.LastPullStatus = existing.LastPullStatus
		in.LastPullError = existing.LastPullError
		in.LastPullCount = existing.LastPullCount
	}
	return r.db.WithContext(ctx).Model(&existing).Select("*").
		Omit("id", "created_at", "deleted_at").Updates(in).Error
}

func (r *GormVulnIntegrationRepository) GetIntegration(ctx context.Context, id, tenantID uuid.UUID) (*domain.VulnIntegration, error) {
	var in domain.VulnIntegration
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&in).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	withDerived(&in)
	return &in, nil
}

func (r *GormVulnIntegrationRepository) GetIntegrationBySource(ctx context.Context, tenantID uuid.UUID, source domain.VulnSource) (*domain.VulnIntegration, error) {
	var in domain.VulnIntegration
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND source = ?", tenantID, source).First(&in).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	withDerived(&in)
	return &in, nil
}

func (r *GormVulnIntegrationRepository) GetIntegrationByWebhookToken(ctx context.Context, token string) (*domain.VulnIntegration, error) {
	if token == "" {
		return nil, nil
	}
	var in domain.VulnIntegration
	err := r.db.WithContext(ctx).
		Where("webhook_token = ? AND webhook_enabled = ? AND enabled = ?", token, true, true).
		First(&in).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	withDerived(&in)
	return &in, nil
}

func (r *GormVulnIntegrationRepository) ListIntegrations(ctx context.Context, tenantID uuid.UUID) ([]domain.VulnIntegration, error) {
	var rows []domain.VulnIntegration
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("source asc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		withDerived(&rows[i])
	}
	return rows, nil
}

// ListDueForPull returns enabled live-pull integrations whose schedule elapsed.
// The elapsed check is done in Go so it is DB-agnostic (also works in sqlite tests).
func (r *GormVulnIntegrationRepository) ListDueForPull(ctx context.Context, now time.Time) ([]domain.VulnIntegration, error) {
	var candidates []domain.VulnIntegration
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND live_pull_enabled = ? AND schedule_minutes > 0", true, true).
		Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	due := make([]domain.VulnIntegration, 0, len(candidates))
	for i := range candidates {
		c := candidates[i]
		if c.LastPullAt == nil || now.Sub(*c.LastPullAt) >= time.Duration(c.ScheduleMinutes)*time.Minute {
			withDerived(&c)
			due = append(due, c)
		}
	}
	return due, nil
}

func (r *GormVulnIntegrationRepository) DeleteIntegration(ctx context.Context, id, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.VulnIntegration{}).Error
}

// ---- Ticketing config (one per tenant) -----------------------------------

func (r *GormVulnIntegrationRepository) UpsertTicketing(ctx context.Context, in *domain.VulnTicketingConfig) error {
	var existing domain.VulnTicketingConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", in.TenantID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(in).Error
	}
	if err != nil {
		return err
	}
	in.ID = existing.ID
	in.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Model(&existing).Select("*").
		Omit("id", "created_at", "deleted_at").Updates(in).Error
}

func (r *GormVulnIntegrationRepository) GetTicketing(ctx context.Context, tenantID uuid.UUID) (*domain.VulnTicketingConfig, error) {
	var in domain.VulnTicketingConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&in).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	in.HasCredentials = in.EncryptedCredentials != ""
	return &in, nil
}

func (r *GormVulnIntegrationRepository) DeleteTicketing(ctx context.Context, tenantID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Delete(&domain.VulnTicketingConfig{}).Error
}
