package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/services"
)

// MultitenantOrgHandler handles organization management endpoints
type MultitenantOrgHandler struct {
	orgService *services.MultitenantOrgService
}

// NewMultitenantOrgHandler creates a new organization manager handler
func NewMultitenantOrgHandler(orgService *services.MultitenantOrgService) *MultitenantOrgHandler {
	return &MultitenantOrgHandler{
		orgService: orgService,
	}
}

// CreateOrganization creates a new organization
func (h *MultitenantOrgHandler) CreateOrganization(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req services.CreateOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	org, err := h.orgService.CreateOrganization(c.Context(), &req, ctx.UserID)
	if err != nil {
		log.Printf("Failed to create organization: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(org)
}

// GetOrganization retrieves an organization by ID or slug
func (h *MultitenantOrgHandler) GetOrganization(c *fiber.Ctx) error {
	orgID := c.Params("id")

	// Try parsing as UUID first
	parsedID, err := uuid.Parse(orgID)
	var org interface{}
	var getErr error

	if err == nil {
		// It's a UUID
		org, getErr = h.orgService.GetOrganizationByID(c.Context(), parsedID)
	} else {
		// Treat as slug
		org, getErr = h.orgService.GetOrganizationBySlug(c.Context(), orgID)
	}

	if getErr != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Organization not found"})
	}

	return c.Status(fiber.StatusOK).JSON(org)
}

// ListMyOrganizations returns organizations the current user belongs to
func (h *MultitenantOrgHandler) ListMyOrganizations(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	orgs, err := h.orgService.GetUserOrganizations(c.Context(), ctx.UserID)
	if err != nil {
		log.Printf("Failed to list organizations: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list organizations"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"organizations": orgs})
}

// UpdateOrganization updates an organization
func (h *MultitenantOrgHandler) UpdateOrganization(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Only org root can update
	if !ctx.Member.IsRoot() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only root can update organization"})
	}

	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	org, err := h.orgService.UpdateOrganization(c.Context(), orgID, updates)
	if err != nil {
		log.Printf("Failed to update organization: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update organization"})
	}

	return c.Status(fiber.StatusOK).JSON(org)
}

// DeleteOrganization deletes an organization (soft delete)
func (h *MultitenantOrgHandler) DeleteOrganization(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Only org root can delete
	if !ctx.Member.IsRoot() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only root can delete organization"})
	}

	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	if err := h.orgService.DeleteOrganization(c.Context(), orgID); err != nil {
		log.Printf("Failed to delete organization: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete organization"})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

// InviteMembers invites users to an organization
func (h *MultitenantOrgHandler) InviteMembers(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	// Check permission to manage members
	can, _ := ctx.Permissions.Can("members", "manage")
	if !can && !ctx.Member.IsAdmin() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Permission denied"})
	}

	var req services.InviteMembersRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	result, err := h.orgService.InviteMembers(c.Context(), orgID, &req, ctx.UserID)
	if err != nil {
		log.Printf("Failed to invite members: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to invite members"})
	}

	return c.Status(fiber.StatusOK).JSON(result)
}

// AcceptInvitation accepts an invitation and adds user to organization
func (h *MultitenantOrgHandler) AcceptInvitation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tokenStr := c.Params("token")
	token, err := uuid.Parse(tokenStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid invitation token"})
	}

	org, err := h.orgService.AcceptInvitation(c.Context(), token, ctx.UserID)
	if err != nil {
		log.Printf("Failed to accept invitation: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":      "Invitation accepted",
		"organization": org,
	})
}

// TransferOwnership transfers root ownership to another user
func (h *MultitenantOrgHandler) TransferOwnership(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Only root can transfer ownership
	if !ctx.Member.IsRoot() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only root can transfer ownership"})
	}

	orgID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid organization ID"})
	}

	var req struct {
		NewOwnerID uuid.UUID `json:"new_owner_id" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.orgService.TransferOwnership(c.Context(), orgID, ctx.UserID, req.NewOwnerID); err != nil {
		log.Printf("Failed to transfer ownership: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Ownership transferred successfully"})
}
