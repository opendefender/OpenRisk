package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/opendefender/openrisk/internal/models"
)

// ExportService handles exporting analytics and compliance data
type ExportService struct {
	dashboardService *DashboardDataService
}

// NewExportService creates a new export service
func NewExportService(dashboardService *DashboardDataService) *ExportService {
	return &ExportService{
		dashboardService: dashboardService,
	}
}

// ExportMetricsCSV exports metrics data to CSV format
func (s *ExportService) ExportMetricsCSV(tenantID string, timeRange string) ([]byte, error) {
	metrics, err := s.dashboardService.GetMetrics(tenantID, timeRange)
	if err != nil {
		log.Printf("Error getting metrics for export: %v", err)
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Convert metrics to CSV records
	type MetricRecord struct {
		Timestamp      string  `csv:"timestamp"`
		TotalRisks     int     `csv:"total_risks"`
		CriticalRisks  int     `csv:"critical_risks"`
		HighRisks      int     `csv:"high_risks"`
		MediumRisks    int     `csv:"medium_risks"`
		LowRisks       int     `csv:"low_risks"`
		AverageScore   float64 `csv:"average_score"`
		MitigationRate float64 `csv:"mitigation_rate"`
	}

	records := make([]MetricRecord, 0)
	for ts, metric := range metrics {
		record := MetricRecord{
			Timestamp:      ts,
			TotalRisks:     metric["total_risks"].(int),
			CriticalRisks:  metric["critical_risks"].(int),
			HighRisks:      metric["high_risks"].(int),
			MediumRisks:    metric["medium_risks"].(int),
			LowRisks:       metric["low_risks"].(int),
			AverageScore:   metric["average_score"].(float64),
			MitigationRate: metric["mitigation_rate"].(float64),
		}
		records = append(records, record)
	}

	// Encode to CSV
	buf, err := csvutil.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics to CSV: %w", err)
	}

	return buf, nil
}

// ExportMetricsJSON exports metrics data to JSON format
func (s *ExportService) ExportMetricsJSON(tenantID string, timeRange string) ([]byte, error) {
	metrics, err := s.dashboardService.GetMetrics(tenantID, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Create JSON structure
	data := map[string]interface{}{
		"exported_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"time_range":  timeRange,
		"metrics":     metrics,
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics to JSON: %w", err)
	}

	return buf, nil
}

// ExportComplianceCSV exports compliance data to CSV format
func (s *ExportService) ExportComplianceCSV(tenantID string) ([]byte, error) {
	report, err := s.dashboardService.GetComplianceReport(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get compliance report: %w", err)
	}

	// Convert to CSV records
	type ComplianceRecord struct {
		Framework      string  `csv:"framework"`
		Score          float64 `csv:"score"`
		Status         string  `csv:"status"`
		ControlsPassed int     `csv:"controls_passed"`
		ControlsFailed int     `csv:"controls_failed"`
		LastAssessed   string  `csv:"last_assessed"`
	}

	records := make([]ComplianceRecord, 0)
	if reportData, ok := report["frameworks"].(map[string]interface{}); ok {
		for framework, data := range reportData {
			if fData, ok := data.(map[string]interface{}); ok {
				record := ComplianceRecord{
					Framework:      framework,
					Score:          fData["score"].(float64),
					Status:         fData["status"].(string),
					ControlsPassed: int(fData["controls_passed"].(float64)),
					ControlsFailed: int(fData["controls_failed"].(float64)),
					LastAssessed:   fData["last_assessed"].(string),
				}
				records = append(records, record)
			}
		}
	}

	buf, err := csvutil.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compliance to CSV: %w", err)
	}

	return buf, nil
}

// ExportComplianceJSON exports compliance data to JSON format
func (s *ExportService) ExportComplianceJSON(tenantID string) ([]byte, error) {
	report, err := s.dashboardService.GetComplianceReport(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get compliance report: %w", err)
	}

	data := map[string]interface{}{
		"exported_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"compliance":  report,
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compliance to JSON: %w", err)
	}

	return buf, nil
}

// ExportTrendsJSON exports trend analysis data
func (s *ExportService) ExportTrendsJSON(tenantID string, days int) ([]byte, error) {
	trends, err := s.dashboardService.GetTrends(tenantID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get trends: %w", err)
	}

	data := map[string]interface{}{
		"exported_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"period_days": days,
		"trends":      trends,
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trends to JSON: %w", err)
	}

	return buf, nil
}

// ExportFullDashboardReport exports complete dashboard in JSON format
func (s *ExportService) ExportFullDashboardReport(tenantID string) ([]byte, error) {
	// Gather all dashboard data
	metrics, err := s.dashboardService.GetMetrics(tenantID, "30d")
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	compliance, err := s.dashboardService.GetComplianceReport(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get compliance: %w", err)
	}

	trends, err := s.dashboardService.GetTrends(tenantID, 30)
	if err != nil {
		return nil, fmt.Errorf("failed to get trends: %w", err)
	}

	// Comprehensive report
	report := map[string]interface{}{
		"version":     "1.0",
		"exported_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"report": map[string]interface{}{
			"metrics":    metrics,
			"compliance": compliance,
			"trends":     trends,
		},
	}

	buf, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal full report: %w", err)
	}

	return buf, nil
}

// GenerateCSVTable generates a simple CSV table from data
func GenerateCSVTable(headers []string, rows [][]string) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	writer := csv.NewWriter(buf)

	// Write headers
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write rows
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}

	return buf.Bytes(), nil
}

// ExportAuditLogsJSON exports audit logs in JSON format with pagination
func (s *ExportService) ExportAuditLogsJSON(tenantID string, limit int, offset int) ([]byte, error) {
	// This would typically call an audit service
	data := map[string]interface{}{
		"exported_at": time.Now().UTC(),
		"tenant_id":   tenantID,
		"limit":       limit,
		"offset":      offset,
		"audit_logs":  []models.AuditLog{},
	}

	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal audit logs: %w", err)
	}

	return buf, nil
}
