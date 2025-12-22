package handlers

import (
	"github.com/gofiber/fiber/v2"
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

	// TODO: Calculate actual stats from database
	stats := StatsResponse{
		TotalThreats:  153,
		CriticalCount: 12,
		HighCount:     28,
		MediumCount:   45,
		LowCount:      68,
		TrendPercent:  12.5,
	}

	return c.JSON(stats)
}
