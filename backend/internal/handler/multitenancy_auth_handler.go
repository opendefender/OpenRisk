package handler

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

// MultitenantAuthHandler handles multi-tenant authentication endpoints
type MultitenantAuthHandler struct {
	authService *service.MultitenantAuthService
}

// NewMultitenantAuthHandler creates a new multi-tenant auth handler
func NewMultitenantAuthHandler(authService *service.MultitenantAuthService) *MultitenantAuthHandler {
	return &MultitenantAuthHandler{
		authService: authService,
	}
}

// Login handles user authentication and returns tokens or organization list
func (h *MultitenantAuthHandler) Login(c *fiber.Ctx) error {
	var req service.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email and password required"})
	}

	response, err := h.authService.Login(c.Context(), &req)
	if err != nil {
		log.Printf("Login failed for %s: %v", req.Email, err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// SelectOrganization handles organization selection for multi-org users
func (h *MultitenantAuthHandler) SelectOrganization(c *fiber.Ctx) error {
	var req struct {
		OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Get user ID from context (set by auth middleware)
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tokens, err := h.authService.SelectOrganization(c.Context(), claims.ID, req.OrganizationID)
	if err != nil {
		log.Printf("Organization selection failed: %v", err)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(tokens)
}

// RefreshToken handles token refresh
func (h *MultitenantAuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	tokens, err := h.authService.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid refresh token"})
	}

	return c.Status(fiber.StatusOK).JSON(tokens)
}

// Logout invalidates a user session
func (h *MultitenantAuthHandler) Logout(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Extract token hash from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing authorization header"})
	}

	// For this implementation, we'd need to extract and hash the token
	// This is a simplified version

	if err := h.authService.Logout(c.Context(), ctx.UserID, ""); err != nil {
		log.Printf("Logout failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to logout"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Logged out successfully"})
}

// GetProfile returns the current user's profile
func (h *MultitenantAuthHandler) GetProfile(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":    ctx.UserID,
		"email": ctx.User.Email,
		"name":  ctx.User.FullName,
	})
}

// GetMyOrganizations returns organizations the current user belongs to
func (h *MultitenantAuthHandler) GetMyOrganizations(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// This would be implemented with the org service
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"organizations": []interface{}{}})
}
