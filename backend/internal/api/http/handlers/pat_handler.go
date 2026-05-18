package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	commonmw "github.com/opendefender/openrisk/internal/middleware"
)

// CreatePATRequest represents the request to create a PAT
type CreatePATRequest struct {
	Name        string    `json:"name" binding:"required,min=1,max=255"`
	Description string    `json:"description,omitempty" binding:"max=1000"`
	Scopes      []string  `json:"scopes,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreatePATResponse represents the response after creating a PAT
type CreatePATResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Token       string   `json:"token"` // Shown only once
	Prefix      string   `json:"token_prefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Message     string   `json:"message"` // Warning about token visibility
}

// ListPATResponse represents a PAT in list responses
type ListPATResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Prefix      string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// RevokePATRequest represents the request to revoke a PAT
type RevokePATRequest struct {
	TokenID string `json:"token_id" binding:"required,uuid"`
}

// PersonalAccessTokenHandler handles PAT operations
type PersonalAccessTokenHandler struct {
	patUseCase   *auth.PersonalAccessTokenUseCase
	auditService *auth.AuditService
}

// NewPersonalAccessTokenHandler creates a new PAT handler
func NewPersonalAccessTokenHandler(
	patUseCase *auth.PersonalAccessTokenUseCase,
	auditService *auth.AuditService,
) *PersonalAccessTokenHandler {
	return &PersonalAccessTokenHandler{
		patUseCase:   patUseCase,
		auditService: auditService,
	}
}

// HandleCreate POST /api/v1/users/tokens
func (h *PersonalAccessTokenHandler) HandleCreate(c *fiber.Ctx) error {
	// Get user ID from context
	userID, err := commonmw.GetUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req CreatePATRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request body",
		})
	}

	// Execute use case
	input := auth.CreatePATInput{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Scopes:      req.Scopes,
		ExpiresAt:   req.ExpiresAt,
	}

	output, err := h.patUseCase.CreateToken(c.Context(), input)
	if err != nil {
		failureReason := err.Error()
		h.auditService.LogFromFiberContext(c, auth.AuditActionPatCreate, false, &failureReason)

		if validationErr, ok := err.(*domain.ValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"code":    "VALIDATION_ERROR",
				"message": validationErr.Message,
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to create token",
		})
	}

	// Build response
	response := CreatePATResponse{
		ID:          output.TokenID.String(),
		Name:        req.Name,
		Description: req.Description,
		Token:       output.Token,
		Prefix:      output.Prefix,
		Scopes:      req.Scopes,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   time.Now(),
		Message:     "This token will only be shown once. Store it securely.",
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// HandleList GET /api/v1/users/tokens
func (h *PersonalAccessTokenHandler) HandleList(c *fiber.Ctx) error {
	// Get user ID from context
	userID, err := commonmw.GetUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
	}

	// Execute use case
	tokens, err := h.patUseCase.ListUserTokens(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to list tokens",
		})
	}

	// Build response
	var response []ListPATResponse
	for _, token := range tokens {
		response = append(response, ListPATResponse{
			ID:          token.ID.String(),
			Name:        token.Name,
			Description: token.Description,
			Prefix:      token.Prefix,
			Scopes:      token.Scopes,
			ExpiresAt:   token.ExpiresAt,
			LastUsedAt:  token.LastUsedAt,
			CreatedAt:   token.CreatedAt,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"tokens": response,
	})
}

// HandleRevoke DELETE /api/v1/users/tokens/:id
func (h *PersonalAccessTokenHandler) HandleRevoke(c *fiber.Ctx) error {
	// Get user ID from context
	userID, err := commonmw.GetUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
	}

	// Get token ID from URL
	tokenIDStr := c.Params("id")
	tokenID, err := uuid.Parse(tokenIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"code":    "INVALID_REQUEST",
			"message": "Invalid token ID format",
		})
	}

	// Execute use case
	if err := h.patUseCase.RevokeToken(c.Context(), tokenID, userID); err != nil {
		failureReason := err.Error()
		h.auditService.LogFromFiberContext(c, auth.AuditActionPatRevoke, false, &failureReason)

		if validationErr, ok := err.(*domain.ValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"code":    "VALIDATION_ERROR",
				"message": validationErr.Message,
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to revoke token",
		})
	}

	// Audit successful revocation
	h.auditService.LogFromFiberContext(c, auth.AuditActionPatRevoke, true, nil)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Token revoked successfully",
	})
}</content>
<parameter name="filePath">/media/alex/5fce5774-0bd1-4b0b-93f8-9af9f811a58e/home/alex/Téléchargements/Git projects/OpenRisk/backend/internal/api/http/handlers/pat_handler.go