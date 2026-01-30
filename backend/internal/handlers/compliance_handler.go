package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"openrisk/backend/internal/audit"
	"openrisk/backend/internal/middleware"

	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/internal/analytics"
	"gorm.io/gorm"
)

// ComplianceHandler handles compliance API endpoints
type ComplianceHandler struct {
	db gorm.DB
}

// NewComplianceHandler creates a new compliance handler
func NewComplianceHandler(db gorm.DB) ComplianceHandler {
	return &ComplianceHandler{db: db}
}

// GetComplianceReport retrieves the compliance report
// GET /api/compliance/report?range=d
func (h ComplianceHandler) GetComplianceReport(c fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	timeRange := c.Query("range", "d")

	// Calculate date range
	var startDate time.Time
	switch timeRange {
	case "d":
		startDate = time.Now().AddDate(, , -)
	case "d":
		startDate = time.Now().AddDate(, , -)
	case "d":
		startDate = time.Now().AddDate(, , -)
	case "y":
		startDate = time.Now().AddDate(-, , )
	default:
		startDate = time.Now().AddDate(, , -)
	}

	// Initialize audit logger
	logger := audit.NewAuditLogger()

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
	frameworks := make([]fiber.Map, )
	frameworkNames := []string{"GDPR", "HIPAA", "SOC", "ISO"}

	for _, name := range frameworkNames {
		score := complianceReport.FrameworkScores[name]

		status := "compliant"
		if score <  {
			status = "non-compliant"
		} else if score <  {
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
	auditEvents := make([]fiber.Map, )
	for i, log := range auditLogs {
		if i >=  {
			break // Limit to  events
		}
		auditEvents = append(auditEvents, fiber.Map{
			"id":        log.UserID + "-" + strconv.Itoa(i),
			"user":      log.UserID,
			"action":    log.Action,
			"resource":  log.ResourceType,
			"timestamp": log.Timestamp.Format(time.RFC),
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
// GET /api/compliance/audit-logs?user=user&action=DELETE
func (h ComplianceHandler) GetAuditLogs(c fiber.Ctx) error {
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
	limitStr := c.Query("limit", "")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit >  {
		limit = 
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
	logs := make([]fiber.Map, )
	for _, log := range auditLogs {
		logs = append(logs, fiber.Map{
			"id":            log.ID,
			"user":          log.UserID,
			"action":        log.Action,
			"resource_type": log.ResourceType,
			"resource_id":   log.ResourceID,
			"status":        log.Status,
			"details":       log.Details,
			"timestamp":     log.Timestamp.Format(time.RFC),
		})
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"count": len(logs),
	})
}

// ExportComplianceReport exports compliance report
// GET /api/compliance/export?format=json
func (h ComplianceHandler) ExportComplianceReport(c fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	format := c.Query("format", "json")
	timeRange := c.Query("range", "d")

	// Get compliance report
	logger := audit.NewAuditLogger()
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
		Limit().
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
		filename := "compliance-report-" + time.Now().Format("--") + ".json"
		c.Set("Content-Disposition", "attachment; filename="+filename)
		c.Set("Content-Type", "application/json")
		return c.JSON(report)

	case "csv":
		filename := "compliance-report-" + time.Now().Format("--") + ".csv"
		c.Set("Content-Disposition", "attachment; filename="+filename)
		c.Set("Content-Type", "text/csv")

		csv := "OpenRisk Compliance Report\n"
		csv += "Generated," + time.Now().Format(time.RFC) + "\n"
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
func (h ComplianceHandler) getFrameworkIssues(framework string, score float) []string {
	issues := make([]string, )

	if score <  {
		issues = append(issues, "Critical compliance violations detected")
	}

	switch framework {
	case "GDPR":
		if score <  {
			issues = append(issues, "Missing user consent documentation")
			issues = append(issues, "Incomplete data deletion logs")
		}

	case "HIPAA":
		if score <  {
			issues = append(issues, "PHI access logging incomplete")
			issues = append(issues, "Encryption standards not verified")
		}

	case "SOC":
		if score <  {
			issues = append(issues, "Access control policies need review")
			issues = append(issues, "Security monitoring gaps identified")
		}

	case "ISO":
		if score <  {
			issues = append(issues, "Information security policies need update")
			issues = append(issues, "Risk assessment overdue")
		}
	}

	return issues
}

// getFrameworkRecommendations returns recommendations for a framework
func (h ComplianceHandler) getFrameworkRecommendations(framework string, score float) []string {
	recommendations := make([]string, )

	if score >=  {
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

	case "SOC":
		recommendations = append(recommendations, "Review and update access control policies")
		recommendations = append(recommendations, "Implement continuous security monitoring")
		recommendations = append(recommendations, "Conduct quarterly compliance assessments")

	case "ISO":
		recommendations = append(recommendations, "Update information security policies")
		recommendations = append(recommendations, "Conduct annual risk assessments")
		recommendations = append(recommendations, "Implement security awareness training")
	}

	return recommendations
}

// generateComplianceTrend generates compliance trend data
func (h ComplianceHandler) generateComplianceTrend(startDate time.Time) []fiber.Map {
	trend := make([]fiber.Map, )

	// Generate daily trend data
	currentDate := startDate
	for currentDate.Before(time.Now()) {
		// Calculate score for this date (simulated)
		daysPasssed := time.Since(currentDate).Hours() / 
		score := . + (daysPasssed  .) // Gradually improving

		if score >  {
			score = 
		}

		trend = append(trend, fiber.Map{
			"date":  currentDate.Format("--"),
			"score": score,
		})

		currentDate = currentDate.AddDate(, , )
	}

	return trend
}

// RegisterComplianceRoutes registers all compliance routes
func RegisterComplianceRoutes(app fiber.App, db gorm.DB) {
	handler := NewComplianceHandler(db)

	// Create protected group - endpoints accessible without /api/v prefix for frontend compatibility
	protected := app.Group("/api/compliance")
	protected.Use(middleware.Protected())

	// Compliance endpoints
	protected.Get("/report", handler.GetComplianceReport)
	protected.Get("/audit-logs", handler.GetAuditLogs)
	protected.Get("/export", handler.ExportComplianceReport)
}

// RegisterTimeSeriesRoutes registers time series analytics routes
func RegisterTimeSeriesRoutes(app fiber.App, db gorm.DB) {
	handler := NewTimeSeriesHandler(db)

	// Create protected group - endpoints accessible without /api/v prefix for frontend compatibility
	protected := app.Group("/api/analytics")
	protected.Use(middleware.Protected())

	// Time series endpoints
	protected.Get("/timeseries", handler.GetTimeSeriesData)
	protected.Post("/compare", handler.ComparePeriods)
	protected.Get("/report", handler.GenerateReport)
}

// TimeSeriesHandler handles time series analytics API endpoints
type TimeSeriesHandler struct {
	db gorm.DB
}

// NewTimeSeriesHandler creates a new time series handler
func NewTimeSeriesHandler(db gorm.DB) TimeSeriesHandler {
	return &TimeSeriesHandler{db: db}
}

// GetTimeSeriesData retrieves time series data for a metric
func (h TimeSeriesHandler) GetTimeSeriesData(c fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metric := c.Query("metric", "latency_ms")
	period := c.Query("period", "daily")
	daysStr := c.Query("days", "")

	days, err := strconv.Atoi(daysStr)
	if err != nil {
		days = 
	}

	// Create analyzer
	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Fetch data from database
	var dataPoints []struct {
		Timestamp time.Time
		Value     float
	}

	query := h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", metric).
		Where("created_at >= ?", time.Now().AddDate(, , -days)).
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
	points := make([]fiber.Map, )
	for _, dp := range series {
		points = append(points, fiber.Map{
			"timestamp": dp.Timestamp.Format(time.RFC),
			"value":     dp.Value,
		})
	}

	// Analyze trend
	trend := analyzer.AnalyzeTrend(metric, time.Hour)
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
	aggregatedPoints := make([]fiber.Map, )
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
func (h TimeSeriesHandler) ComparePeriods(c fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	var req struct {
		Metric  string json:"metric"
		Period struct {
			Start string json:"start"
			End   string json:"end"
		} json:"period"
		Period struct {
			Start string json:"start"
			End   string json:"end"
		} json:"period"
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Parse dates
	pStart, err := time.Parse(time.RFC, req.Period.Start)
	if err != nil {
		pStart, _ = time.Parse("--", req.Period.Start)
	}

	pEnd, err := time.Parse(time.RFC, req.Period.End)
	if err != nil {
		pEnd, _ = time.Parse("--", req.Period.End)
	}

	pStart, err := time.Parse(time.RFC, req.Period.Start)
	if err != nil {
		pStart, _ = time.Parse("--", req.Period.Start)
	}

	pEnd, err := time.Parse(time.RFC, req.Period.End)
	if err != nil {
		pEnd, _ = time.Parse("--", req.Period.End)
	}

	// Fetch data
	var dataPoints []struct {
		Timestamp time.Time
		Value     float
	}

	h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", req.Metric).
		Where("timestamp >= ? AND timestamp <= ?", pStart, pEnd).
		Order("timestamp ASC").
		Scan(&dataPoints)

	for _, dp := range dataPoints {
		analyzer.AddDataPoint(req.Metric, analytics.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.Value,
		})
	}

	// Compare periods
	comparison := analyzer.ComparePeriods(req.Metric, pStart, pEnd, pStart, pEnd)

	return c.JSON(fiber.Map{
		"metric":          req.Metric,
		"period_average": comparison.PeriodAverage,
		"period_average": comparison.PeriodAverage,
		"absolute_change": comparison.AbsoluteChange,
		"percent_change":  comparison.PercentChange,
		"period_min":     comparison.PeriodMin,
		"period_max":     comparison.PeriodMax,
		"period_min":     comparison.PeriodMin,
		"period_max":     comparison.PeriodMax,
	})
}

// GenerateReport generates a performance report
func (h TimeSeriesHandler) GenerateReport(c fiber.Ctx) error {
	// Check authentication
	userID := c.Locals("userID")
	if userID == nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	metric := c.Query("metric", "latency_ms")
	windowStr := c.Query("window", "")

	window, err := strconv.Atoi(windowStr)
	if err != nil {
		window = 
	}

	analyzer := analytics.NewTimeSeriesAnalyzer()

	// Fetch data
	var dataPoints []struct {
		Timestamp time.Time
		Value     float
	}

	h.db.
		Table("analytics_timeseries").
		Select("timestamp, value").
		Where("metric_name = ?", metric).
		Where("created_at >= ?", time.Now().Add(-time.Duration(window)time.Hour)).
		Order("timestamp ASC").
		Scan(&dataPoints)

	for _, dp := range dataPoints {
		analyzer.AddDataPoint(metric, analytics.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.Value,
		})
	}

	// Generate report
	report := analyzer.GeneratePerformanceReport(metric, time.Duration(window)time.Hour)

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
func (h TimeSeriesHandler) generateMetricCards(metric string, series []analytics.DataPoint) []fiber.Map {
	if len(series) ==  {
		return []fiber.Map{}
	}

	values := make([]float, len(series))
	for i, dp := range series {
		values[i] = dp.Value
	}

	// Calculate statistics
	sum := .
	min := values[]
	max := values[]

	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg := sum / float(len(values))

	// Calculate percent change
	oldValue := values[]
	newValue := values[len(values)-]
	percentChange := ((newValue - oldValue) / oldValue)  

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
			"isPositive": percentChange >= ,
			"unit":       unit,
		},
		{
			"title":      "Average",
			"value":      avg,
			"change":     ,
			"isPositive": false,
			"unit":       unit,
		},
		{
			"title":      "Minimum",
			"value":      min,
			"change":     ,
			"isPositive": false,
			"unit":       unit,
		},
		{
			"title":      "Maximum",
			"value":      max,
			"change":     ,
			"isPositive": false,
			"unit":       unit,
		},
	}
}
