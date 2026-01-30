package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// MarketplaceHandler handles marketplace HTTP requests
type MarketplaceHandler struct {
	service services.MarketplaceService
}

// NewMarketplaceHandler creates a new MarketplaceHandler
func NewMarketplaceHandler(service services.MarketplaceService) MarketplaceHandler {
	return &MarketplaceHandler{
		service: service,
	}
}

// RegisterRoutes registers marketplace routes
func (h MarketplaceHandler) RegisterRoutes(app fiber.Router) {
	marketplace := app.Group("/marketplace")

	// Connector endpoints
	marketplace.Get("/connectors", h.ListConnectors)
	marketplace.Get("/connectors/:id", h.GetConnector)
	marketplace.Get("/connectors/search", h.SearchConnectors)
	marketplace.Post("/connectors/:id/reviews", h.AddConnectorReview)

	// Installation endpoints (protected)
	marketplace.Post("/apps", h.InstallApp)
	marketplace.Get("/apps", h.ListApps)
	marketplace.Get("/apps/:id", h.GetApp)
	marketplace.Put("/apps/:id", h.UpdateApp)
	marketplace.Post("/apps/:id/enable", h.EnableApp)
	marketplace.Post("/apps/:id/disable", h.DisableApp)
	marketplace.Delete("/apps/:id", h.UninstallApp)
	marketplace.Put("/apps/:id/sync", h.UpdateAppSync)
	marketplace.Post("/apps/:id/sync", h.TriggerSync)
	marketplace.Get("/apps/:id/logs", h.GetAppLogs)
}

// ListConnectors lists all available connectors
// @Summary List marketplace connectors
// @Description Get all available connectors with optional filtering
// @Tags Marketplace
// @Param status query string false "Filter by status (active, inactive, beta, deprecated)"
// @Param category query string false "Filter by category"
// @Param limit query int false "Limit (default: )"
// @Param offset query int false "Offset (default: )"
// @Produce json
// @Success  {object} map[string]interface{}
// @Router /marketplace/connectors [get]
func (h MarketplaceHandler) ListConnectors(c fiber.Ctx) error {
	limit := c.QueryInt("limit", )
	offset := c.QueryInt("offset", )

	var status domain.ConnectorStatus
	if s := c.Query("status"); s != "" {
		st := domain.ConnectorStatus(s)
		status = &st
	}

	category := c.Query("category", "")

	connectors, total, err := h.service.ListConnectors(c.Context(), status, category, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":   connectors,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetConnector retrieves a connector by ID
// @Summary Get connector details
// @Description Retrieve detailed information about a specific connector
// @Tags Marketplace
// @Param id path string true "Connector ID"
// @Produce json
// @Success  {object} domain.Connector
// @Router /marketplace/connectors/{id} [get]
func (h MarketplaceHandler) GetConnector(c fiber.Ctx) error {
	connectorID := c.Params("id")
	if connectorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "connector_id is required",
		})
	}

	connector, err := h.service.GetConnector(c.Context(), connectorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(connector)
}

// SearchConnectors searches connectors by query
// @Summary Search connectors
// @Description Search connectors by name, description, or author
// @Tags Marketplace
// @Param q query string true "Search query"
// @Param limit query int false "Limit (default: )"
// @Param offset query int false "Offset (default: )"
// @Produce json
// @Success  {object} map[string]interface{}
// @Router /marketplace/connectors/search [get]
func (h MarketplaceHandler) SearchConnectors(c fiber.Ctx) error {
	query := c.Query("q", "")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "search query is required",
		})
	}

	limit := c.QueryInt("limit", )
	offset := c.QueryInt("offset", )

	connectors, total, err := h.service.SearchConnectors(c.Context(), query, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":   connectors,
		"total":  total,
		"query":  query,
		"limit":  limit,
		"offset": offset,
	})
}

// AddConnectorReview adds a review to a connector
// @Summary Add connector review
// @Description Add a review and rating to a connector
// @Tags Marketplace
// @Param id path string true "Connector ID"
// @Param body body map[string]interface{} true "Review data"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/connectors/{id}/reviews [post]
func (h MarketplaceHandler) AddConnectorReview(c fiber.Ctx) error {
	connectorID := c.Params("id")
	userID := c.Locals("user_id").(string)

	body := struct {
		Author  string json:"author"
		Rating  int    json:"rating"
		Comment string json:"comment"
	}{}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if body.Author == "" || body.Rating ==  {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "author and rating are required",
		})
	}

	if err := h.service.AddConnectorReview(c.Context(), connectorID, userID, body.Author, body.Rating, body.Comment); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "review added successfully",
	})
}

// InstallApp installs a connector
// @Summary Install connector
// @Description Install a connector as a marketplace app
// @Tags Marketplace
// @Param body body map[string]interface{} true "Installation data"
// @Produce json
// @Success  {object} domain.MarketplaceApp
// @Router /marketplace/apps [post]
func (h MarketplaceHandler) InstallApp(c fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	tenantID := c.Locals("tenant_id").(string)

	body := struct {
		ConnectorID   string                 json:"connector_id"
		AppName       string                 json:"app_name"
		Configuration map[string]interface{} json:"configuration"
	}{}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if body.ConnectorID == "" || body.AppName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "connector_id and app_name are required",
		})
	}

	if body.Configuration == nil {
		body.Configuration = make(map[string]interface{})
	}

	app, err := h.service.InstallApp(c.Context(), body.ConnectorID, tenantID, userID, body.AppName, body.Configuration)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(app)
}

// ListApps lists installed apps
// @Summary List installed apps
// @Description Get all installed marketplace apps for the tenant
// @Tags Marketplace
// @Param limit query int false "Limit (default: )"
// @Param offset query int false "Offset (default: )"
// @Produce json
// @Success  {object} map[string]interface{}
// @Router /marketplace/apps [get]
func (h MarketplaceHandler) ListApps(c fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(string)
	limit := c.QueryInt("limit", )
	offset := c.QueryInt("offset", )

	apps, total, err := h.service.ListApps(c.Context(), tenantID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":   apps,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetApp retrieves an installed app
// @Summary Get app details
// @Description Retrieve details of an installed marketplace app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Produce json
// @Success  {object} domain.MarketplaceApp
// @Router /marketplace/apps/{id} [get]
func (h MarketplaceHandler) GetApp(c fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "app_id is required",
		})
	}

	app, err := h.service.GetApp(c.Context(), appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(app)
}

// UpdateApp updates app configuration
// @Summary Update app configuration
// @Description Update the configuration of an installed app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Param body body map[string]interface{} true "Configuration update"
// @Produce json
// @Success  {object} domain.MarketplaceApp
// @Router /marketplace/apps/{id} [put]
func (h MarketplaceHandler) UpdateApp(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	body := struct {
		Configuration map[string]interface{} json:"configuration"
	}{}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if body.Configuration == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "configuration is required",
		})
	}

	app, err := h.service.UpdateApp(c.Context(), appID, userID, body.Configuration)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(app)
}

// EnableApp enables an app
// @Summary Enable app
// @Description Enable a disabled marketplace app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/apps/{id}/enable [post]
func (h MarketplaceHandler) EnableApp(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.service.EnableApp(c.Context(), appID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "app enabled successfully",
	})
}

// DisableApp disables an app
// @Summary Disable app
// @Description Disable a marketplace app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/apps/{id}/disable [post]
func (h MarketplaceHandler) DisableApp(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.service.DisableApp(c.Context(), appID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "app disabled successfully",
	})
}

// UninstallApp uninstalls an app
// @Summary Uninstall app
// @Description Remove a marketplace app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/apps/{id} [delete]
func (h MarketplaceHandler) UninstallApp(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.service.UninstallApp(c.Context(), appID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "app uninstalled successfully",
	})
}

// UpdateAppSync updates sync configuration
// @Summary Update sync configuration
// @Description Update auto-sync settings for an app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Param body body map[string]interface{} true "Sync configuration"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/apps/{id}/sync [put]
func (h MarketplaceHandler) UpdateAppSync(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	body := struct {
		AutoSync     bool json:"auto_sync"
		SyncInterval int  json:"sync_interval"
	}{}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if body.SyncInterval <  {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "sync_interval must be at least  seconds",
		})
	}

	if err := h.service.UpdateAppSync(c.Context(), appID, userID, body.AutoSync, body.SyncInterval); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "sync configuration updated successfully",
	})
}

// TriggerSync manually triggers a sync
// @Summary Trigger manual sync
// @Description Manually trigger a sync for an app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Produce json
// @Success  {object} map[string]string
// @Router /marketplace/apps/{id}/sync [post]
func (h MarketplaceHandler) TriggerSync(c fiber.Ctx) error {
	appID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if err := h.service.TriggerSync(c.Context(), appID, userID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "sync triggered successfully",
	})
}

// GetAppLogs retrieves app logs
// @Summary Get app logs
// @Description Retrieve activity logs for an app
// @Tags Marketplace
// @Param id path string true "App ID"
// @Param action query string false "Filter by action"
// @Param limit query int false "Limit (default: )"
// @Param offset query int false "Offset (default: )"
// @Produce json
// @Success  {object} map[string]interface{}
// @Router /marketplace/apps/{id}/logs [get]
func (h MarketplaceHandler) GetAppLogs(c fiber.Ctx) error {
	appID := c.Params("id")
	action := c.Query("action", "")
	limit, err := strconv.Atoi(c.Query("limit", ""))
	if err != nil {
		limit = 
	}
	offset, err := strconv.Atoi(c.Query("offset", ""))
	if err != nil {
		offset = 
	}

	logs, total, err := h.service.GetAppLogs(c.Context(), appID, action, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
