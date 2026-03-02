package handlers

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/services"
	"github.com/opendefender/openrisk/internal/validation"
)

// OrganizationHandler handles organization-related HTTP requests
type OrganizationHandler struct {
	orgService *services.OrganizationService
	validator  *validation.RequestValidator
}

func NewOrganizationHandler(orgService *services.OrganizationService, validator *validation.RequestValidator) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
		validator:  validator,
	}
}

// CreateOrganization creates a new organization
func (h *OrganizationHandler) CreateOrganization(c *fiber.Ctx) error {
	var req services.CreateOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	org, err := h.orgService.CreateOrganization(c.Context(), &req)
	if err != nil {
		log.Printf("Error creating organization: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create organization"})
	}

	return c.Status(fiber.StatusCreated).JSON(org)
}

// GetOrganization retrieves an organization by ID
func (h *OrganizationHandler) GetOrganization(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	org, err := h.orgService.GetOrganization(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Organization not found"})
	}

	return c.JSON(org)
}

// UpdateOrganization updates an organization
func (h *OrganizationHandler) UpdateOrganization(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	var req services.UpdateOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Website != "" {
		updates["website"] = req.Website
	}
	if req.Country != "" {
		updates["country"] = req.Country
	}
	if req.Industry != "" {
		updates["industry"] = req.Industry
	}
	if req.CompanySize != "" {
		updates["company_size"] = req.CompanySize
	}
	if req.Timezone != "" {
		updates["timezone"] = req.Timezone
	}

	org, err := h.orgService.UpdateOrganization(c.Context(), orgID, updates)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update organization"})
	}

	return c.JSON(org)
}

// UpgradeSubscription upgrades an organization's subscription
func (h *OrganizationHandler) UpgradeSubscription(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	var req struct {
		Tier string `json:"tier" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	org, err := h.orgService.UpgradeSubscription(c.Context(), orgID, req.Tier)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(org)
}

// AddMember adds a user to an organization
func (h *OrganizationHandler) AddMember(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	var req services.AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	member, err := h.orgService.AddMemberToOrganization(c.Context(), orgID, req.UserID, req.Role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to add member"})
	}

	return c.Status(fiber.StatusCreated).JSON(member)
}

// RemoveMember removes a user from an organization
func (h *OrganizationHandler) RemoveMember(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	if err := h.orgService.RemoveMemberFromOrganization(c.Context(), orgID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove member"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// GetMembers retrieves all members of an organization
func (h *OrganizationHandler) GetMembers(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	members, err := h.orgService.GetOrganizationMembers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve members"})
	}

	return c.JSON(members)
}

// UpdateMemberRole updates a member's role
func (h *OrganizationHandler) UpdateMemberRole(c *fiber.Ctx) error {
	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var req struct {
		Role string `json:"role" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.orgService.UpdateMemberRole(c.Context(), orgID, userID, req.Role); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update member role"})
	}

	return c.SendStatus(fiber.StatusOK)
}

// MigrationHandler handles data migration from self-hosted to SaaS
type MigrationHandler struct {
	migrationService *services.MigrationService
	validator        *validation.RequestValidator
}

func NewMigrationHandler(migrationService *services.MigrationService, validator *validation.RequestValidator) *MigrationHandler {
	return &MigrationHandler{
		migrationService: migrationService,
		validator:        validator,
	}
}

// CreateMigrationJob creates a new migration job
func (h *MigrationHandler) CreateMigrationJob(c *fiber.Ctx) error {
	var req services.CreateMigrationJobRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	job, err := h.migrationService.CreateMigrationJob(c.Context(), &req)
	if err != nil {
		log.Printf("Error creating migration job: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create migration job"})
	}

	return c.Status(fiber.StatusCreated).JSON(job)
}

// GetMigrationJob retrieves a migration job
func (h *MigrationHandler) GetMigrationJob(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	job, err := h.migrationService.GetMigrationJob(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Migration job not found"})
	}

	return c.JSON(job)
}

// StartMigration starts a migration job
func (h *MigrationHandler) StartMigration(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	job, err := h.migrationService.StartMigration(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(job)
}

// UploadMigrationFile uploads a data file for migration
func (h *MigrationHandler) UploadMigrationFile(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to read file"})
	}

	fileType := c.FormValue("fileType")
	if fileType == "" {
		fileType = "json"
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer src.Close()

	if err := h.migrationService.ImportDataFile(c.Context(), jobID, src, fileType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Migration failed: %v", err)})
	}

	return c.JSON(fiber.Map{"message": "File uploaded and processing started"})
}

// GetMigrationStatus retrieves the status of a migration
func (h *MigrationHandler) GetMigrationStatus(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	status, err := h.migrationService.GetMigrationStatus(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Migration job not found"})
	}

	return c.JSON(status)
}

// CompleteMigration marks a migration as completed
func (h *MigrationHandler) CompleteMigration(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	// Validate migration before completion
	status, err := h.migrationService.ValidateMigration(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Validation failed"})
	}

	// Check success rate
	successRate, _ := status["success_rate"].(float64)
	if successRate < 0.95 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":        "Migration validation failed: success rate below 95%",
			"success_rate": successRate,
			"validation":   status,
		})
	}

	job, err := h.migrationService.CompleteMigration(c.Context(), jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to complete migration"})
	}

	return c.JSON(job)
}

// CancelMigration cancels a migration job
func (h *MigrationHandler) CancelMigration(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	job, err := h.migrationService.FailMigration(c.Context(), jobID, "Migration cancelled by user")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to cancel migration"})
	}

	return c.JSON(job)
}

// RegisterOrganizationRoutes registers all organization routes
func RegisterOrganizationRoutes(app *fiber.App, handler *OrganizationHandler) {
	api := app.Group("/api/v1/organizations")

	api.Post("", handler.CreateOrganization)
	api.Get("/:id", handler.GetOrganization)
	api.Put("/:id", handler.UpdateOrganization)
	api.Post("/:id/upgrade", handler.UpgradeSubscription)
	api.Post("/:id/members", handler.AddMember)
	api.Delete("/:id/members/:userId", handler.RemoveMember)
	api.Get("/:id/members", handler.GetMembers)
	api.Patch("/:id/members/:userId/role", handler.UpdateMemberRole)
}

// RegisterMigrationRoutes registers all migration routes
func RegisterMigrationRoutes(app *fiber.App, handler *MigrationHandler) {
	api := app.Group("/api/v1/migrations")

	api.Post("", handler.CreateMigrationJob)
	api.Get("/:jobId", handler.GetMigrationJob)
	api.Post("/:jobId/start", handler.StartMigration)
	api.Post("/:jobId/upload", handler.UploadMigrationFile)
	api.Get("/:jobId/status", handler.GetMigrationStatus)
	api.Post("/:jobId/complete", handler.CompleteMigration)
	api.Post("/:jobId/cancel", handler.CancelMigration)
}
