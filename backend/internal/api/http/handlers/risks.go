package handlers

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/rs/zerolog"
)

// RiskHandler encapsulates all HTTP handlers for risks
type RiskHandler struct {
	createUC         *risk.CreateRiskUseCase
	getUC            *risk.GetRiskUseCase
	listUC           *risk.ListRisksUseCase
	updateUC         *risk.UpdateRiskUseCase
	deleteUC         *risk.DeleteRiskUseCase
	acceptUC         *risk.AcceptRiskUseCase
	duplicateUC      *risk.DuplicateRiskUseCase
	scoreBreakdownUC *risk.GetScoreBreakdownUseCase
	historyUC        *risk.GetHistoryUseCase
	bulkActionUC     *risk.BulkActionUseCase
	importUC         *risk.ImportRisksUseCase
	exportUC         *risk.ExportRisksUseCase
	riskRepository   domain.RiskRepository
	logger           zerolog.Logger
}

// NewRiskHandler creates a new risk handler
func NewRiskHandler(
	createUC *risk.CreateRiskUseCase,
	getUC *risk.GetRiskUseCase,
	listUC *risk.ListRisksUseCase,
	updateUC *risk.UpdateRiskUseCase,
	deleteUC *risk.DeleteRiskUseCase,
	acceptUC *risk.AcceptRiskUseCase,
	duplicateUC *risk.DuplicateRiskUseCase,
	scoreBreakdownUC *risk.GetScoreBreakdownUseCase,
	historyUC *risk.GetHistoryUseCase,
	bulkActionUC *risk.BulkActionUseCase,
	importUC *risk.ImportRisksUseCase,
	exportUC *risk.ExportRisksUseCase,
	riskRepository domain.RiskRepository,
	logger zerolog.Logger,
) *RiskHandler {
	return &RiskHandler{
		createUC:         createUC,
		getUC:            getUC,
		listUC:           listUC,
		updateUC:         updateUC,
		deleteUC:         deleteUC,
		acceptUC:         acceptUC,
		duplicateUC:      duplicateUC,
		scoreBreakdownUC: scoreBreakdownUC,
		historyUC:        historyUC,
		bulkActionUC:     bulkActionUC,
		importUC:         importUC,
		exportUC:         exportUC,
		riskRepository:   riskRepository,
		logger:           logger,
	}
}

// =============================================================================
// HTTP Request/Response Models
// =============================================================================

type CreateRiskRequest struct {
	Name        string   `json:"name" validate:"required,max=255"`
	Description string   `json:"description"`
	Probability float64  `json:"probability" validate:"required,min=0,max=1"`
	Impact      float64  `json:"impact" validate:"required,min=0,max=10"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Frameworks  []string `json:"frameworks"`
	AssetID     *string  `json:"asset_id"`
	Source      string   `json:"source" validate:"required"`
}

type UpdateRiskRequest struct {
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Probability   *float64 `json:"probability"`
	Impact        *float64 `json:"impact"`
	Status        *string  `json:"status"`
	Tags          []string `json:"tags"`
	Frameworks    []string `json:"frameworks"`
	AssignedTo    *string  `json:"assigned_to"`
	TreatmentPlan *string  `json:"treatment_plan"`
}

type AcceptRiskRequest struct {
	Justification string `json:"justification" validate:"required"`
}

// RiskResponse represents a risk in API responses
type RiskResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Probability float64   `json:"probability"`
	Impact      float64   `json:"impact"`
	Score       float64   `json:"score"`
	Criticality string    `json:"criticality"`
	Tags        []string  `json:"tags"`
	Frameworks  []string  `json:"frameworks"`
	AssignedTo  *string   `json:"assigned_to,omitempty"`
	ReviewerID  *string   `json:"reviewer_id,omitempty"`
	Source      string    `json:"source"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

func riskToResponse(r *domain.Risk) RiskResponse {
	resp := RiskResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Status:      string(r.Status),
		Probability: r.Probability,
		Impact:      r.Impact,
		Score:       r.Score,
		Criticality: string(r.Criticality),
		Tags:        r.Tags,
		Frameworks:  r.Frameworks,
		Source:      string(r.Source),
		CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   r.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if r.AssignedTo != nil {
		s := r.AssignedTo.String()
		resp.AssignedTo = &s
	}
	if r.ReviewerID != nil {
		s := r.ReviewerID.String()
		resp.ReviewerID = &s
	}
	return resp
}

// =============================================================================
// Endpoints
// =============================================================================

// CreateRisk POST /api/v1/risks
func (h *RiskHandler) CreateRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	userID, err := extractUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
	}

	var req CreateRiskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Convert request to use case input
	input := risk.CreateRiskInput{
		Title:       req.Name,
		Description: req.Description,
		Probability: req.Probability,
		Impact:      req.Impact,
		Status:      domain.RiskStatus(req.Status),
		Tags:        req.Tags,
		Frameworks:  req.Frameworks,
		Source:      req.Source,
		Owner:       userID.String(),
	}

	// Execute use case
	newRisk, err := h.createUC.Execute(c.Context(), tenantID, input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to create risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create risk"})
	}

	return c.Status(fiber.StatusCreated).JSON(riskToResponse(newRisk))
}

// ListRisks GET /api/v1/risks
func (h *RiskHandler) ListRisks(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	sortBy := c.Query("sort", "created_at")
	sortOrder := c.Query("order", "desc")

	query := domain.RiskQuery{
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Page:      page,
		Limit:     limit,
	}

	// Parse status filter
	statusParam := c.Query("status")
	if statusParam != "" {
		query.Status = []string{statusParam}
	}

	result, err := h.listUC.Execute(c.Context(), tenantID, query)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list risks")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list risks"})
	}

	// Convert to response format
	data := make([]RiskResponse, len(result.Data))
	for i, r := range result.Data {
		data[i] = riskToResponse(&r)
	}

	return c.JSON(fiber.Map{
		"data":        data,
		"total":       result.Total,
		"page":        result.Page,
		"limit":       result.Limit,
		"total_pages": result.TotalPages,
	})
}

// GetRisk GET /api/v1/risks/:id
func (h *RiskHandler) GetRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	riskData, err := h.getUC.Execute(c.Context(), tenantID, riskID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get risk"})
	}
	if riskData == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "risk not found"})
	}

	return c.JSON(riskToResponse(riskData))
}

// UpdateRisk PATCH /api/v1/risks/:id
func (h *RiskHandler) UpdateRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	var req UpdateRiskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Convert to use case input
	input := risk.UpdateRiskInput{
		Title:       req.Name,
		Description: req.Description,
		Impact:      req.Impact,
		Probability: req.Probability,
		Tags:        req.Tags,
		Frameworks:  req.Frameworks,
	}

	if req.Status != nil {
		s := domain.RiskStatus(*req.Status)
		input.Status = &s
	}

	updatedRisk, err := h.updateUC.Execute(c.Context(), tenantID, riskID, input)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to update risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update risk"})
	}
	if updatedRisk == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "risk not found"})
	}

	return c.JSON(riskToResponse(updatedRisk))
}

// DeleteRisk DELETE /api/v1/risks/:id
func (h *RiskHandler) DeleteRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	if err := h.deleteUC.Execute(c.Context(), tenantID, riskID); err != nil {
		h.logger.Error().Err(err).Msg("failed to delete risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete risk"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// AcceptRisk POST /api/v1/risks/:id/accept
func (h *RiskHandler) AcceptRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	userID, err := extractUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	var req AcceptRiskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	acceptedRisk, err := h.acceptUC.Execute(c.Context(), tenantID, riskID, risk.AcceptRiskInput{
		Justification: req.Justification,
	}, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to accept risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to accept risk"})
	}

	return c.JSON(riskToResponse(acceptedRisk))
}

// DuplicateRisk POST /api/v1/risks/:id/duplicate
func (h *RiskHandler) DuplicateRisk(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	userID, err := extractUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	duplicatedRisk, err := h.duplicateUC.Execute(c.Context(), tenantID, riskID, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to duplicate risk")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to duplicate risk"})
	}

	return c.Status(fiber.StatusCreated).JSON(riskToResponse(duplicatedRisk))
}

// GetScoreBreakdown GET /api/v1/risks/:id/score-breakdown
func (h *RiskHandler) GetScoreBreakdown(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	breakdown, err := h.scoreBreakdownUC.Execute(c.Context(), tenantID, riskID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get score breakdown")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get score breakdown"})
	}

	return c.JSON(breakdown)
}

// GetHistory GET /api/v1/risks/:id/history
func (h *RiskHandler) GetHistory(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	riskID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid risk ID"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	history, err := h.historyUC.Execute(c.Context(), tenantID, riskID, risk.GetHistoryInput{
		Page:  page,
		Limit: limit,
	})
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get history")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get history"})
	}

	return c.JSON(history)
}

// BulkAction POST /api/v1/risks/bulk
func (h *RiskHandler) BulkAction(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	userID, err := extractUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
	}

	var req risk.BulkActionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	result, err := h.bulkActionUC.Execute(c.Context(), tenantID, req, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to perform bulk action")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to perform bulk action"})
	}

	return c.JSON(result)
}

// ExportRisks GET /api/v1/risks/export
func (h *RiskHandler) ExportRisks(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	format := risk.ExportFormat(c.Query("format", "json"))
	if format != risk.ExportFormatJSON && format != risk.ExportFormatCSV && format != risk.ExportFormatXLSX {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid export format"})
	}

	// Build query from parameters
	query := domain.NewRiskQuery()
	query.Page = 1
	query.Limit = 1000

	data, err := h.exportUC.Execute(c.Context(), tenantID, query, format)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to export risks")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to export risks"})
	}

	// Set response headers
	contentType := "application/json"
	filename := "risks.json"

	if format == risk.ExportFormatCSV {
		contentType = "text/csv"
		filename = "risks.csv"
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	return c.SendStream(data)
}

// ImportRisks POST /api/v1/risks/import
func (h *RiskHandler) ImportRisks(c *fiber.Ctx) error {
	tenantID, err := extractTenantID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	userID, err := extractUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	format := risk.ImportFormat(c.FormValue("format", "json"))

	// Read file content
	openFile, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to read file"})
	}
	defer openFile.Close()

	// Read all file content
	buffer := make([]byte, file.Size)
	if _, err := openFile.Read(buffer); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to read file"})
	}

	// Execute import
	result, err := h.importUC.Execute(c.Context(), tenantID, buffer, format, userID)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to import risks")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to import risks"})
	}

	return c.JSON(result)
}

// =============================================================================
// Helper Functions
// =============================================================================

func extractTenantID(c *fiber.Ctx) (uuid.UUID, error) {
	// Get from middleware (should be set by auth middleware)
	tenantIDStr := c.Locals("tenant_id")
	if tenantIDStr == nil {
		return uuid.Nil, fmt.Errorf("tenant_id not found in context")
	}
	return uuid.Parse(tenantIDStr.(string))
}

func extractUserID(c *fiber.Ctx) (uuid.UUID, error) {
	// Get from middleware (should be set by auth middleware)
	userIDStr := c.Locals("user_id")
	if userIDStr == nil {
		return uuid.Nil, fmt.Errorf("user_id not found in context")
	}
	return uuid.Parse(userIDStr.(string))
}

// RegisterRoutes registers all risk routes
func (h *RiskHandler) RegisterRoutes(app *fiber.App) {
	risks := app.Group("/api/v1/risks")

	risks.Post("", h.CreateRisk)                           // POST /api/v1/risks
	risks.Get("", h.ListRisks)                             // GET /api/v1/risks
	risks.Get("/:id", h.GetRisk)                           // GET /api/v1/risks/:id
	risks.Patch("/:id", h.UpdateRisk)                      // PATCH /api/v1/risks/:id
	risks.Delete("/:id", h.DeleteRisk)                     // DELETE /api/v1/risks/:id
	risks.Post("/bulk", h.BulkAction)                      // POST /api/v1/risks/bulk (before /:id)
	risks.Post("/:id/accept", h.AcceptRisk)                // POST /api/v1/risks/:id/accept
	risks.Post("/:id/duplicate", h.DuplicateRisk)          // POST /api/v1/risks/:id/duplicate
	risks.Get("/:id/score-breakdown", h.GetScoreBreakdown) // GET /api/v1/risks/:id/score-breakdown
	risks.Get("/:id/history", h.GetHistory)                // GET /api/v1/risks/:id/history
	risks.Get("/export", h.ExportRisks)                    // GET /api/v1/risks/export
	risks.Post("/import", h.ImportRisks)                   // POST /api/v1/risks/import
}
