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
func TestTimeSeriesAnalyzer_AddDataPoint(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add data point
	dp := analytics.DataPoint{
		Timestamp: time.Now(),
		Value:     .,
	}

	analyzer.AddDataPoint("test_metric", dp)

	// Verify data was added
	series := analyzer.GetSeries("test_metric")
	assert.NotNil(t, series)
	assert.Len(t, series, )
	assert.Equal(t, ., series[].Value)
}

// TestTimeSeriesAnalyzer_AddMultipleDataPoints tests adding multiple points
func TestTimeSeriesAnalyzer_AddMultipleDataPoints(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add multiple data points
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: time.Now().Add(time.Duration(i)  time.Second),
			Value:     float(i  ),
		}
		analyzer.AddDataPoint("test_metric", dp)
	}

	series := analyzer.GetSeries("test_metric")
	assert.Len(t, series, )
}

// TestTimeSeriesAnalyzer_AnalyzeTrend tests trend analysis
func TestTimeSeriesAnalyzer_AnalyzeTrend(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add increasing trend
	baseTime := time.Now()
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i)  time.Hour),
			Value:     float(i  ),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Analyze trend
	trend := analyzer.AnalyzeTrend("metric", time.Hour)

	assert.NotNil(t, trend)
	assert.Equal(t, analytics.TREND_UP, trend.Direction)
	assert.Greater(t, trend.Magnitude, .)
	assert.Greater(t, trend.Confidence, .)
}

// TestTimeSeriesAnalyzer_AnalyzeTrendDownward tests downward trend
func TestTimeSeriesAnalyzer_AnalyzeTrendDownward(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add decreasing trend
	baseTime := time.Now()
	for i := ; i > ; i-- {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(-i)  time.Hour),
			Value:     float(i  ),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	trend := analyzer.AnalyzeTrend("metric", time.Hour)

	assert.Equal(t, analytics.TREND_DOWN, trend.Direction)
	assert.Greater(t, trend.Confidence, .)
}

// TestTimeSeriesAnalyzer_AggregateData tests data aggregation
func TestTimeSeriesAnalyzer_AggregateData(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add data with hourly distribution
	baseTime := time.Now().Truncate(time.Hour)
	for i := ; i < ; i++ {
		for j := ; j < ; j++ {
			dp := analytics.DataPoint{
				Timestamp: baseTime.Add(time.Duration(i)time.Hour + time.Duration(j)time.Minute),
				Value:     float((i + )  (j + )),
			}
			analyzer.AddDataPoint("metric", dp)
		}
	}

	// Aggregate hourly
	aggregated := analyzer.AggregateData("metric", analytics.HOURLY)

	assert.NotNil(t, aggregated)
	assert.Len(t, aggregated.DataPoints, )

	// Verify aggregation values
	for _, ap := range aggregated.DataPoints {
		assert.Greater(t, ap.Average, .)
		assert.GreaterOrEqual(t, ap.Max, ap.Min)
		assert.GreaterOrEqual(t, ap.Max, ap.Average)
	}
}

// TestTimeSeriesAnalyzer_AggregateDailyData tests daily aggregation
func TestTimeSeriesAnalyzer_AggregateDailyData(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add  days of data
	baseTime := time.Now().Truncate(  time.Hour)
	for day := ; day < ; day++ {
		for hour := ; hour < ; hour++ {
			dp := analytics.DataPoint{
				Timestamp: baseTime.Add(time.Duration(day+hour)  time.Hour),
				Value:     float(hour  ),
			}
			analyzer.AddDataPoint("metric", dp)
		}
	}

	aggregated := analyzer.AggregateData("metric", analytics.DAILY)

	assert.Len(t, aggregated.DataPoints, )
	assert.NotZero(t, aggregated.StdDev)
}

// TestTimeSeriesAnalyzer_ComparePeriods tests period comparison
func TestTimeSeriesAnalyzer_ComparePeriods(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	baseTime := time.Now()

	// Add data for period  (-)
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i)  time.Hour),
			Value:     float( + i),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Add data for period  (-)
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i)  time.Hour),
			Value:     float( + (i-)),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	// Compare periods
	periodStart := baseTime
	periodEnd := baseTime.Add(  time.Hour)
	periodStart := baseTime.Add(  time.Hour)
	periodEnd := baseTime.Add(  time.Hour)

	comparison := analyzer.ComparePeriods("metric", periodStart, periodEnd, periodStart, periodEnd)

	assert.NotNil(t, comparison)
	assert.Greater(t, comparison.PercentChange, .)
}

// TestTimeSeriesAnalyzer_GeneratePerformanceReport tests report generation
func TestTimeSeriesAnalyzer_GeneratePerformanceReport(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add sample data
	baseTime := time.Now()
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i)  time.Minute),
			Value:     float( + i%),
		}
		analyzer.AddDataPoint("latency", dp)
	}

	report := analyzer.GeneratePerformanceReport("latency", time.Hour)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.MetricName)
	assert.Greater(t, report.DataPoints, )
	assert.Greater(t, report.AverageValue, .)
}

// TestTimeSeriesAnalyzer_ExportToJSON tests JSON export
func TestTimeSeriesAnalyzer_ExportToJSON(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	dp := analytics.DataPoint{
		Timestamp: time.Now(),
		Value:     .,
	}
	analyzer.AddDataPoint("metric", dp)

	json := analyzer.ExportToJSON("metric")

	assert.NotEmpty(t, json)
	assert.Contains(t, json, "metric")
}

// TestTimeSeriesAnalyzer_Forecasting tests forecasting functionality
func TestTimeSeriesAnalyzer_Forecasting(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Add linear data
	baseTime := time.Now()
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: baseTime.Add(time.Duration(i)  time.Hour),
			Value:     float(i),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	trend := analyzer.AnalyzeTrend("metric", time.Hour)

	assert.NotNil(t, trend)
	assert.Greater(t, trend.Forecast, .)
}

// TestTimeSeriesAnalyzer_DashboardBuilder tests dashboard building
func TestTimeSeriesAnalyzer_DashboardBuilder(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer()
	builder := analyzer.CreateDashboard()

	assert.NotNil(t, builder)

	// Add widgets
	builder.AddWidget("widget", "cpu_usage", analytics.HOURLY)
	builder.AddWidget("widget", "memory_usage", analytics.DAILY)

	dashboard := builder.Build()

	assert.NotNil(t, dashboard)
	assert.Len(t, dashboard.Widgets, )
}

// TestTimeSeriesAnalyzer_MaxCapacity tests max capacity handling
func TestTimeSeriesAnalyzer_MaxCapacity(t testing.T) {
	analyzer := analytics.NewTimeSeriesAnalyzer() // Small capacity

	// Add more than capacity
	for i := ; i < ; i++ {
		dp := analytics.DataPoint{
			Timestamp: time.Now().Add(time.Duration(i)  time.Second),
			Value:     float(i),
		}
		analyzer.AddDataPoint("metric", dp)
	}

	series := analyzer.GetSeries("metric")
	assert.LessOrEqual(t, len(series), )
}

// ========== COMPLIANCE CHECKER TESTS ==========

// TestAuditLogger_LogEvent tests event logging
func TestAuditLogger_LogEvent(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user",
		Action:       audit.ACTION_CREATE,
		ResourceType: "risk",
		ResourceID:   "risk",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Created new risk",
	}

	err := logger.LogEvent(ctx, log)
	require.NoError(t, err)

	// Verify logging
	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.Len(t, logs, )
	assert.Equal(t, "user", logs[].UserID)
}

// TestAuditLogger_MultipleEvents tests logging multiple events
func TestAuditLogger_MultipleEvents(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	// Log  events
	for i := ; i < ; i++ {
		log := &audit.AuditLog{
			UserID:       "user",
			Action:       audit.ACTION_UPDATE,
			ResourceType: "risk",
			ResourceID:   "risk",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.Len(t, logs, )
}

// TestAuditLogger_FilterByUserID tests filtering by user
func TestAuditLogger_FilterByUserID(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	// Log events from different users
	for i := ; i < ; i++ {
		log := &audit.AuditLog{
			UserID:       "user",
			Action:       audit.ACTION_READ,
			ResourceType: "risk",
			ResourceID:   "risk",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	for i := ; i < ; i++ {
		log := &audit.AuditLog{
			UserID:       "user",
			Action:       audit.ACTION_READ,
			ResourceType: "risk",
			ResourceID:   "risk",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	// Filter by user
	logs := logger.GetAuditLog(ctx, "user", "", "", "")
	assert.Len(t, logs, )
}

// TestAuditLogger_FilterByAction tests filtering by action
func TestAuditLogger_FilterByAction(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	actions := []string{audit.ACTION_CREATE, audit.ACTION_UPDATE, audit.ACTION_DELETE}

	for _, action := range actions {
		for i := ; i < ; i++ {
			log := &audit.AuditLog{
				UserID:       "user",
				Action:       action,
				ResourceType: "risk",
				ResourceID:   "risk",
				Timestamp:    time.Now(),
				Status:       audit.STATUS_SUCCESS,
			}
			logger.LogEvent(ctx, log)
		}
	}

	// Filter by action
	logs := logger.GetAuditLog(ctx, "", audit.ACTION_DELETE, "", "")
	assert.Len(t, logs, )
}

// TestComplianceChecker_GDPRCompliance tests GDPR compliance
func TestComplianceChecker_GDPRCompliance(t testing.T) {
	logger := audit.NewAuditLogger()
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log user data deletion
	log := &audit.AuditLog{
		UserID:       "user",
		Action:       audit.ACTION_DELETE,
		ResourceType: "user_data",
		ResourceID:   "user",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	// Check GDPR compliance
	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Greater(t, report.FrameworkScores["GDPR"], )
}

// TestComplianceChecker_HIPAACompliance tests HIPAA compliance
func TestComplianceChecker_HIPAACompliance(t testing.T) {
	logger := audit.NewAuditLogger()
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log PHI access
	log := &audit.AuditLog{
		UserID:       "doctor",
		Action:       audit.ACTION_READ,
		ResourceType: "phi",
		ResourceID:   "phi",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Accessed patient medical record",
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "HIPAA")
}

// TestComplianceChecker_SOCCompliance tests SOC compliance
func TestComplianceChecker_SOCCompliance(t testing.T) {
	logger := audit.NewAuditLogger()
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log access control action
	log := &audit.AuditLog{
		UserID:       "admin",
		Action:       audit.ACTION_UPDATE,
		ResourceType: "access_control",
		ResourceID:   "policy",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "SOC")
}

// TestComplianceChecker_ISOCompliance tests ISO compliance
func TestComplianceChecker_ISOCompliance(t testing.T) {
	logger := audit.NewAuditLogger()
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log security policy action
	log := &audit.AuditLog{
		UserID:       "security",
		Action:       audit.ACTION_CREATE,
		ResourceType: "security_policy",
		ResourceID:   "policy",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
	}
	logger.LogEvent(ctx, log)

	report := checker.CheckCompliance(ctx)

	assert.NotNil(t, report)
	assert.Contains(t, report.FrameworkScores, "ISO")
}

// TestDataRetentionManager tests data retention policies
func TestDataRetentionManager_ArchivePolicy(t testing.T) {
	manager := audit.NewDataRetentionManager()

	// Create policy
	manager.SetRetentionPolicy("user_data", time.Hour, time.Hour)

	oldTime := time.Now().Add(-    time.Hour)
	shouldArchive := manager.ShouldArchive("user_data", oldTime)

	assert.True(t, shouldArchive)
}

// TestDataRetentionManager_DeletePolicy tests deletion policy
func TestDataRetentionManager_DeletePolicy(t testing.T) {
	manager := audit.NewDataRetentionManager()

	// Create policy
	manager.SetRetentionPolicy("user_data", time.Hour, time.Hour)

	oldTime := time.Now().Add(-    time.Hour)
	shouldDelete := manager.ShouldDelete("user_data", oldTime)

	assert.True(t, shouldDelete)
}

// TestAuditLogger_MaxCapacity tests max capacity enforcement
func TestAuditLogger_MaxCapacity(t testing.T) {
	logger := audit.NewAuditLogger() // Small capacity
	ctx := context.Background()

	// Log more than capacity
	for i := ; i < ; i++ {
		log := &audit.AuditLog{
			UserID:       "user",
			Action:       audit.ACTION_READ,
			ResourceType: "data",
			ResourceID:   "id",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	assert.LessOrEqual(t, len(logs), )
}

// TestAuditLogger_FailedAction tests failed action logging
func TestAuditLogger_FailedAction(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user",
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
	assert.Len(t, logs, )
	assert.Equal(t, audit.STATUS_FAILURE, logs[].Status)
}

// TestComplianceReport_Scoring tests compliance scoring
func TestComplianceReport_Scoring(t testing.T) {
	logger := audit.NewAuditLogger()
	checker := audit.NewComplianceChecker(logger)
	ctx := context.Background()

	// Log multiple compliance-related actions
	for i := ; i < ; i++ {
		log := &audit.AuditLog{
			UserID:       "admin",
			Action:       audit.ACTION_UPDATE,
			ResourceType: "policy",
			ResourceID:   "policy",
			Timestamp:    time.Now(),
			Status:       audit.STATUS_SUCCESS,
		}
		logger.LogEvent(ctx, log)
	}

	report := checker.CheckCompliance(ctx)

	// Verify scores are between -
	for framework, score := range report.FrameworkScores {
		assert.GreaterOrEqual(t, score, , "Score for %s should be >= ", framework)
		assert.LessOrEqual(t, score, , "Score for %s should be <= ", framework)
	}
}

// TestAuditLogger_CryptographicIntegrity tests cryptographic integrity
func TestAuditLogger_CryptographicIntegrity(t testing.T) {
	logger := audit.NewAuditLogger()
	ctx := context.Background()

	log := &audit.AuditLog{
		UserID:       "user",
		Action:       audit.ACTION_CREATE,
		ResourceType: "data",
		ResourceID:   "id",
		Timestamp:    time.Now(),
		Status:       audit.STATUS_SUCCESS,
		Details:      "Original details",
	}

	err := logger.LogEvent(ctx, log)
	require.NoError(t, err)

	logs := logger.GetAuditLog(ctx, "", "", "", "")
	require.Len(t, logs, )

	// Hash should be non-empty
	assert.NotEmpty(t, logs[].ChangeHash)
}
