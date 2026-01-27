package tests

import (
	"context"
	"testing"
	"time"

	"openrisk/backend/internal/analytics"
	"openrisk/backend/internal/audit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== TIME SERIES ANALYZER TESTS ==========

// TestTimeSeriesAnalyzer_AddDataPoint tests adding data points
func TestTimeSeriesAnalyzer_AddDataPoint(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add data point
	dp := analytics.DataPoint{
		Timestamp: time.Now(),
		Value:     42.5,
	}

	analyzer.AddDataPoint("test_metric", dp)

	// Verify data was added
	series := analyzer.GetSeries("test_metric")
	assert.NotNil(t, series)
	assert.Len(t, series, 1)
	assert.Equal(t, 42.5, series[0].Value)
}

// TestTimeSeriesAnalyzer_AddMultipleDataPoints tests adding multiple points
func TestTimeSeriesAnalyzer_AddMultipleDataPoints(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add multiple data points
	for i := 0; i < 100; i++ {
		dp := analytics.DataPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Value:     float64(i * 10),
		}
		analyzer.AddDataPoint("test_metric", dp)
	}

	series := analyzer.GetSeries("test_metric")
	assert.Len(t, series, 100)
}

// TestTimeSeriesAnalyzer_AnalyzeTrend tests trend analysis
func TestTimeSeriesAnalyzer_AnalyzeTrend(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add increasing trend
	baseTime := time.Now()
	for i := 0; i < 50; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(i * 2),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Analyze trend
	trend := analyzer.AnalyzeTrend("metric", 24*time.Hour)

	assert.NotNil(t, trend)
	assert.Equal(t, analytics.TREND_UP, trend.Direction)
	assert.Greater(t, trend.Magnitude, 0.0)
	assert.Greater(t, trend.Confidence, 0.5)
}

// TestTimeSeriesAnalyzer_AnalyzeTrendDownward tests downward trend
func TestTimeSeriesAnalyzer_AnalyzeTrendDownward(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add decreasing trend
	baseTime := time.Now()
	for i := 50; i > 0; i-- {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(50-i) * time.Hour),
			Value:     float64(i * 2),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	trend := analyzer.AnalyzeTrend("metric", 24*time.Hour)

	assert.Equal(t, analytics.TREND_DOWN, trend.Direction)
	assert.Greater(t, trend.Confidence, 0.5)
}

// TestTimeSeriesAnalyzer_AggregateData tests data aggregation
func TestTimeSeriesAnalyzer_AggregateData(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add data with hourly distribution
	baseTime := time.Now().Truncate(time.Hour)
	for i := 0; i < 24; i++ {
		for j := 0; j < 10; j++ {
			dp := analytics.DataPoint{
				Timestamp: baseTime.Add(time.Duration(i)*time.Hour + time.Duration(j)*time.Minute),
				Value:     float64((i + 1) * (j + 1)),
			}
			analyzer.AddDataPoint("metric", dp)
		}
	}

	// Aggregate hourly
	aggregated := analyzer.AggregateData("metric", analytics.HOURLY)

	assert.NotNil(t, aggregated)
	assert.Len(t, aggregated.DataPoints, 24)

	// Verify aggregation values
	for _, ap := range aggregated.DataPoints {
		assert.Greater(t, ap.Average, 0.0)
		assert.GreaterOrEqual(t, ap.Max, ap.Min)
		assert.GreaterOrEqual(t, ap.Max, ap.Average)
	}
}

// TestTimeSeriesAnalyzer_AggregateDailyData tests daily aggregation
func TestTimeSeriesAnalyzer_AggregateDailyData(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(10000)

	// Add 7 days of data
	baseTime := time.Now().Truncate(24 * time.Hour)
	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			dp := analytics.DataPoint{
				Timestamp: baseTime.Add(time.Duration(day*24+hour) * time.Hour),
				Value:     float64(hour * 100),
			}
			analyzer.AddDataPoint("metric", dp)
		}
	}

	aggregated := analyzer.AggregateData("metric", analytics.DAILY)

	assert.Len(t, aggregated.DataPoints, 7)
	assert.NotZero(t, aggregated.StdDev)
}

// TestTimeSeriesAnalyzer_ComparePeriods tests period comparison
func TestTimeSeriesAnalyzer_ComparePeriods(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(10000)

	baseTime := time.Now()

	// Add data for period 1 (100-200)
	for i := 0; i < 50; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(100 + i*2),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Add data for period 2 (200-400)
	for i := 50; i < 100; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(200 + (i-50)*4),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Compare periods
	period1Start := baseTime
	period1End := baseTime.Add(50 * time.Hour)
	period2Start := baseTime.Add(50 * time.Hour)
	period2End := baseTime.Add(100 * time.Hour)

	comparison := analyzer.ComparePeriods("metric", period1Start, period1End, period2Start, period2End)

	assert.NotNil(t, comparison)
	assert.Greater(t, comparison.PercentChange, 50.0)
}

// TestTimeSeriesAnalyzer_GeneratePerformanceReport tests report generation
func TestTimeSeriesAnalyzer_GeneratePerformanceReport(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add sample data
	baseTime := time.Now()
	for i := 0; i < 100; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Value:     float64(50 + i%30),
		}
		analyzer.AddDataPoint("latency", dp)
	}

	report := analyzer.GeneratePerformanceReport("latency", 1*time.Hour)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.MetricName)
	assert.Greater(t, report.DataPoints, 0)
	assert.Greater(t, report.AverageValue, 0.0)
}

// TestTimeSeriesAnalyzer_ExportToJSON tests JSON export
func TestTimeSeriesAnalyzer_ExportToJSON(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(100)

	dp := analytics.DataPoint{
		Timestamp: time.Now(),
		Value:     42.5,
	}
	analyzer.AddDataPoint("metric", dp)

	json := analyzer.ExportToJSON("metric")

	assert.NotEmpty(t, json)
	assert.Contains(t, json, "metric")
}

// TestTimeSeriesAnalyzer_Forecasting tests forecasting functionality
func TestTimeSeriesAnalyzer_Forecasting(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)

	// Add linear data
	baseTime := time.Now()
	for i := 0; i < 100; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(i),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	trend := analyzer.AnalyzeTrend("metric", 24*time.Hour)

	assert.NotNil(t, trend)
	assert.Greater(t, trend.Forecast, 0.0)
}

// TestTimeSeriesAnalyzer_DashboardBuilder tests dashboard building
func TestTimeSeriesAnalyzer_DashboardBuilder(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(1000)
	builder := analyzer.CreateDashboard()

	assert.NotNil(t, builder)

	// Add widgets
	builder.AddWidget("widget1", "cpu_usage", analytics.HOURLY)
	builder.AddWidget("widget2", "memory_usage", analytics.DAILY)

	dashboard := builder.Build()

	assert.NotNil(t, dashboard)
	assert.Len(t, dashboard.Widgets, 2)
}

// TestTimeSeriesAnalyzer_MaxCapacity tests max capacity handling
func TestTimeSeriesAnalyzer_MaxCapacity(t *testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer(10) // Small capacity

	// Add more than capacity
	for i := 0; i < 20; i++ {
		dp := analytics.DataPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Value:     float64(i),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	series := analyzer.GetSeries("metric")
	assert.LessOrEqual(t, len(series), 10)
}

// ========== COMPLIANCE CHECKER TESTS ==========

// TestAuditLogger_LogEvent tests event logging
func TestAuditLogger_LogEvent(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user123",
		Action:       audit.ACTION_CREATE,
		ResourceType: "risk",
		ResourceID:   "risk456",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Created new risk",
	}

	err := logger.LogEvent(ctx, log)
	require.NoError(t, err)

	// Verify logging
	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.Len(t, logs, 1)
	assert.Equal(t, "user123", logs[0].UserID)
}

// TestAuditLogger_MultipleEvents tests logging multiple events
func TestAuditLogger_MultipleEvents(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	// Log 100 events
	for i := 0; i < 100; i++ {
		log := &audit.AuditLog{
			UserID:       "user123",
			Action:       audit.ACTION_UPDATE,
			ResourceType: "risk",
			ResourceID:   "risk456",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.Len(t, logs, 100)
}

// TestAuditLogger_FilterByUserID tests filtering by user
func TestAuditLogger_FilterByUserID(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	// Log events from different users
	for i := 0; i < 10; i++ {
		log := &audit.AuditLog{
			UserID:       "user1",
			Action:       audit.ACTION_READ,
			ResourceType: "risk",
			ResourceID:   "risk1",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	for i := 0; i < 5; i++ {
		log := &audit.AuditLog{
			UserID:       "user2",
			Action:       audit.ACTION_READ,
			ResourceType: "risk",
			ResourceID:   "risk2",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	// Filter by user
	logs := logger.GetAuditLog(ctx, "user1", "", "", "")
	assert.Len(t, logs, 10)
}

// TestAuditLogger_FilterByAction tests filtering by action
func TestAuditLogger_FilterByAction(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	actions := []string{audit.ACTION_CREATE, audit.ACTION_UPDATE, audit.ACTION_DELETE}

	for _, action := range actions {
		for i := 0; i < 5; i++ {
			log := &audit.AuditLog{
				UserID:       "user1",
				Action:       action,
				ResourceType: "risk",
				ResourceID:   "risk1",
				Timestamp:    time.Now(),
				Status:       audit.STATUS_SUCCESS,
			}
			logger.LogEvent(ctx, log)
		}
	}

	// Filter by action
	logs := logger.GetAuditLog(ctx, "", audit.ACTION_DELETE, "", "")
	assert.Len(t, logs, 5)
}

// TestComplianceChecker_GDPRCompliance tests GDPR compliance
func TestComplianceChecker_GDPRCompliance(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log user data deletion
	log := &audit.AuditLog{
		UserID:       "user123",
		Action:       audit.ACTION_DELETE,
		ResourceType: "user_data",
		ResourceID:   "user123",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	// Check GDPR compliance
	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Greater(t, report.FrameworkScores["GDPR"], 0)
}

// TestComplianceChecker_HIPAACompliance tests HIPAA compliance
func TestComplianceChecker_HIPAACompliance(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log PHI access
	log := &audit.AuditLog{
		UserID:       "doctor1",
		Action:       audit.ACTION_READ,
		ResourceType: "phi",
		ResourceID:   "phi123",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Accessed patient medical record",
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "HIPAA")
}

// TestComplianceChecker_SOC2Compliance tests SOC2 compliance
func TestComplianceChecker_SOC2Compliance(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log access control action
	log := &audit.AuditLog{
		UserID:       "admin1",
		Action:       audit.ACTION_UPDATE,
		ResourceType: "access_control",
		ResourceID:   "policy1",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "SOC2")
}

// TestComplianceChecker_ISO27001Compliance tests ISO27001 compliance
func TestComplianceChecker_ISO27001Compliance(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log security policy action
	log := &audit.AuditLog{
		UserID:       "security1",
		Action:       audit.ACTION_CREATE,
		ResourceType: "security_policy",
		ResourceID:   "policy1",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "ISO27001")
}

// TestDataRetentionManager tests data retention policies
func TestDataRetentionManager_ArchivePolicy(t *testing.T) {
	manager := audit.NewDataRetentionManager()

	// Create policy
	manager.SetRetentionPolicy("user_data", 30*24*time.Hour, 60*24*time.Hour)

	oldTime := time.Now().Add(-45 * 24 * time.Hour)
	shouldArchive := manager.ShouldArchive("user_data", oldTime)

	assert.True(t, shouldArchive)
}

// TestDataRetentionManager_DeletePolicy tests deletion policy
func TestDataRetentionManager_DeletePolicy(t *testing.T) {
	manager := audit.NewDataRetentionManager()

	// Create policy
	manager.SetRetentionPolicy("user_data", 30*24*time.Hour, 90*24*time.Hour)

	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	shouldDelete := manager.ShouldDelete("user_data", oldTime)

	assert.True(t, shouldDelete)
}

// TestAuditLogger_MaxCapacity tests max capacity enforcement
func TestAuditLogger_MaxCapacity(t *testing.T) {
	logger := audit.NewAuditLogger(50) // Small capacity
	ctx := context.Background()

	// Log more than capacity
	for i := 0; i < 100; i++ {
		log := &audit.AuditLog{
			UserID:       "user1",
			Action:       audit.ACTION_READ,
			ResourceType: "data",
			ResourceID:   "id",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.LessOrEqual(t, len(logs), 50)
}

// TestAuditLogger_FailedAction tests failed action logging
func TestAuditLogger_FailedAction(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user1",
		Action:       audit.ACTION_DELETE,
		ResourceType: "data",
		ResourceID:   "id",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_FAILURE,
		Details:      "Permission denied",
	}

	err := logger.LogEvent(ctx, log)
	require.NoError(t, err)

	logs := logger.GetAuditLog(ctx, "", "", "", audit.STATUS_FAILURE)
	assert.Len(t, logs, 1)
	assert.Equal(t, audit.STATUS_FAILURE, logs[0].Status)
}

// TestComplianceReport_Scoring tests compliance scoring
func TestComplianceReport_Scoring(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log multiple compliance-related actions
	for i := 0; i < 10; i++ {
		log := &audit.AuditLog{
			UserID:       "admin",
			Action:       audit.ACTION_UPDATE,
			ResourceType: "policy",
			ResourceID:   "policy1",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	report := checker.CheckCompliance(ctx)

	// Verify scores are between 0-100
	for framework, score := range report.FrameworkScores {
		assert.GreaterOrEqual(t, score, 0, "Score for %s should be >= 0", framework)
		assert.LessOrEqual(t, score, 100, "Score for %s should be <= 100", framework)
	}
}

// TestAuditLogger_CryptographicIntegrity tests cryptographic integrity
func TestAuditLogger_CryptographicIntegrity(t *testing.T) {
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user1",
		Action:       audit.ACTION_CREATE,
		ResourceType: "data",
		ResourceID:   "id1",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Original details",
	}

	err := logger.LogEvent(ctx, log)
	require.NoError(t, err)

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	require.Len(t, logs, 1)

	// Hash should be non-empty
	assert.NotEmpty(t, logs[0].ChangeHash)
}
