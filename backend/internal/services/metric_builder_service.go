package services

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/models"
	"gorm.io/datatypes"
)

// MetricBuilderService handles custom metric creation and calculation
type MetricBuilderService struct {
	db *database.Database
}

// NewMetricBuilderService creates a new metric builder service
func NewMetricBuilderService(db *database.Database) *MetricBuilderService {
	return &MetricBuilderService{
		db: db,
	}
}

// CreateCustomMetric creates a new custom metric definition
func (s *MetricBuilderService) CreateCustomMetric(tenantID string, def models.MetricDefinition, createdBy string) (*models.CustomMetric, error) {
	// Validate metric type
	validTypes := map[string]bool{"count": true, "average": true, "sum": true, "percentage": true}
	if !validTypes[def.MetricType] {
		return nil, fmt.Errorf("invalid metric type: %s", def.MetricType)
	}

	// Validate aggregation
	validAgg := map[string]bool{"daily": true, "weekly": true, "monthly": true, "yearly": true}
	if !validAgg[def.Aggregation] {
		return nil, fmt.Errorf("invalid aggregation: %s", def.Aggregation)
	}

	// Convert filters to JSON
	filterBytes, err := datatypes.JSONType(fmt.Sprintf(`%v`, def.Filters)).MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal filters: %w", err)
	}

	metric := &models.CustomMetric{
		TenantID:    tenantID,
		Name:        def.Name,
		Description: def.Description,
		MetricType:  def.MetricType,
		Formula:     def.Formula,
		DataSource:  def.DataSource,
		Filters:     filterBytes,
		Aggregation: def.Aggregation,
		IsActive:    true,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}

	if err := database.DB.Create(metric).Error; err != nil {
		log.Printf("Error creating custom metric: %v", err)
		return nil, fmt.Errorf("failed to create metric: %w", err)
	}

	return metric, nil
}

// GetCustomMetric retrieves a custom metric by ID
func (s *MetricBuilderService) GetCustomMetric(tenantID string, metricID uint) (*models.CustomMetric, error) {
	var metric models.CustomMetric
	if err := database.DB.Where("tenant_id = ? AND id = ?", tenantID, metricID).First(&metric).Error; err != nil {
		return nil, fmt.Errorf("metric not found: %w", err)
	}
	return &metric, nil
}

// ListCustomMetrics lists all custom metrics for a tenant
func (s *MetricBuilderService) ListCustomMetrics(tenantID string) ([]models.CustomMetric, error) {
	var metrics []models.CustomMetric
	if err := database.DB.Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("created_at DESC").
		Find(&metrics).Error; err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}
	return metrics, nil
}

// UpdateCustomMetric updates a custom metric definition
func (s *MetricBuilderService) UpdateCustomMetric(tenantID string, metricID uint, def models.MetricDefinition) (*models.CustomMetric, error) {
	metric, err := s.GetCustomMetric(tenantID, metricID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":        def.Name,
		"description": def.Description,
		"formula":     def.Formula,
		"updated_at":  time.Now(),
	}

	if err := database.DB.Model(metric).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update metric: %w", err)
	}

	return metric, nil
}

// DeleteCustomMetric soft deletes a custom metric
func (s *MetricBuilderService) DeleteCustomMetric(tenantID string, metricID uint) error {
	if err := database.DB.Where("tenant_id = ? AND id = ?", tenantID, metricID).
		Delete(&models.CustomMetric{}).Error; err != nil {
		return fmt.Errorf("failed to delete metric: %w", err)
	}
	return nil
}

// CalculateMetricValue calculates the current value for a metric
func (s *MetricBuilderService) CalculateMetricValue(metric *models.CustomMetric) (float64, error) {
	var value float64

	switch metric.MetricType {
	case "count":
		// Count risks matching criteria
		var count int64
		if err := database.DB.Model(&models.Risk{}).
			Where("tenant_id = ?", metric.TenantID).
			Count(&count).Error; err != nil {
			return 0, fmt.Errorf("failed to count: %w", err)
		}
		value = float64(count)

	case "average":
		// Calculate average score
		var avgScore float64
		if err := database.DB.Model(&models.Risk{}).
			Where("tenant_id = ?", metric.TenantID).
			Select("COALESCE(AVG(score), 0)").
			Row().Scan(&avgScore); err != nil {
			return 0, fmt.Errorf("failed to calculate average: %w", err)
		}
		value = avgScore

	case "sum":
		// Sum of values
		var sumVal float64
		if err := database.DB.Model(&models.Risk{}).
			Where("tenant_id = ?", metric.TenantID).
			Select("COALESCE(SUM(CAST(score AS FLOAT)), 0)").
			Row().Scan(&sumVal); err != nil {
			return 0, fmt.Errorf("failed to calculate sum: %w", err)
		}
		value = sumVal

	case "percentage":
		// Calculate percentage
		var total, critical int64
		database.DB.Model(&models.Risk{}).
			Where("tenant_id = ?", metric.TenantID).
			Count(&total)
		database.DB.Model(&models.Risk{}).
			Where("tenant_id = ? AND severity = ?", metric.TenantID, "critical").
			Count(&critical)

		if total > 0 {
			value = (float64(critical) / float64(total)) * 100
		}
	}

	return value, nil
}

// RecordMetricValue records a metric value with timestamp
func (s *MetricBuilderService) RecordMetricValue(metricID uint, tenantID string, value float64) (*models.MetricValue, error) {
	metricValue := &models.MetricValue{
		MetricID:  metricID,
		TenantID:  tenantID,
		Value:     value,
		Timestamp: time.Now().UTC(),
	}

	if err := database.DB.Create(metricValue).Error; err != nil {
		return nil, fmt.Errorf("failed to record metric value: %w", err)
	}

	return metricValue, nil
}

// GetMetricHistory retrieves historical values for a metric
func (s *MetricBuilderService) GetMetricHistory(tenantID string, metricID uint, days int) ([]models.MetricValue, error) {
	var history []models.MetricValue
	startDate := time.Now().AddDate(0, 0, -days)

	if err := database.DB.Where("tenant_id = ? AND metric_id = ? AND timestamp >= ?",
		tenantID, metricID, startDate).
		Order("timestamp ASC").
		Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get metric history: %w", err)
	}

	return history, nil
}

// CalculateTrend analyzes metric trend
func (s *MetricBuilderService) CalculateTrend(history []models.MetricValue) string {
	if len(history) < 2 {
		return "stable"
	}

	first := history[0].Value
	last := history[len(history)-1].Value

	if math.Abs(last-first) < 0.01 {
		return "stable"
	} else if last > first {
		return "up"
	}
	return "down"
}

// CalculateChange computes percentage change
func (s *MetricBuilderService) CalculateChange(history []models.MetricValue) float64 {
	if len(history) < 2 || history[0].Value == 0 {
		return 0
	}

	first := history[0].Value
	last := history[len(history)-1].Value

	return ((last - first) / first) * 100
}

// GetCalculatedMetric returns a complete calculated metric with history
func (s *MetricBuilderService) GetCalculatedMetric(tenantID string, metricID uint) (*models.CalculatedMetric, error) {
	metric, err := s.GetCustomMetric(tenantID, metricID)
	if err != nil {
		return nil, err
	}

	currentValue, err := s.CalculateMetricValue(metric)
	if err != nil {
		return nil, err
	}

	history, err := s.GetMetricHistory(tenantID, metricID, 30)
	if err != nil {
		log.Printf("Error getting history: %v", err)
	}

	calculated := &models.CalculatedMetric{
		MetricID:  metricID,
		Name:      metric.Name,
		Value:     currentValue,
		Timestamp: time.Now(),
		Trend:     s.CalculateTrend(history),
		Change:    s.CalculateChange(history),
		History:   history,
	}

	return calculated, nil
}

// CompareMetrics compares multiple metrics over a time period
func (s *MetricBuilderService) CompareMetrics(tenantID string, metricIDs []uint, days int) (*models.MetricComparison, error) {
	comparison := &models.MetricComparison{
		Period:  fmt.Sprintf("Last %d days", days),
		Metrics: make([]models.CalculatedMetric, 0),
	}

	for _, metricID := range metricIDs {
		calculated, err := s.GetCalculatedMetric(tenantID, metricID)
		if err != nil {
			log.Printf("Error calculating metric %d: %v", metricID, err)
			continue
		}
		comparison.Metrics = append(comparison.Metrics, *calculated)
	}

	return comparison, nil
}

// ExportMetricsSnapshot exports current metric values
func (s *MetricBuilderService) ExportMetricsSnapshot(tenantID string) (map[string]interface{}, error) {
	metrics, err := s.ListCustomMetrics(tenantID)
	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]interface{})
	snapshot["exported_at"] = time.Now().UTC()
	snapshot["tenant_id"] = tenantID
	snapshot["metrics"] = make([]map[string]interface{}, 0)

	metricsData := snapshot["metrics"].([]map[string]interface{})
	for _, metric := range metrics {
		value, err := s.CalculateMetricValue(&metric)
		if err != nil {
			continue
		}

		metricData := map[string]interface{}{
			"id":        metric.ID,
			"name":      metric.Name,
			"value":     value,
			"type":      metric.MetricType,
			"timestamp": time.Now().UTC(),
		}
		metricsData = append(metricsData, metricData)
	}

	return snapshot, nil
}
