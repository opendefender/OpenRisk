package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ReportHandler manages report endpoints
type ReportHandler struct {
	db *gorm.DB
}

// NewReportHandler creates a new report handler
func NewReportHandler(db *gorm.DB) *ReportHandler {
	return &ReportHandler{db: db}
}

// GetReports retrieves all reports with pagination
func (h *ReportHandler) GetReports(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	reportType := c.Query("type")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	type ReportResponse struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		Format      string `json:"format"`
		CreatedAt   string `json:"created_at"`
		GeneratedBy string `json:"generated_by"`
		Status      string `json:"status"`
		Size        string `json:"size"`
	}

	var reports []ReportResponse
	query := h.db

	if reportType != "" {
		query = query.Where("type = ?", reportType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	result := query.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&reports)

	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	var total int64
	countQuery := h.db
	if reportType != "" {
		countQuery = countQuery.Where("type = ?", reportType)
	}
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	countQuery.Model(&ReportResponse{}).Count(&total)

	return c.JSON(fiber.Map{
		"reports": reports,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// GetReport retrieves a single report by ID
func (h *ReportHandler) GetReport(c *fiber.Ctx) error {
	id := c.Params("id")

	type ReportDetail struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		Format      string `json:"format"`
		CreatedAt   string `json:"created_at"`
		GeneratedBy string `json:"generated_by"`
		Status      string `json:"status"`
		Size        string `json:"size"`
		Content     string `json:"content"`
	}

	var report ReportDetail
	result := h.db.Where("id = ?", id).First(&report)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Report not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": result.Error.Error()})
	}

	return c.JSON(report)
}

// GetReportStats retrieves report statistics
func (h *ReportHandler) GetReportStats(c *fiber.Ctx) error {
	type StatsResponse struct {
		TotalReports    int `json:"total_reports"`
		CompletedCount  int `json:"completed"`
		GeneratingCount int `json:"generating"`
		ScheduledCount  int `json:"scheduled"`
	}

	// TODO: Calculate actual stats from database
	stats := StatsResponse{
		TotalReports:    6,
		CompletedCount:  4,
		GeneratingCount: 1,
		ScheduledCount:  1,
	}

	return c.JSON(stats)
}
