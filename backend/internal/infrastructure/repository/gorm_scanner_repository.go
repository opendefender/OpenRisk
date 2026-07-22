// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// ScanConfig
// ---------------------------------------------------------------------------

// GormScanConfigRepository implements domain.ScanConfigRepository.
// ABSOLUTE RULE: filter by tenant_id on EVERY query.
type GormScanConfigRepository struct{ db *gorm.DB }

func NewGormScanConfigRepository(db *gorm.DB) *GormScanConfigRepository {
	return &GormScanConfigRepository{db: db}
}

func (r *GormScanConfigRepository) Create(ctx context.Context, cfg *domain.ScanConfig) error {
	if cfg.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *GormScanConfigRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ScanConfig, error) {
	var cfg domain.ScanConfig
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get scan config: %w", err)
	}
	return &cfg, nil
}

func (r *GormScanConfigRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.ScanConfig, error) {
	var cfgs []domain.ScanConfig
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&cfgs).Error
	return cfgs, err
}

// Update persists mutable fields. Uses Model()+Where()+Updates() (not Save())
// so the tenant_id WHERE clause is actually honored — see GormAssetRepository.
// Select is explicit so a zero EncryptedCredentials never blanks stored creds
// unless intentionally rotated.
func (r *GormScanConfigRepository) Update(ctx context.Context, cfg *domain.ScanConfig) error {
	if cfg.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	result := r.db.WithContext(ctx).
		Model(&domain.ScanConfig{}).
		Where("id = ? AND tenant_id = ?", cfg.ID, cfg.TenantID).
		Select("name", "enabled", "encrypted_credentials", "regions", "targets", "agent_ids", "options", "schedule_minutes", "next_run_at").
		Updates(cfg)
	if result.Error != nil {
		return fmt.Errorf("failed to update scan config: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("scan config not found")
	}
	return nil
}

// ListDueScheduled returns enabled recurring configs due to run. NOT
// tenant-scoped by design — the scheduler worker runs globally.
func (r *GormScanConfigRepository) ListDueScheduled(ctx context.Context, now time.Time) ([]domain.ScanConfig, error) {
	var cfgs []domain.ScanConfig
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND schedule_minutes > 0 AND (next_run_at IS NULL OR next_run_at <= ?)", true, now).
		Find(&cfgs).Error
	return cfgs, err
}

// UpdateNextRun advances a config's schedule bookkeeping (tenant-scoped).
func (r *GormScanConfigRepository) UpdateNextRun(ctx context.Context, id, tenantID uuid.UUID, lastRun, nextRun time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&domain.ScanConfig{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{"last_run_at": lastRun, "next_run_at": nextRun})
	if result.Error != nil {
		return fmt.Errorf("failed to update next run: %w", result.Error)
	}
	return nil
}

func (r *GormScanConfigRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ScanConfig{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete scan config: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("scan config not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// ScannerAgent
// ---------------------------------------------------------------------------

// GormScannerAgentRepository implements domain.ScannerAgentRepository.
type GormScannerAgentRepository struct{ db *gorm.DB }

func NewGormScannerAgentRepository(db *gorm.DB) *GormScannerAgentRepository {
	return &GormScannerAgentRepository{db: db}
}

func (r *GormScannerAgentRepository) Create(ctx context.Context, agent *domain.ScannerAgent) error {
	if agent.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(agent).Error
}

func (r *GormScannerAgentRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ScannerAgent, error) {
	var agent domain.ScannerAgent
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&agent).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return &agent, nil
}

// GetByTokenHash resolves an agent from its scoped-token hash. This runs BEFORE
// we know the tenant (the agent authenticates by token), so it is intentionally
// not tenant-scoped — but revoked agents are excluded so a rotated/revoked token
// can never authenticate.
func (r *GormScannerAgentRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.ScannerAgent, error) {
	if tokenHash == "" {
		return nil, nil
	}
	var agent domain.ScannerAgent
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND status <> ?", tokenHash, domain.AgentRevoked).
		First(&agent).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get agent by token: %w", err)
	}
	return &agent, nil
}

func (r *GormScannerAgentRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.ScannerAgent, error) {
	var agents []domain.ScannerAgent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("registered_at DESC").
		Find(&agents).Error
	return agents, err
}

func (r *GormScannerAgentRepository) Update(ctx context.Context, agent *domain.ScannerAgent) error {
	if agent.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	result := r.db.WithContext(ctx).
		Model(&domain.ScannerAgent{}).
		Where("id = ? AND tenant_id = ?", agent.ID, agent.TenantID).
		Select("name", "version", "status", "last_heartbeat", "ip", "hostname",
			"os", "token_hash", "push_secret_enc", "last_scan_job_id", "token_rotated_at").
		Updates(agent)
	if result.Error != nil {
		return fmt.Errorf("failed to update agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}

func (r *GormScannerAgentRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.ScannerAgent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// ScanJob
// ---------------------------------------------------------------------------

// GormScanJobRepository implements domain.ScanJobRepository.
type GormScanJobRepository struct{ db *gorm.DB }

func NewGormScanJobRepository(db *gorm.DB) *GormScanJobRepository {
	return &GormScanJobRepository{db: db}
}

func (r *GormScanJobRepository) Create(ctx context.Context, job *domain.ScanJob) error {
	if job.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *GormScanJobRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ScanJob, error) {
	var job domain.ScanJob
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get scan job: %w", err)
	}
	return &job, nil
}

func (r *GormScanJobRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.ScanJob, error) {
	var jobs []domain.ScanJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

func (r *GormScanJobRepository) ListByStatus(ctx context.Context, tenantID uuid.UUID, status domain.ScanJobStatus) ([]domain.ScanJob, error) {
	var jobs []domain.ScanJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, status).
		Order("created_at ASC").
		Find(&jobs).Error
	return jobs, err
}

func (r *GormScanJobRepository) CountActiveByTenant(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&domain.ScanJob{}).
		Where("tenant_id = ? AND status IN ?", tenantID,
			[]domain.ScanJobStatus{domain.ScanClaimed, domain.ScanRunning}).
		Count(&n).Error
	return n, err
}

// Update persists the mutable lifecycle fields of a job.
func (r *GormScanJobRepository) Update(ctx context.Context, job *domain.ScanJob) error {
	if job.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	result := r.db.WithContext(ctx).
		Model(&domain.ScanJob{}).
		Where("id = ? AND tenant_id = ?", job.ID, job.TenantID).
		Select("status", "claimed_by_agent", "preview_key", "assets_found",
			"findings_found", "error", "started_at", "completed_at").
		Updates(job)
	if result.Error != nil {
		return fmt.Errorf("failed to update scan job: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("scan job not found")
	}
	return nil
}
