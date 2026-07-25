// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/domain"
)

// RiskTimelineService handles risk history and timeline operations.
//
// RiskHistory rows carry no tenant_id of their own (they are children of a Risk),
// so every read MUST be gated by the parent risk's tenant. Without that gate any
// authenticated user could read another tenant's risk history — scores, statuses,
// who changed what — simply by knowing (or guessing) a risk UUID (RULE #2).
type RiskTimelineService struct {
	db *gorm.DB
}

// NewRiskTimelineService creates a new risk timeline service
func NewRiskTimelineService() *RiskTimelineService {
	return &RiskTimelineService{
		db: database.DB,
	}
}

// ownsRisk returns domain.ErrNotFound unless the risk exists within the tenant.
func (s *RiskTimelineService) ownsRisk(tenantID, riskID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return domain.ErrNotFound
	}
	var count int64
	if err := s.db.Model(&domain.Risk{}).
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify risk ownership: %w", err)
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetRiskTimeline retrieves the timeline/history for a specific risk (tenant-scoped)
func (s *RiskTimelineService) GetRiskTimeline(tenantID, riskID uuid.UUID) ([]*domain.RiskHistory, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, err
	}
	var history []*domain.RiskHistory
	if err := s.db.Where("risk_id = ?", riskID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get risk timeline: %w", err)
	}
	return history, nil
}

// GetRiskTimelineWithPagination retrieves paginated risk history (tenant-scoped)
func (s *RiskTimelineService) GetRiskTimelineWithPagination(tenantID, riskID uuid.UUID, limit int, offset int) ([]*domain.RiskHistory, int64, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, 0, err
	}
	var history []*domain.RiskHistory
	var total int64

	query := s.db.Where("risk_id = ?", riskID)

	if err := query.Model(&domain.RiskHistory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&history).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get risk timeline: %w", err)
	}

	return history, total, nil
}

// GetRiskChangesByType retrieves history entries of a specific change type (tenant-scoped)
func (s *RiskTimelineService) GetRiskChangesByType(tenantID, riskID uuid.UUID, changeType string) ([]*domain.RiskHistory, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, err
	}
	var history []*domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, changeType).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get risk changes: %w", err)
	}
	return history, nil
}

// GetStatusChanges retrieves only status change events (tenant-scoped)
func (s *RiskTimelineService) GetStatusChanges(tenantID, riskID uuid.UUID) ([]*domain.RiskHistory, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, err
	}
	var history []*domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, "STATUS_CHANGE").
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetScoreChanges retrieves only score change events (tenant-scoped)
func (s *RiskTimelineService) GetScoreChanges(tenantID, riskID uuid.UUID) ([]*domain.RiskHistory, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, err
	}
	var history []*domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, "SCORE_CHANGE").
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// ComputeRiskTrend analyzes the risk score trend over time (tenant-scoped)
func (s *RiskTimelineService) ComputeRiskTrend(tenantID, riskID uuid.UUID) (map[string]interface{}, error) {
	history, err := s.GetRiskTimeline(tenantID, riskID)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return map[string]interface{}{
			"trend":     "stable",
			"direction": "none",
			"change":    0.0,
		}, nil
	}

	// Compare oldest and newest scores
	oldest := history[len(history)-1].Score
	newest := history[0].Score
	change := newest - oldest
	pctChange := 0.0

	if oldest != 0 {
		pctChange = (change / oldest) * 100
	}

	trend := "stable"
	direction := "none"

	if change > 0.5 {
		trend = "increasing"
		direction = "up"
	} else if change < -0.5 {
		trend = "decreasing"
		direction = "down"
	}

	return map[string]interface{}{
		"trend":      trend,
		"direction":  direction,
		"change":     change,
		"pct_change": pctChange,
		"oldest":     oldest,
		"newest":     newest,
		"days_ago":   history[len(history)-1].CreatedAt.Unix(),
	}, nil
}

// GetRecentChanges gets the most recent N changes across the tenant's risks. The
// join to risks (which carry tenant_id) keeps this from leaking other tenants'
// activity — the previous implementation returned changes across ALL tenants.
func (s *RiskTimelineService) GetRecentChanges(tenantID uuid.UUID, limit int) ([]*domain.RiskHistory, error) {
	if tenantID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	var history []*domain.RiskHistory
	if err := s.db.
		Joins("JOIN risks ON risks.id = risk_histories.risk_id").
		Where("risks.tenant_id = ?", tenantID).
		Order("risk_histories.created_at DESC").
		Limit(limit).
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetChangesSince gets all changes since a specific time (tenant-scoped)
func (s *RiskTimelineService) GetChangesSince(tenantID, riskID uuid.UUID, sinceUnix int64) ([]*domain.RiskHistory, error) {
	if err := s.ownsRisk(tenantID, riskID); err != nil {
		return nil, err
	}
	var history []*domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND EXTRACT(EPOCH FROM created_at) > ?", riskID, sinceUnix).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// IsNotFound reports whether err is the tenant-scoped not-found sentinel, so
// handlers can map ownership failures to 404 without importing domain.
func IsNotFound(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}
