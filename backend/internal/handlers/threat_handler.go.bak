package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// ThreatHandler manages threat intelligence endpoints
type ThreatHandler struct {
	db *gorm.DB
}

// NewThreatHandler creates a new threat handler
func NewThreatHandler(db *gorm.DB) *ThreatHandler {
	return &ThreatHandler{db: db}
}

// GetThreats retrieves all threats with optional filtering
func (h *ThreatHandler) GetThreats(c *fiber.Ctx) error {
	severity := c.Query("severity")
	country := c.Query("country")

	type ThreatResponse struct {
		ID        string  `json:"id"`
		Country   string  `json:"country"`
		Code      string  `json:"code"`
		Threats   int     `json:"threats"`
		Severity  string  `json:"severity"`
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lon"`
	}

	var threats []ThreatResponse
	query := h.db

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if country != "" {
		query = query.Where("country ILIKE ?", "%"+country+"%")
	}

	result := query.Find(&threats)

	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	var total int64
	h.db.Model(&ThreatResponse{}).Count(&total)

	return c.JSON(fiber.Map{
		"threats": threats,
		"total":   total,
	})
}

// GetThreatStats retrieves threat statistics
func (h *ThreatHandler) GetThreatStats(c *fiber.Ctx) error {
	type StatsResponse struct {
		TotalThreats  int     `json:"total_threats"`
		CriticalCount int     `json:"critical"`
		HighCount     int     `json:"high"`
		MediumCount   int     `json:"medium"`
		LowCount      int     `json:"low"`
		TrendPercent  float64 `json:"trend_percent"`
	}

	stats := StatsResponse{}

	// Calculate total number of threats
	if err := database.DB.Model(&domain.Threat{}).Count(&stats.TotalThreats).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve threat statistics",
		})
	}

	// Count threats by severity level
	database.DB.Model(&domain.Threat{}).
		Where("severity = ?", "critical").
		Count(&stats.CriticalCount)

	database.DB.Model(&domain.Threat{}).
		Where("severity = ?", "high").
		Count(&stats.HighCount)

	database.DB.Model(&domain.Threat{}).
		Where("severity = ?", "medium").
		Count(&stats.MediumCount)

	database.DB.Model(&domain.Threat{}).
		Where("severity = ?", "low").
		Count(&stats.LowCount)

	// Calculate trend percentage (comparing current month to previous month)
	now := time.Now()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	previousMonthStart := currentMonthStart.AddDate(0, -1, 0)

	var currentMonthCount, previousMonthCount int64
	database.DB.Model(&domain.Threat{}).
		Where("created_at >= ?", currentMonthStart).
		Count(&currentMonthCount)

	database.DB.Model(&domain.Threat{}).
		Where("created_at >= ? AND created_at < ?", previousMonthStart, currentMonthStart).
		Count(&previousMonthCount)

	if previousMonthCount > 0 {
		stats.TrendPercent = float64((currentMonthCount-previousMonthCount)*100) / float64(previousMonthCount)
	}

	return c.JSON(stats)
}
