package services

import (
	"context"
	"fmt"
	"time"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// DashboardDataService aggregates data for the analytics dashboard
type DashboardDataService struct {
	db              *gorm.DB
	metricsRegistry *prometheus.Registry
}

// NewDashboardDataService creates a new dashboard data service
func NewDashboardDataService(db *gorm.DB, metricsRegistry *prometheus.Registry) *DashboardDataService {
	return &DashboardDataService{
		db:              db,
		metricsRegistry: metricsRegistry,
	}
}

// DashboardMetrics represents KPI metrics for the dashboard
type DashboardMetrics struct {
	AverageRiskScore  float64   `json:"average_risk_score"`
	TrendingUpPercent float64   `json:"trending_up_percent"`
	OverdueCount      int64     `json:"overdue_count"`
	SLAComplianceRate float64   `json:"sla_compliance_rate"`
	TotalRisks        int64     `json:"total_risks"`
	ActiveRisks       int64     `json:"active_risks"`
	MitigationRate    float64   `json:"mitigation_rate"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GetDashboardMetrics aggregates key metrics for the dashboard
func (s *DashboardDataService) GetDashboardMetrics(ctx context.Context) (*DashboardMetrics, error) {
	metrics := &DashboardMetrics{
		UpdatedAt: time.Now(),
	}

	// Get total and active risks
	s.db.WithContext(ctx).Model(&domain.Risk{}).Count(&metrics.TotalRisks)
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("status = ?", "active").
		Count(&metrics.ActiveRisks)

	// Calculate average risk score
	type scoreResult struct {
		AvgScore float64
	}
	var scoreRes scoreResult
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Select("COALESCE(AVG(score), 0) as avg_score").
		Scan(&scoreRes)
	metrics.AverageRiskScore = scoreRes.AvgScore

	// Calculate trending up percentage (risks with score increasing)
	if metrics.TotalRisks > 0 {
		var trendingUp int64
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("score > ? AND updated_at > ?", metrics.AverageRiskScore*0.9, time.Now().AddDate(0, 0, -7)).
			Count(&trendingUp)
		metrics.TrendingUpPercent = float64(trendingUp) / float64(metrics.TotalRisks) * 100
	}

	// Count overdue mitigations
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("due_date < ? AND status != ?", time.Now(), "completed").
		Count(&metrics.OverdueCount)

	// Calculate SLA compliance (mitigations completed on time / total mitigations)
	var totalMitigations int64
	var completedOnTime int64
	s.db.WithContext(ctx).Model(&domain.Mitigation{}).Count(&totalMitigations)
	if totalMitigations > 0 {
		s.db.WithContext(ctx).Model(&domain.Mitigation{}).
			Where("status = ? AND completed_at <= due_date", "completed").
			Count(&completedOnTime)
		metrics.SLAComplianceRate = float64(completedOnTime) / float64(totalMitigations) * 100
	}

	// Calculate mitigation rate
	var mitigatedRisks int64
	s.db.WithContext(ctx).Model(&domain.Risk{}).
		Where("status = ?", "mitigated").
		Count(&mitigatedRisks)
	if metrics.TotalRisks > 0 {
		metrics.MitigationRate = float64(mitigatedRisks) / float64(metrics.TotalRisks) * 100
	}

	return metrics, nil
}

// RiskTrendDataPoint represents a data point in risk trend
type RiskTrendDataPoint struct {
	Date      string  `json:"date"`
	Score     float64 `json:"score"`
	Count     int64   `json:"count"`
	NewRisks  int64   `json:"new_risks"`
	Mitigated int64   `json:"mitigated"`
}

// GetRiskTrends returns risk trends for the last 7 days
func (s *DashboardDataService) GetRiskTrends(ctx context.Context) ([]RiskTrendDataPoint, error) {
	var trends []RiskTrendDataPoint
	now := time.Now()

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		point := RiskTrendDataPoint{
			Date: startOfDay.Format("2006-01-02"),
		}

		// Total risks as of this date
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at <= ?", endOfDay).
			Count(&point.Count)

		// Average score as of this date
		type scoreResult struct {
			AvgScore float64
		}
		var scoreRes scoreResult
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at <= ?", endOfDay).
			Select("COALESCE(AVG(score), 0) as avg_score").
			Scan(&scoreRes)
		point.Score = scoreRes.AvgScore

		// New risks created this day
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
			Count(&point.NewRisks)

		// Risks mitigated this day
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("status = ? AND updated_at >= ? AND updated_at < ?", "mitigated", startOfDay, endOfDay).
			Count(&point.Mitigated)

		trends = append(trends, point)
	}

	return trends, nil
}

// RiskSeverityDistribution represents risk count by severity
type RiskSeverityDistribution struct {
	Critical int64 `json:"critical"`
	High     int64 `json:"high"`
	Medium   int64 `json:"medium"`
	Low      int64 `json:"low"`
}

// GetSeverityDistribution returns risk count by severity level
func (s *DashboardDataService) GetSeverityDistribution(ctx context.Context) (*RiskSeverityDistribution, error) {
	dist := &RiskSeverityDistribution{}

	// Map severity levels to counts
	severityLevels := map[string]*int64{
		"critical": &dist.Critical,
		"high":     &dist.High,
		"medium":   &dist.Medium,
		"low":      &dist.Low,
	}

	for severity, countPtr := range severityLevels {
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("severity = ?", severity).
			Count(countPtr)
	}

	return dist, nil
}

// MitigationStatus represents mitigation status count
type MitigationStatus struct {
	Completed  int64 `json:"completed"`
	InProgress int64 `json:"in_progress"`
	NotStarted int64 `json:"not_started"`
	Overdue    int64 `json:"overdue"`
}

// GetMitigationStatus returns mitigation count by status
func (s *DashboardDataService) GetMitigationStatus(ctx context.Context) (*MitigationStatus, error) {
	status := &MitigationStatus{}

	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status = ?", "completed").
		Count(&status.Completed)

	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status = ?", "in_progress").
		Count(&status.InProgress)

	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("status = ?", "not_started").
		Count(&status.NotStarted)

	s.db.WithContext(ctx).Model(&domain.Mitigation{}).
		Where("due_date < ? AND status != ?", time.Now(), "completed").
		Count(&status.Overdue)

	return status, nil
}

// TopRisk represents a risk with key details
type TopRisk struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Score           float64   `json:"score"`
	Severity        string    `json:"severity"`
	Status          string    `json:"status"`
	TrendPercent    float64   `json:"trend_percent"`
	LastUpdated     time.Time `json:"last_updated"`
	AssignedTeam    string    `json:"assigned_team,omitempty"`
	MitigationCount int64     `json:"mitigation_count"`
}

// GetTopRisks returns the top N risks by score
func (s *DashboardDataService) GetTopRisks(ctx context.Context, limit int) ([]TopRisk, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	var topRisks []TopRisk
	var risks []domain.Risk

	err := s.db.WithContext(ctx).
		Preload("Team").
		Order("score DESC").
		Limit(limit).
		Find(&risks).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch top risks: %w", err)
	}

	for _, risk := range risks {
		topRisk := TopRisk{
			ID:          risk.ID,
			Name:        risk.Name,
			Score:       risk.Score,
			Severity:    risk.Severity,
			Status:      risk.Status,
			LastUpdated: risk.UpdatedAt,
		}

		// Count mitigations for this risk
		s.db.WithContext(ctx).Model(&domain.Mitigation{}).
			Where("risk_id = ?", risk.ID).
			Count(&topRisk.MitigationCount)

		// Set assigned team if available
		if risk.Team != nil {
			topRisk.AssignedTeam = risk.Team.Name
		}

		// Calculate trend percentage (change from 7 days ago)
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		var historyScore float64
		s.db.WithContext(ctx).Model(&domain.Risk{}).
			Where("id = ? AND updated_at >= ?", risk.ID, sevenDaysAgo).
			Select("COALESCE(AVG(score), ?)", risk.Score).
			Scan(&historyScore)
		if historyScore > 0 {
			topRisk.TrendPercent = ((risk.Score - historyScore) / historyScore) * 100
		}

		topRisks = append(topRisks, topRisk)
	}

	return topRisks, nil
}

// MitigationProgress represents a mitigation with progress details
type MitigationProgress struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	Progress      int64     `json:"progress"` // 0-100
	DueDate       time.Time `json:"due_date"`
	Owner         string    `json:"owner,omitempty"`
	RiskID        string    `json:"risk_id"`
	RiskName      string    `json:"risk_name"`
	Cost          float64   `json:"cost"`
	LastUpdated   time.Time `json:"last_updated"`
	DaysRemaining int       `json:"days_remaining"`
}

// GetMitigationProgress returns mitigations with progress tracking
func (s *DashboardDataService) GetMitigationProgress(ctx context.Context, limit int) ([]MitigationProgress, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	var progressList []MitigationProgress
	var mitigations []domain.Mitigation

	err := s.db.WithContext(ctx).
		Preload("Risk").
		Order("due_date ASC").
		Limit(limit).
		Find(&mitigations).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch mitigation progress: %w", err)
	}

	for _, m := range mitigations {
		progress := MitigationProgress{
			ID:          m.ID,
			Name:        m.Name,
			Status:      m.Status,
			Progress:    m.Progress,
			DueDate:     m.DueDate,
			Cost:        m.Cost,
			LastUpdated: m.UpdatedAt,
		}

		// Set owner if available
		if m.Owner != nil {
			progress.Owner = m.Owner.Name
		}

		// Set risk details
		if m.Risk != nil {
			progress.RiskID = m.Risk.ID
			progress.RiskName = m.Risk.Name
		}

		// Calculate days remaining
		now := time.Now()
		if m.DueDate.After(now) {
			progress.DaysRemaining = int(m.DueDate.Sub(now).Hours() / 24)
		} else if m.Status != "completed" {
			progress.DaysRemaining = -int(now.Sub(m.DueDate).Hours() / 24) // negative = overdue
		}

		progressList = append(progressList, progress)
	}

	return progressList, nil
}

// DashboardAnalytics combines all dashboard data
type DashboardAnalytics struct {
	Metrics              *DashboardMetrics         `json:"metrics"`
	RiskTrends           []RiskTrendDataPoint      `json:"risk_trends"`
	SeverityDistribution *RiskSeverityDistribution `json:"severity_distribution"`
	MitigationStatus     *MitigationStatus         `json:"mitigation_status"`
	TopRisks             []TopRisk                 `json:"top_risks"`
	MitigationProgress   []MitigationProgress      `json:"mitigation_progress"`
	GeneratedAt          time.Time                 `json:"generated_at"`
}

// GetCompleteDashboardData aggregates all dashboard data in one call
func (s *DashboardDataService) GetCompleteDashboardData(ctx context.Context) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		GeneratedAt: time.Now(),
	}

	var err error

	// Get all data in parallel where possible
	// For now, sequential for simplicity
	if analytics.Metrics, err = s.GetDashboardMetrics(ctx); err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	if analytics.RiskTrends, err = s.GetRiskTrends(ctx); err != nil {
		return nil, fmt.Errorf("failed to get risk trends: %w", err)
	}

	if analytics.SeverityDistribution, err = s.GetSeverityDistribution(ctx); err != nil {
		return nil, fmt.Errorf("failed to get severity distribution: %w", err)
	}

	if analytics.MitigationStatus, err = s.GetMitigationStatus(ctx); err != nil {
		return nil, fmt.Errorf("failed to get mitigation status: %w", err)
	}

	if analytics.TopRisks, err = s.GetTopRisks(ctx, 5); err != nil {
		return nil, fmt.Errorf("failed to get top risks: %w", err)
	}

	if analytics.MitigationProgress, err = s.GetMitigationProgress(ctx, 10); err != nil {
		return nil, fmt.Errorf("failed to get mitigation progress: %w", err)
	}

	return analytics, nil
}
