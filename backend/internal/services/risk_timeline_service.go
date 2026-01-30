package services

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// RiskTimelineService handles risk history and timeline operations
type RiskTimelineService struct {
	db gorm.DB
}

// NewRiskTimelineService creates a new risk timeline service
func NewRiskTimelineService() RiskTimelineService {
	return &RiskTimelineService{
		db: database.DB,
	}
}

// GetRiskTimeline retrieves the timeline/history for a specific risk
func (s RiskTimelineService) GetRiskTimeline(riskID uuid.UUID) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Where("risk_id = ?", riskID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get risk timeline: %w", err)
	}
	return history, nil
}

// GetRiskTimelineWithPagination retrieves paginated risk history
func (s RiskTimelineService) GetRiskTimelineWithPagination(riskID uuid.UUID, limit int, offset int) ([]domain.RiskHistory, int, error) {
	var history []domain.RiskHistory
	var total int

	query := s.db.Where("risk_id = ?", riskID)

	if err := query.Model(&domain.RiskHistory{}).Count(&total).Error; err != nil {
		return nil, , err
	}

	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&history).Error; err != nil {
		return nil, , fmt.Errorf("failed to get risk timeline: %w", err)
	}

	return history, total, nil
}

// GetRiskChangesByType retrieves history entries of a specific change type
func (s RiskTimelineService) GetRiskChangesByType(riskID uuid.UUID, changeType string) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, changeType).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get risk changes: %w", err)
	}
	return history, nil
}

// GetStatusChanges retrieves only status change events
func (s RiskTimelineService) GetStatusChanges(riskID uuid.UUID) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, "STATUS_CHANGE").
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetScoreChanges retrieves only score change events
func (s RiskTimelineService) GetScoreChanges(riskID uuid.UUID) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND change_type = ?", riskID, "SCORE_CHANGE").
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// ComputeRiskTrend analyzes the risk score trend over time
func (s RiskTimelineService) ComputeRiskTrend(riskID uuid.UUID) (map[string]interface{}, error) {
	history, err := s.GetRiskTimeline(riskID)
	if err != nil {
		return nil, err
	}

	if len(history) ==  {
		return map[string]interface{}{
			"trend":     "stable",
			"direction": "none",
			"change":    .,
		}, nil
	}

	// Compare oldest and newest scores
	oldest := history[len(history)-].Score
	newest := history[].Score
	change := newest - oldest
	pctChange := .

	if oldest !=  {
		pctChange = (change / oldest)  
	}

	trend := "stable"
	direction := "none"

	if change > . {
		trend = "increasing"
		direction = "up"
	} else if change < -. {
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
		"days_ago":   history[len(history)-].CreatedAt.Unix(),
	}, nil
}

// GetRecentChanges gets the most recent N changes across all risks
func (s RiskTimelineService) GetRecentChanges(limit int) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Order("created_at DESC").
		Limit(limit).
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetChangesSince gets all changes since a specific time
func (s RiskTimelineService) GetChangesSince(riskID uuid.UUID, sinceUnix int) ([]domain.RiskHistory, error) {
	var history []domain.RiskHistory
	if err := s.db.Where("risk_id = ? AND EXTRACT(EPOCH FROM created_at) > ?", riskID, sinceUnix).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}
