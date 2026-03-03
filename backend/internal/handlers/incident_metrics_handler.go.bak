package handlers

import (
	"time"

	"github.com/opendefender/openrisk/internal/models"

	"github.com/gofiber/fiber/v2"
)

// IncidentMetricsResponse represents the metrics response
type IncidentMetricsResponse struct {
	Metrics struct {
		TotalIncidents      int     `json:"totalIncidents"`
		OpenIncidents       int     `json:"openIncidents"`
		InProgressIncidents int     `json:"inProgressIncidents"`
		ResolvedIncidents   int     `json:"resolvedIncidents"`
		AvgResolutionTime   float64 `json:"avgResolutionTime"`
		SLAComplianceRate   float64 `json:"slaComplianceRate"`
		CriticalCount       int     `json:"criticalCount"`
		HighCount           int     `json:"highCount"`
		MediumCount         int     `json:"mediumCount"`
		LowCount            int     `json:"lowCount"`
	} `json:"metrics"`
}

// IncidentTrendResponse represents trend data
type IncidentTrendResponse struct {
	Trends []struct {
		Date       string `json:"date"`
		Created    int    `json:"created"`
		Resolved   int    `json:"resolved"`
		InProgress int    `json:"inProgress"`
		Open       int    `json:"open"`
	} `json:"trends"`
}

// GetIncidentMetrics returns aggregated incident metrics
func GetIncidentMetrics(c *fiber.Ctx) error {
	timeRange := c.Query("timeRange", "30d")

	// Parse time range
	var days int
	switch timeRange {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "1y":
		days = 365
	default:
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// Get tenant ID from context
	tenantID := c.Locals("tenantID").(string)

	// Fetch incidents for tenant within time range
	db := c.Locals("db")
	var incidents []models.Incident
	db.Where("tenant_id = ? AND created_at >= ?", tenantID, startDate).Find(&incidents)

	// Calculate metrics
	metrics := IncidentMetricsResponse{}
	metrics.Metrics.TotalIncidents = len(incidents)

	var totalResolutionTime float64
	resolvedCount := 0

	for _, incident := range incidents {
		switch incident.Status {
		case "Open":
			metrics.Metrics.OpenIncidents++
		case "InProgress":
			metrics.Metrics.InProgressIncidents++
		case "Resolved":
			metrics.Metrics.ResolvedIncidents++
			if !incident.ResolvedAt.IsZero() {
				resolutionTime := incident.ResolvedAt.Sub(incident.CreatedAt).Hours()
				totalResolutionTime += resolutionTime
				resolvedCount++
			}
		}

		switch incident.Severity {
		case "Critical":
			metrics.Metrics.CriticalCount++
		case "High":
			metrics.Metrics.HighCount++
		case "Medium":
			metrics.Metrics.MediumCount++
		case "Low":
			metrics.Metrics.LowCount++
		}
	}

	// Calculate averages
	if resolvedCount > 0 {
		metrics.Metrics.AvgResolutionTime = totalResolutionTime / float64(resolvedCount)
	}

	// Calculate SLA compliance (assume 24-hour SLA for critical, 48 for high, etc.)
	slaCompliant := 0
	for _, incident := range incidents {
		if incident.Status == "Resolved" && !incident.ResolvedAt.IsZero() {
			var slaHours float64
			switch incident.Severity {
			case "Critical":
				slaHours = 4
			case "High":
				slaHours = 8
			case "Medium":
				slaHours = 24
			case "Low":
				slaHours = 48
			}

			resolutionTime := incident.ResolvedAt.Sub(incident.CreatedAt).Hours()
			if resolutionTime <= slaHours {
				slaCompliant++
			}
		}
	}

	if metrics.Metrics.ResolvedIncidents > 0 {
		metrics.Metrics.SLAComplianceRate = (float64(slaCompliant) / float64(metrics.Metrics.ResolvedIncidents)) * 100
	}

	return c.JSON(metrics)
}

// GetIncidentTrends returns trend data for incident metrics
func GetIncidentTrends(c *fiber.Ctx) error {
	timeRange := c.Query("timeRange", "30d")

	var days int
	switch timeRange {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "1y":
		days = 365
	default:
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	tenantID := c.Locals("tenantID").(string)

	db := c.Locals("db")
	var incidents []models.Incident
	db.Where("tenant_id = ? AND created_at >= ?", tenantID, startDate).
		Order("created_at ASC").
		Find(&incidents)

	// Build trend data by day
	trends := make(map[string]map[string]int)

	for _, incident := range incidents {
		dateStr := incident.CreatedAt.Format("2006-01-02")
		if trends[dateStr] == nil {
			trends[dateStr] = make(map[string]int)
		}
		trends[dateStr]["created"]++

		if incident.Status == "Resolved" && !incident.ResolvedAt.IsZero() {
			resolvedDateStr := incident.ResolvedAt.Format("2006-01-02")
			if trends[resolvedDateStr] == nil {
				trends[resolvedDateStr] = make(map[string]int)
			}
			trends[resolvedDateStr]["resolved"]++
		}
	}

	// Calculate running totals
	response := IncidentTrendResponse{}
	var openCount int
	var inProgressCount int

	for d := 0; d <= days; d++ {
		date := startDate.AddDate(0, 0, d)
		dateStr := date.Format("2006-01-02")

		created := trends[dateStr]["created"]
		resolved := trends[dateStr]["resolved"]

		openCount += created - resolved
		if created > 0 && d > 0 {
			inProgressCount += created / 2 // Rough estimate
		}

		trend := map[string]interface{}{
			"date":       dateStr,
			"created":    created,
			"resolved":   resolved,
			"inProgress": inProgressCount,
			"open":       openCount,
		}

		// Convert to the response format
		trendData := struct {
			Date       string `json:"date"`
			Created    int    `json:"created"`
			Resolved   int    `json:"resolved"`
			InProgress int    `json:"inProgress"`
			Open       int    `json:"open"`
		}{
			Date:       dateStr,
			Created:    created,
			Resolved:   resolved,
			InProgress: inProgressCount,
			Open:       openCount,
		}
		response.Trends = append(response.Trends, trendData)
	}

	return c.JSON(response)
}
