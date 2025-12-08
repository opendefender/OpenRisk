package services

import (
	"context"
	"time"

	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// AnalyticsService handles risk and mitigation analytics
type AnalyticsService struct {
	db *gorm.DB
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(db *gorm.DB) *AnalyticsService {
	return &AnalyticsService{
		db: db,
	}
}

// RiskMetrics represents aggregated risk metrics
type RiskMetrics struct {
	TotalRisks       int64            `json:"total_risks"`
	ActiveRisks      int64            `json:"active_risks"`
	MitigatedRisks   int64            `json:"mitigated_risks"`
	AverageScore     float64          `json:"average_score"`
	HighRisks        int64            `json:"high_risks"`
	MediumRisks      int64            `json:"medium_risks"`
	LowRisks         int64            `json:"low_risks"`
	RisksByFramework map[string]int64 `json:"risks_by_framework"`
	RisksByStatus    map[string]int64 `json:"risks_by_status"`
	CreatedThisMonth int64            `json:"created_this_month"`
	UpdatedThisMonth int64            `json:"updated_this_month"`
}

// GetRiskMetrics returns aggregated risk metrics
func (s *AnalyticsService) GetRiskMetrics(ctx context.Context) (*RiskMetrics, error) {
	metrics := &RiskMetrics{
		RisksByFramework: make(map[string]int64),
		RisksByStatus:    make(map[string]int64),
	}

	// Total risks
	s.db.WithContext(ctx).Model(&domain.Risk{}).Count(&metrics.TotalRisks)

	// Active risks
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("status = ?", "active").
		Count(&metrics.ActiveRisks)

	// Mitigated risks
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("status = ?", "mitigated").
		Count(&metrics.MitigatedRisks)

	// Average score
	type scoreResult struct {
		Avg float64
	}
	var scoreRes scoreResult
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Select("AVG(score) as avg").
		Scan(&scoreRes)
	metrics.AverageScore = scoreRes.Avg

	// Risks by level
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("level = ?", "high").
		Count(&metrics.HighRisks)
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("level = ?", "medium").
		Count(&metrics.MediumRisks)
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("level = ?", "low").
		Count(&metrics.LowRisks)

	// Risks by framework
	type frameworkResult struct {
		Framework string
		Count     int64
	}
	var frameworks []frameworkResult
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Select("framework, COUNT(*) as count").
		Group("framework").
		Scan(&frameworks)
	for _, fw := range frameworks {
		metrics.RisksByFramework[fw.Framework] = fw.Count
	}

	// Risks by status
	type statusResult struct {
		Status string
		Count  int64
	}
	var statuses []statusResult
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statuses)
	for _, st := range statuses {
		metrics.RisksByStatus[st.Status] = st.Count
	}

	// Created this month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("created_at >= ?", monthStart).
		Count(&metrics.CreatedThisMonth)

	// Updated this month
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("updated_at >= ?", monthStart).
		Count(&metrics.UpdatedThisMonth)

	return metrics, nil
}

// RiskTrendPoint represents a point in a trend
type RiskTrendPoint struct {
	Date      time.Time `json:"date"`
	Count     int64     `json:"count"`
	AvgScore  float64   `json:"avg_score"`
	NewRisks  int64     `json:"new_risks"`
	Mitigated int64     `json:"mitigated"`
}

// GetRiskTrends returns risk trends over time (last 30 days)
func (s *AnalyticsService) GetRiskTrends(ctx context.Context, days int) ([]RiskTrendPoint, error) {
	var trends []RiskTrendPoint

	// Generate daily data for last N days
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		point := RiskTrendPoint{
			Date: startOfDay,
		}

		// Total risks as of this date
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at <= ?", endOfDay).
			Count(&point.Count)

		// Average score
		type scoreResult struct {
			Avg float64
		}
		var scoreRes scoreResult
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at <= ?", endOfDay).
			Select("AVG(score) as avg").
			Scan(&scoreRes)
		point.AvgScore = scoreRes.Avg

		// New risks created on this day
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
			Count(&point.NewRisks)

		// Mitigated on this day
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("status = ?", "mitigated").
			Where("updated_at >= ? AND updated_at < ?", startOfDay, endOfDay).
			Count(&point.Mitigated)

		trends = append(trends, point)
	}

	return trends, nil
}

// MitigationMetrics represents mitigation analytics
type MitigationMetrics struct {
	TotalMitigations     int64            `json:"total_mitigations"`
	CompletedMitigations int64            `json:"completed_mitigations"`
	PendingMitigations   int64            `json:"pending_mitigations"`
	OverdueMitigations   int64            `json:"overdue_mitigations"`
	CompletionRate       float64          `json:"completion_rate"`
	AvgCompletionDays    float64          `json:"avg_completion_days"`
	RisksWithMitigation  int64            `json:"risks_with_mitigation"`
	MitigationsByRisk    map[string]int64 `json:"mitigations_by_risk"`
}

// GetMitigationMetrics returns mitigation analytics
func (s *AnalyticsService) GetMitigationMetrics(ctx context.Context) (*MitigationMetrics, error) {
	metrics := &MitigationMetrics{
		MitigationsByRisk: make(map[string]int64),
	}

	// Total mitigations
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).Count(&metrics.TotalMitigations)

	// Completed mitigations
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status = ?", "completed").
		Count(&metrics.CompletedMitigations)

	// Pending mitigations
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status IN ?", []string{"open", "in_progress"}).
		Count(&metrics.PendingMitigations)

	// Overdue mitigations
	now := time.Now()
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status != ?", "completed").
		Where("due_date < ?", now).
		Count(&metrics.OverdueMitigations)

	// Completion rate
	if metrics.TotalMitigations > 0 {
		metrics.CompletionRate = float64(metrics.CompletedMitigations) / float64(metrics.TotalMitigations) * 100
	}

	// Average completion days
	type completionResult struct {
		AvgDays float64
	}
	var compRes completionResult
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status = ?", "completed").
		Select("AVG(EXTRACT(DAY FROM (completed_at - created_at))) as avg_days").
		Scan(&compRes)
	metrics.AvgCompletionDays = compRes.AvgDays

	// Risks with mitigation
	type riskMitigationResult struct {
		RiskID string
		Count  int64
	}
	var riskMitigations []riskMitigationResult
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Select("risk_id, COUNT(*) as count").
		Group("risk_id").
		Scan(&riskMitigations)
	metrics.RisksWithMitigation = int64(len(riskMitigations))

	return metrics, nil
}

// FrameworkAnalytics represents framework compliance analytics
type FrameworkAnalytics struct {
	Framework            string  `json:"framework"`
	TotalControls        int64   `json:"total_controls"`
	ImplementedControls  int64   `json:"implemented_controls"`
	CompliancePercentage float64 `json:"compliance_percentage"`
	AssociatedRisks      int64   `json:"associated_risks"`
	AverageRiskScore     float64 `json:"average_risk_score"`
}

// GetFrameworkAnalytics returns compliance analytics by framework
func (s *AnalyticsService) GetFrameworkAnalytics(ctx context.Context) ([]FrameworkAnalytics, error) {
	var analytics []FrameworkAnalytics

	type frameworkResult struct {
		Framework string
	}
	var frameworks []frameworkResult
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Distinct("framework").
		Scan(&frameworks)

	for _, fw := range frameworks {
		analytic := FrameworkAnalytics{Framework: fw.Framework}

		// Associated risks
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("framework = ?", fw.Framework).
			Count(&analytic.AssociatedRisks)

		// Average risk score
		type scoreResult struct {
			Avg float64
		}
		var scoreRes scoreResult
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("framework = ?", fw.Framework).
			Select("AVG(score) as avg").
			Scan(&scoreRes)
		analytic.AverageRiskScore = scoreRes.Avg

		analytics = append(analytics, analytic)
	}

	return analytics, nil
}

// DashboardSnapshot represents a complete dashboard snapshot
type DashboardSnapshot struct {
	Timestamp          time.Time            `json:"timestamp"`
	RiskMetrics        *RiskMetrics         `json:"risk_metrics"`
	MitigationMetrics  *MitigationMetrics   `json:"mitigation_metrics"`
	FrameworkAnalytics []FrameworkAnalytics `json:"framework_analytics"`
	Trends             []RiskTrendPoint     `json:"trends"`
}

// GetDashboardSnapshot returns a complete dashboard snapshot
func (s *AnalyticsService) GetDashboardSnapshot(ctx context.Context) (*DashboardSnapshot, error) {
	snapshot := &DashboardSnapshot{
		Timestamp: time.Now(),
	}

	// Get all metrics
	riskMetrics, err := s.GetRiskMetrics(ctx)
	if err != nil {
		return nil, err
	}
	snapshot.RiskMetrics = riskMetrics

	mitigationMetrics, err := s.GetMitigationMetrics(ctx)
	if err != nil {
		return nil, err
	}
	snapshot.MitigationMetrics = mitigationMetrics

	frameworkAnalytics, err := s.GetFrameworkAnalytics(ctx)
	if err != nil {
		return nil, err
	}
	snapshot.FrameworkAnalytics = frameworkAnalytics

	trends, err := s.GetRiskTrends(ctx, 30)
	if err != nil {
		return nil, err
	}
	snapshot.Trends = trends

	return snapshot, nil
}
