package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"openrisk/backend/internal/audit"
	"openrisk/backend/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/analytics"
	"gorm.io/gorm"
)

// ComplianceHandler handles compliance API endpoints
type ComplianceHandler struct {
	db *gorm.DB
}

// NewComplianceHandler creates a new compliance handler
func NewComplianceHandler(db *gorm.DB) *ComplianceHandler {
	return &ComplianceHandler{db: db}
}

// GetComplianceReport retrieves the compliance report
// GET /api/compliance/report?range=30d
func (h *ComplianceHandler) GetComplianceReport(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	timeRange := c.Query("range", "30d")

	// Calculate date range
	var startDate time.Time
	switch timeRange {
	case "7d":
		startDate = time.Now().AddDate(0, 0, -7)
	case "30d":
		startDate = time.Now().AddDate(0, 0, -30)
	case "90d":
		startDate = time.Now().AddDate(0, 0, -90)
	case "1y":
		startDate = time.Now().AddDate(-1, 0, 0)
	default:
		startDate = time.Now().AddDate(0, 0, -30)
	}

	// Initialize audit logger
	logger := audit.NewAuditLogger(10000)

	// Fetch audit logs from database
	var auditLogs []struct {
		UserID       string
		Action       string
		ResourceType string
		ResourceID   string
		Status       string
		Timestamp    time.Time
	}

	h.db.
		Table("audit_logs").
		Select("user_id, action, resource_type, resource_id, status, timestamp").
		Where("created_at >= ?", startDate).
		Order("created_at DESC").
		Scan(&auditLogs)

	// Add to logger
	ctx := context.Background()
	for _, log := range auditLogs {
		logger.LogEvent(ctx, &audit.AuditLog{
			UserID:       log.UserID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceID:   log.ResourceID,
			Status:       log.Status,
			Timestamp:    log.Timestamp,
		})
	}

	// Check compliance
	checker := audit.NewComplianceChecker(logger)
	complianceReport := checker.CheckCompliance(ctx)

	// Format frameworks
	frameworks := make([]fiber.Map, 0)
	frameworkNames := []string{"GDPR", "HIPAA", "SOC2", "ISO27001"}

	for _, name := range frameworkNames {
		score := complianceReport.FrameworkScores[name]

		status := "compliant"
		if score < 60 {
			status = "non-compliant"
		} else if score < 80 {
			status = "warning"
		}

		frameworks = append(frameworks, fiber.Map{
			"name":            name,
			"score":           score,
			"status":          status,
			"issues":          h.getFrameworkIssues(name, score),
			"recommendations": h.getFrameworkRecommendations(name, score),
		})
	}

	// Format audit events
	auditEvents := make([]fiber.Map, 0)
	for i, log := range auditLogs {
		if i >= 50 {
			break // Limit to 50 events
		}
		auditEvents = append(auditEvents, fiber.Map{
			"id":        log.UserID + "-" + strconv.Itoa(i),
			"user":      log.UserID,
			"action":    log.Action,
			"resource":  log.ResourceType,
			"timestamp": log.Timestamp.Format(time.RFC3339),
			"status":    log.Status,
		})
	}

	// Generate compliance trend
	trend := h.generateComplianceTrend(startDate)

	return c.JSON(fiber.Map{
		"overallScore": complianceReport.OverallScore,
		"frameworks":   frameworks,
		"auditEvents":  auditEvents,
		"trend":        trend,
	})
}

// GetAuditLogs retrieves audit logs
// GET /api/compliance/audit-logs?user=user123&action=DELETE
func (h *ComplianceHandler) GetAuditLogs(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	filterUser := c.Query("user", "")
	filterAction := c.Query("action", "")
	filterResource := c.Query("resource", "")
	filterStatus := c.Query("status", "")
	limitStr := c.Query("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit > 1000 {
		limit = 100
	}

	// Query database
	var auditLogs []struct {
		ID           int
		UserID       string
		Action       string
		ResourceType string
		ResourceID   string
		Status       string
		Details      string
		Timestamp    time.Time
	}

	query := h.db.Table("audit_logs")

	if filterUser != "" {
		query = query.Where("user_id = ?", filterUser)
	}
	if filterAction != "" {
		query = query.Where("action = ?", filterAction)
	}
	if filterResource != "" {
		query = query.Where("resource_type = ?", filterResource)
	}
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}

	query.
		Order("created_at DESC").
		Limit(limit).
		Scan(&auditLogs)

	// Format response
	logs := make([]fiber.Map, 0)
	for _, log := range auditLogs {
		logs = append(logs, fiber.Map{
			"id":            log.ID,
			"user":          log.UserID,
			"action":        log.Action,
			"resource_type": log.ResourceType,
			"resource_id":   log.ResourceID,
			"status":        log.Status,
			"details":       log.Details,
			"timestamp":     log.Timestamp.Format(time.RFC3339),
		})
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"count": len(logs),
	})
}

// ExportComplianceReport exports compliance report
// GET /api/compliance/export?format=json
func (h *ComplianceHandler) ExportComplianceReport(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	format := c.Query("format", "json")
	timeRange := c.Query("range", "30d")

	// Get compliance report
	logger := audit.NewAuditLogger(10000)
	ctx := context.Background()

	var auditLogs []struct {
		UserID       string
		Action       string
		ResourceType string
		ResourceID   string
		Status       string
		Timestamp    time.Time
	}

	h.db.
		Table("audit_logs").
		Select("user_id, action, resource_type, resource_id, status, timestamp").
		Order("created_at DESC").
		Limit(5000).
		Scan(&auditLogs)

	for _, log := range auditLogs {
		logger.LogEvent(ctx, &audit.AuditLog{
			UserID:       log.UserID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceID:   log.ResourceID,
			Status:       log.Status,
			Timestamp:    log.Timestamp,
		})
	}

	checker := audit.NewComplianceChecker(logger)
	report := checker.CheckCompliance(ctx)

	switch format {
	case "json":
		filename := "compliance-report-" + time.Now().Format("2006-01-02") + ".json"
		c.Set("Content-Disposition", "attachment; filename="+filename)
		c.Set("Content-Type", "application/json")
		return c.JSON(report)

	case "csv":
		filename := "compliance-report-" + time.Now().Format("2006-01-02") + ".csv"
		c.Set("Content-Disposition", "attachment; filename="+filename)
		c.Set("Content-Type", "text/csv")

		csv := "OpenRisk Compliance Report\n"
		csv += "Generated," + time.Now().Format(time.RFC3339) + "\n"
		csv += "Overall Score," + strconv.Itoa(int(report.OverallScore)) + "\n\n"

		csv += "Framework,Score\n"
		for framework, score := range report.FrameworkScores {
			csv += framework + "," + strconv.Itoa(int(score)) + "\n"
		}

		return c.SendString(csv)

	default:
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported format",
		})
	}
}

// getFrameworkIssues returns issues for a framework
func (h *ComplianceHandler) getFrameworkIssues(framework string, score float64) []string {
	issues := make([]string, 0)

	if score < 50 {
		issues = append(issues, "Critical compliance violations detected")
	}

	switch framework {
	case "GDPR":
		if score < 80 {
			issues = append(issues, "Missing user consent documentation")
			issues = append(issues, "Incomplete data deletion logs")
		}

	case "HIPAA":
		if score < 80 {
			issues = append(issues, "PHI access logging incomplete")
			issues = append(issues, "Encryption standards not verified")
		}

	case "SOC2":
		if score < 80 {
			issues = append(issues, "Access control policies need review")
			issues = append(issues, "Security monitoring gaps identified")
		}

	case "ISO27001":
		if score < 80 {
			issues = append(issues, "Information security policies need update")
			issues = append(issues, "Risk assessment overdue")
		}
	}

	return issues
}

// getFrameworkRecommendations returns recommendations for a framework
func (h *ComplianceHandler) getFrameworkRecommendations(framework string, score float64) []string {
	recommendations := make([]string, 0)

	if score >= 80 {
		recommendations = append(recommendations, "Maintain current compliance level")
	}

	switch framework {
	case "GDPR":
		recommendations = append(recommendations, "Implement automated consent tracking")
		recommendations = append(recommendations, "Enable data deletion audit logs")
		recommendations = append(recommendations, "Conduct quarterly compliance reviews")

	case "HIPAA":
		recommendations = append(recommendations, "Implement PHI access logging")
		recommendations = append(recommendations, "Enable end-to-end encryption")
		recommendations = append(recommendations, "Conduct monthly security audits")

	case "SOC2":
		recommendations = append(recommendations, "Review and update access control policies")
		recommendations = append(recommendations, "Implement continuous security monitoring")
		recommendations = append(recommendations, "Conduct quarterly compliance assessments")

	case "ISO27001":
		recommendations = append(recommendations, "Update information security policies")
		recommendations = append(recommendations, "Conduct annual risk assessments")
		recommendations = append(recommendations, "Implement security awareness training")
	}

	return recommendations
}

// generateComplianceTrend generates compliance trend data
func (h *ComplianceHandler) generateComplianceTrend(startDate time.Time) []fiber.Map {
	trend := make([]fiber.Map, 0)

	// Generate daily trend data
	currentDate := startDate
	for currentDate.Before(time.Now()) {
		// Calculate score for this date (simulated)
		daysPasssed := time.Since(currentDate).Hours() / 24
		score := 75.0 + (daysPasssed * 0.2) // Gradually improving

		if score > 95 {
			score = 95
		}

		trend = append(trend, fiber.Map{
			"date":  currentDate.Format("2006-01-02"),
			"score": score,
		})

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return trend
}

// RegisterComplianceRoutes registers all compliance routes
func RegisterComplianceRoutes(app *fiber.App, db *gorm.DB) {
	handler := NewComplianceHandler(db)

	// Create protected group - endpoints accessible without /api/v1 prefix for frontend compatibility
	protected := app.Group("/api/compliance")
	protected.Use(middleware.Protected())

	// Compliance endpoints
	protected.Get("/report", handler.GetComplianceReport)
	protected.Get("/audit-logs", handler.GetAuditLogs)
	protected.Get("/export", handler.ExportComplianceReport)
}

// RegisterTimeSeriesRoutes registers time series analytics routes
func RegisterTimeSeriesRoutes(app *fiber.App, db *gorm.DB) {
	handler := NewTimeSeriesHandler(db)

	// Create protected group - endpoints accessible without /api/v1 prefix for frontend compatibility
	protected := app.Group("/api/analytics")
	protected.Use(middleware.Protected())

	// Time series endpoints
	protected.Get("/timeseries", handler.GetTimeSeriesData)
	protected.Post("/compare", handler.ComparePeriods)
	protected.Get("/report", handler.GenerateReport)
}

// TimeSeriesHandler handles time series analytics API endpoints
type TimeSeriesHandler struct {
	db *gorm.DB
}

// NewTimeSeriesHandler creates a new time series handler
func NewTimeSeriesHandler(db *gorm.DB) *TimeSeriesHandler {
	return &TimeSeriesHandler{db: db}
}

// GetTimeSeriesData retrieves time series data for a metric
func (h *TimeSeriesHandler) GetTimeSeriesData(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metric := c.Query("metric", "latency_ms")
	period := c.Query("period", "daily")
	daysStr := c.Query("days", "7")

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 7
	}

	// Create analyzer
	analyzer := analytics.NewTimeSeriesAnalyzer(10000)

	// Fetch data from database
	var dataPoints []struct {
		Timestamp time.Time
		Value     float64
	}

	query := h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", metric).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -days)).
		Order("timestamp ASC").
		Scan(&dataPoints)

	if query.Error != nil && query.Error != gorm.ErrRecordNotFound {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch data",
		})
	}

	// Add data points to analyzer
	for _, dp := range dataPoints {
		analyzer.AddDataPoint(metric, analytics.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.Value,
		})
	}

	// Get series
	series := analyzer.GetSeries(metric)

	// Convert to API format
	points := make([]fiber.Map, 0)
	for _, dp := range series {
		points = append(points, fiber.Map{
			"timestamp": dp.Timestamp.Format(time.RFC3339),
			"value":     dp.Value,
		})
	}

	// Analyze trend
	trend := analyzer.AnalyzeTrend(metric, 24*time.Hour)
	trendData := fiber.Map{}
	if trend != nil {
		trendData = fiber.Map{
			"direction":  trend.Direction,
			"magnitude":  trend.Magnitude,
			"confidence": trend.Confidence,
			"forecast":   trend.Forecast,
		}
	}

	// Aggregate data
	var aggregationLevel string
	switch period {
	case "hourly":
		aggregationLevel = analytics.HOURLY
	case "weekly":
		aggregationLevel = analytics.WEEKLY
	case "monthly":
		aggregationLevel = analytics.MONTHLY
	default:
		aggregationLevel = analytics.DAILY
	}

	aggregated := analyzer.AggregateData(metric, aggregationLevel)
	aggregatedPoints := make([]fiber.Map, 0)
	if aggregated != nil {
		for _, ap := range aggregated.DataPoints {
			aggregatedPoints = append(aggregatedPoints, fiber.Map{
				"timestamp": ap.Timestamp,
				"average":   ap.Average,
				"min":       ap.Min,
				"max":       ap.Max,
				"stddev":    ap.StdDev,
			})
		}
	}

	// Generate metric cards
	metricCards := h.generateMetricCards(metric, series)

	return c.JSON(fiber.Map{
		"metric":     metric,
		"period":     period,
		"points":     points,
		"trend":      trendData,
		"aggregated": aggregatedPoints,
		"cards":      metricCards,
	})
}

// ComparePeriods compares metrics across two time periods
func (h *TimeSeriesHandler) ComparePeriods(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var req struct {
		Metric  string `json:"metric"`
		Period1 struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"period1"`
		Period2 struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"period2"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	analyzer := analytics.NewTimeSeriesAnalyzer(10000)

	// Parse dates
	p1Start, err := time.Parse(time.RFC3339, req.Period1.Start)
	if err != nil {
		p1Start, _ = time.Parse("2006-01-02", req.Period1.Start)
	}

	p1End, err := time.Parse(time.RFC3339, req.Period1.End)
	if err != nil {
		p1End, _ = time.Parse("2006-01-02", req.Period1.End)
	}

	p2Start, err := time.Parse(time.RFC3339, req.Period2.Start)
	if err != nil {
		p2Start, _ = time.Parse("2006-01-02", req.Period2.Start)
	}

	p2End, err := time.Parse(time.RFC3339, req.Period2.End)
	if err != nil {
		p2End, _ = time.Parse("2006-01-02", req.Period2.End)
	}

	// Fetch data
	var dataPoints []struct {
		Timestamp time.Time
		Value     float64
	}

	h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", req.Metric).
		Where("timestamp >= ? AND timestamp <= ?", p1Start, p2End).
		Order("timestamp ASC").
		Scan(&dataPoints)

	for _, dp := range dataPoints {
		analyzer.AddDataPoint(req.Metric, analytics.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.Value,
		})
	}

	// Compare periods
	comparison := analyzer.ComparePeriods(req.Metric, p1Start, p1End, p2Start, p2End)

	return c.JSON(fiber.Map{
		"metric":          req.Metric,
		"period1_average": comparison.Period1Average,
		"period2_average": comparison.Period2Average,
		"absolute_change": comparison.AbsoluteChange,
		"percent_change":  comparison.PercentChange,
		"period1_min":     comparison.Period1Min,
		"period1_max":     comparison.Period1Max,
		"period2_min":     comparison.Period2Min,
		"period2_max":     comparison.Period2Max,
	})
}

// GenerateReport generates a performance report
func (h *TimeSeriesHandler) GenerateReport(c *fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metric := c.Query("metric", "latency_ms")
	windowStr := c.Query("window", "24")

	window, err := strconv.Atoi(windowStr)
	if err != nil {
		window = 24
	}

	analyzer := analytics.NewTimeSeriesAnalyzer(10000)

	// Fetch data
	var dataPoints []struct {
		Timestamp time.Time
		Value     float64
	}

	h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", metric).
		Where("created_at >= ?", time.Now().Add(-time.Duration(window)*time.Hour)).
		Order("timestamp ASC").
		Scan(&dataPoints)

	for _, dp := range dataPoints {
		analyzer.AddDataPoint(metric, analytics.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.Value,
		})
	}

	// Generate report
	report := analyzer.GeneratePerformanceReport(metric, time.Duration(window)*time.Hour)

	return c.JSON(fiber.Map{
		"metric_name":   report.MetricName,
		"data_points":   report.DataPoints,
		"average_value": report.AverageValue,
		"min_value":     report.MinValue,
		"max_value":     report.MaxValue,
		"std_dev":       report.StdDev,
		"time_window":   strconv.Itoa(window) + " hours",
	})
}

// generateMetricCards generates metric cards for dashboard
func (h *TimeSeriesHandler) generateMetricCards(metric string, series []analytics.DataPoint) []fiber.Map {
	if len(series) == 0 {
		return []fiber.Map{}
	}

	values := make([]float64, len(series))
	for i, dp := range series {
		values[i] = dp.Value
	}

	// Calculate statistics
	sum := 0.0
	min := values[0]
	max := values[0]

	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg := sum / float64(len(values))

	// Calculate percent change
	oldValue := values[0]
	newValue := values[len(values)-1]
	percentChange := ((newValue - oldValue) / oldValue) * 100

	unit := ""
	switch metric {
	case "latency_ms":
		unit = "ms"
	case "throughput_rps":
		unit = "RPS"
	case "error_rate":
		unit = "%"
	case "cpu_usage":
		unit = "%"
	case "memory_usage":
		unit = "%"
	}

	return []fiber.Map{
		{
			"title":      "Current Value",
			"value":      newValue,
			"change":     percentChange,
			"isPositive": percentChange >= 0,
			"unit":       unit,
		},
		{
			"title":      "Average",
			"value":      avg,
			"change":     0,
			"isPositive": false,
			"unit":       unit,
		},
		{
			"title":      "Minimum",
			"value":      min,
			"change":     0,
			"isPositive": false,
			"unit":       unit,
		},
		{
			"title":      "Maximum",
			"value":      max,
			"change":     0,
			"isPositive": false,
			"unit":       unit,
		},
	}
}
