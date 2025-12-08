package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// RiskTimelineHandler handles risk timeline and history endpoints
type RiskTimelineHandler struct {
	service *services.RiskTimelineService
}

// NewRiskTimelineHandler creates a new risk timeline handler
func NewRiskTimelineHandler() *RiskTimelineHandler {
	return &RiskTimelineHandler{
		service: services.NewRiskTimelineService(),
	}
}

// GetRiskTimeline handles GET /risks/:id/timeline
// Retrieves the full history/timeline for a risk
func (h *RiskTimelineHandler) GetRiskTimeline(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	// Get pagination parameters
	limit := 50
	if l := c.QueryInt("limit"); l > 0 && l <= 500 {
		limit = l
	}

	offset := 0
	if o := c.QueryInt("offset"); o >= 0 {
		offset = o
	}

	// Get timeline with pagination
	timeline, total, err := h.service.GetRiskTimelineWithPagination(riskID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"timeline": timeline,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"count":    len(timeline),
	})
}

// GetStatusChanges handles GET /risks/:id/timeline/status-changes
// Retrieves only status change events
func (h *RiskTimelineHandler) GetStatusChanges(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	changes, err := h.service.GetStatusChanges(riskID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"changes": changes,
		"count":   len(changes),
	})
}

// GetScoreChanges handles GET /risks/:id/timeline/score-changes
// Retrieves only score change events
func (h *RiskTimelineHandler) GetScoreChanges(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	changes, err := h.service.GetScoreChanges(riskID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"changes": changes,
		"count":   len(changes),
	})
}

// GetRiskTrend handles GET /risks/:id/timeline/trend
// Analyzes the risk score trend over time
func (h *RiskTimelineHandler) GetRiskTrend(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	trend, err := h.service.ComputeRiskTrend(riskID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(trend)
}

// GetChangesByType handles GET /risks/:id/timeline/changes/:type
// Retrieves history entries of a specific change type
func (h *RiskTimelineHandler) GetChangesByType(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	changeType := c.Params("type")
	if changeType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing change type parameter",
		})
	}

	changes, err := h.service.GetRiskChangesByType(riskID, changeType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"change_type": changeType,
		"changes":     changes,
		"count":       len(changes),
	})
}

// GetRecentActivity handles GET /timeline/recent
// Gets the most recent changes across all risks
func (h *RiskTimelineHandler) GetRecentActivity(c *fiber.Ctx) error {
	limit := 100
	if l := c.QueryInt("limit"); l > 0 && l <= 1000 {
		limit = l
	}

	activity, err := h.service.GetRecentChanges(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"activity": activity,
		"count":    len(activity),
		"limit":    limit,
	})
}

// GetChangesSince handles GET /risks/:id/timeline/since/:unix_timestamp
// Gets all changes since a specific time
func (h *RiskTimelineHandler) GetChangesSince(c *fiber.Ctx) error {
	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid risk ID",
		})
	}

	since := int64(c.QueryInt("since"))
	if since == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing or invalid 'since' parameter (unix timestamp)",
		})
	}

	changes, err := h.service.GetChangesSince(riskID, since)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"since":   since,
		"changes": changes,
		"count":   len(changes),
	})
}

// TimelineEvent represents a single event in the timeline
type TimelineEvent struct {
	ID          uuid.UUID         `json:"id"`
	RiskID      uuid.UUID         `json:"risk_id"`
	Score       float64           `json:"score"`
	Impact      int               `json:"impact"`
	Probability int               `json:"probability"`
	Status      domain.RiskStatus `json:"status"`
	ChangeType  string            `json:"change_type"`
	ChangedBy   string            `json:"changed_by"`
	CreatedAt   int64             `json:"timestamp"`
}

// ConvertToTimelineEvent converts RiskHistory to TimelineEvent
func ConvertToTimelineEvent(h *domain.RiskHistory) *TimelineEvent {
	return &TimelineEvent{
		ID:          h.ID,
		RiskID:      h.RiskID,
		Score:       h.Score,
		Impact:      h.Impact,
		Probability: h.Probability,
		Status:      h.Status,
		ChangeType:  h.ChangeType,
		ChangedBy:   h.ChangedBy,
		CreatedAt:   h.CreatedAt.Unix(),
	}
}
