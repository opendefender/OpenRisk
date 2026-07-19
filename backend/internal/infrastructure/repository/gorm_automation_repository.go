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

// GormAutomationRuleRepository is the Postgres-backed store for SOAR rules.
// ABSOLUTE RULE: every query filters by tenant_id.
type GormAutomationRuleRepository struct{ db *gorm.DB }

func NewGormAutomationRuleRepository(db *gorm.DB) *GormAutomationRuleRepository {
	return &GormAutomationRuleRepository{db: db}
}

var _ domain.AutomationRuleRepository = (*GormAutomationRuleRepository)(nil)

func (r *GormAutomationRuleRepository) Create(ctx context.Context, rule *domain.AutomationRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *GormAutomationRuleRepository) Update(ctx context.Context, rule *domain.AutomationRule) error {
	res := r.db.WithContext(ctx).
		Model(&domain.AutomationRule{}).
		Where("id = ? AND tenant_id = ?", rule.ID, rule.TenantID).
		Select("name", "description", "enabled", "trigger", "conditions", "actions", "sla", "priority", "updated_at").
		Updates(map[string]interface{}{
			"name":        rule.Name,
			"description": rule.Description,
			"enabled":     rule.Enabled,
			"trigger":     rule.Trigger,
			"conditions":  rule.Conditions,
			"actions":     rule.Actions,
			"sla":         rule.SLA,
			"priority":    rule.Priority,
			"updated_at":  time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("automation rule", rule.ID)
	}
	return nil
}

func (r *GormAutomationRuleRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.AutomationRule, error) {
	var rule domain.AutomationRule
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&rule).Error
	if err == gorm.ErrRecordNotFound {
		return nil, domain.NewNotFoundError("automation rule", id)
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *GormAutomationRuleRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.AutomationRule, error) {
	var rules []domain.AutomationRule
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("priority ASC, created_at DESC").
		Find(&rules).Error
	return rules, err
}

func (r *GormAutomationRuleRepository) ListEnabledByTrigger(ctx context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger) ([]domain.AutomationRule, error) {
	var rules []domain.AutomationRule
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND trigger = ? AND enabled = ?", tenantID, trigger, true).
		Order("priority ASC, created_at ASC").
		Find(&rules).Error
	return rules, err
}

func (r *GormAutomationRuleRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.AutomationRule{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("automation rule", id)
	}
	return nil
}

func (r *GormAutomationRuleRepository) RecordTriggered(ctx context.Context, id, tenantID uuid.UUID, at time.Time) error {
	// Best-effort counter bump; never blocks the engine.
	return r.db.WithContext(ctx).
		Model(&domain.AutomationRule{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(map[string]interface{}{
			"last_triggered_at": at,
			"trigger_count":     gorm.Expr("trigger_count + 1"),
		}).Error
}

// GormAutomationExecutionRepository stores execution audit records.
type GormAutomationExecutionRepository struct{ db *gorm.DB }

func NewGormAutomationExecutionRepository(db *gorm.DB) *GormAutomationExecutionRepository {
	return &GormAutomationExecutionRepository{db: db}
}

var _ domain.AutomationExecutionRepository = (*GormAutomationExecutionRepository)(nil)

func (r *GormAutomationExecutionRepository) Create(ctx context.Context, e *domain.AutomationExecution) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *GormAutomationExecutionRepository) Update(ctx context.Context, e *domain.AutomationExecution) error {
	res := r.db.WithContext(ctx).
		Model(&domain.AutomationExecution{}).
		Where("id = ? AND tenant_id = ?", e.ID, e.TenantID).
		Updates(map[string]interface{}{
			"status":      e.Status,
			"steps":       e.Steps,
			"error":       e.Error,
			"subject":     e.Subject,
			"severity":    e.Severity,
			"finished_at": e.FinishedAt,
		})
	return res.Error
}

func (r *GormAutomationExecutionRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.AutomationExecution, error) {
	var e domain.AutomationExecution
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&e).Error
	if err == gorm.ErrRecordNotFound {
		return nil, domain.NewNotFoundError("automation execution", id)
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *GormAutomationExecutionRepository) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.AutomationExecution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []domain.AutomationExecution
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("started_at DESC").
		Limit(limit).Offset(offset).
		Find(&out).Error
	return out, err
}

func (r *GormAutomationExecutionRepository) ListByRule(ctx context.Context, ruleID, tenantID uuid.UUID, limit int) ([]domain.AutomationExecution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []domain.AutomationExecution
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND rule_id = ?", tenantID, ruleID).
		Order("started_at DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// GormSLATrackerRepository stores SLA countdowns.
type GormSLATrackerRepository struct{ db *gorm.DB }

func NewGormSLATrackerRepository(db *gorm.DB) *GormSLATrackerRepository {
	return &GormSLATrackerRepository{db: db}
}

var _ domain.SLATrackerRepository = (*GormSLATrackerRepository)(nil)

func (r *GormSLATrackerRepository) Create(ctx context.Context, t *domain.SLATracker) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GormSLATrackerRepository) Update(ctx context.Context, t *domain.SLATracker) error {
	res := r.db.WithContext(ctx).
		Model(&domain.SLATracker{}).
		Where("id = ? AND tenant_id = ?", t.ID, t.TenantID).
		Updates(map[string]interface{}{
			"status":           t.Status,
			"escalation_level": t.EscalationLevel,
			"escalated_at":     t.EscalatedAt,
			"closed_at":        t.ClosedAt,
			"updated_at":       time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError("sla tracker", t.ID)
	}
	return nil
}

func (r *GormSLATrackerRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.SLATracker, error) {
	var t domain.SLATracker
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, domain.NewNotFoundError("sla tracker", id)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *GormSLATrackerRepository) ListOpen(ctx context.Context, tenantID uuid.UUID) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status IN ?", tenantID, []domain.SLAStatus{domain.SLAOpen, domain.SLABreached, domain.SLAEscalated}).
		Order("due_at ASC").
		Find(&out).Error
	return out, err
}

// ListBreaching runs cross-tenant on the scheduler: still-open trackers whose
// escalate_at has elapsed. Each row carries its own tenant_id.
func (r *GormSLATrackerRepository) ListBreaching(ctx context.Context, now time.Time) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	err := r.db.WithContext(ctx).
		Where("status IN ? AND escalate_at IS NOT NULL AND escalate_at <= ?",
			[]domain.SLAStatus{domain.SLAOpen, domain.SLABreached}, now).
		Order("escalate_at ASC").
		Limit(500).
		Find(&out).Error
	return out, err
}

func (r *GormSLATrackerRepository) ListOpenByRisk(ctx context.Context, tenantID, riskID uuid.UUID) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND risk_id = ? AND status IN ?",
			tenantID, riskID, []domain.SLAStatus{domain.SLAOpen, domain.SLABreached, domain.SLAEscalated}).
		Find(&out).Error
	return out, err
}

// GormAutomationChannelRepository stores the tenant outbound-channel config.
type GormAutomationChannelRepository struct{ db *gorm.DB }

func NewGormAutomationChannelRepository(db *gorm.DB) *GormAutomationChannelRepository {
	return &GormAutomationChannelRepository{db: db}
}

var _ domain.AutomationChannelRepository = (*GormAutomationChannelRepository)(nil)

func channelDerived(c *domain.AutomationChannelConfig) {
	if c != nil {
		c.HasSlack = c.SlackWebhookURL != ""
		c.HasTeams = c.TeamsWebhookURL != ""
	}
}

func (r *GormAutomationChannelRepository) Upsert(ctx context.Context, c *domain.AutomationChannelConfig) error {
	var existing domain.AutomationChannelConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", c.TenantID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		if createErr := r.db.WithContext(ctx).Create(c).Error; createErr != nil {
			return createErr
		}
		channelDerived(c)
		return nil
	}
	if err != nil {
		return err
	}
	c.ID = existing.ID
	c.CreatedAt = existing.CreatedAt
	// Preserve stored webhook URLs when the caller sends blanks (write-only fields).
	if c.SlackWebhookURL == "" {
		c.SlackWebhookURL = existing.SlackWebhookURL
	}
	if c.TeamsWebhookURL == "" {
		c.TeamsWebhookURL = existing.TeamsWebhookURL
	}
	if err := r.db.WithContext(ctx).Model(&domain.AutomationChannelConfig{}).
		Where("tenant_id = ?", c.TenantID).
		Updates(map[string]interface{}{
			"slack_enabled":     c.SlackEnabled,
			"slack_webhook_url": c.SlackWebhookURL,
			"teams_enabled":     c.TeamsEnabled,
			"teams_webhook_url": c.TeamsWebhookURL,
			"email_enabled":     c.EmailEnabled,
			"default_email":     c.DefaultEmail,
			"updated_at":        time.Now(),
		}).Error; err != nil {
		return err
	}
	channelDerived(c)
	return nil
}

func (r *GormAutomationChannelRepository) Get(ctx context.Context, tenantID uuid.UUID) (*domain.AutomationChannelConfig, error) {
	var c domain.AutomationChannelConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		// Absent config is not an error — the tenant simply has no channels yet.
		return &domain.AutomationChannelConfig{TenantID: tenantID, EmailEnabled: true}, nil
	}
	if err != nil {
		return nil, err
	}
	channelDerived(&c)
	return &c, nil
}

func (r *GormSLATrackerRepository) ListOpenLinkedToRisk(ctx context.Context) ([]domain.SLATracker, error) {
	var out []domain.SLATracker
	err := r.db.WithContext(ctx).
		Where("risk_id IS NOT NULL AND status IN ?",
			[]domain.SLAStatus{domain.SLAOpen, domain.SLABreached, domain.SLAEscalated}).
		Limit(500).
		Find(&out).Error
	return out, err
}

func (r *GormSLATrackerRepository) Stats(ctx context.Context, tenantID uuid.UUID) (domain.SLAStats, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.WithContext(ctx).
		Model(&domain.SLATracker{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ?", tenantID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return domain.SLAStats{}, err
	}
	var s domain.SLAStats
	for _, row := range rows {
		switch domain.SLAStatus(row.Status) {
		case domain.SLAOpen:
			s.Open = row.Count
		case domain.SLABreached:
			s.Breached = row.Count
		case domain.SLAEscalated:
			s.Escalated = row.Count
		case domain.SLAMet:
			s.Met = row.Count
		case domain.SLAClosed:
			s.Closed = row.Count
		}
	}
	return s, nil
}
