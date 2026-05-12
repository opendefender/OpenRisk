package handlers
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

// SwitchOrgRequest represents the request to switch organization
type SwitchOrgRequest struct {
	OrganizationID string `json:"organization_id" binding:"required,uuid"`
}

// SwitchOrgResponse represents the response after switching organization
type SwitchOrgResponse struct {
	User         *UserResponse         `json:"user"`
	Organization *OrganizationResponse `json:"organization"`
	Member       *MemberResponse       `json:"member"`
	TokenPair    *TokenPairResponse    `json:"token_pair"`
}

// UserResponse represents user data in responses
type UserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// OrganizationResponse represents organization data in responses
type OrganizationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// MemberResponse represents member data in responses
type MemberResponse struct {
	Role               string   `json:"role"`
	Permissions        []string `json:"permissions"`
	JoinedAt           string   `json:"joined_at"`
}

// TokenPairResponse represents token pair in responses
type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// SwitchOrgHandler handles organization switching
type SwitchOrgHandler struct {
	switchOrgUseCase *auth.SwitchOrgUseCase
	auditService     *auth.AuditService
}

// NewSwitchOrgHandler creates a new switch org handler
func NewSwitchOrgHandler(
	switchOrgUseCase *auth.SwitchOrgUseCase,
	auditService *auth.AuditService,
) *SwitchOrgHandler {
	return &SwitchOrgHandler{
		switchOrgUseCase: switchOrgUseCase,
		auditService:     auditService,
	}
}

// Handle POST /auth/switch-org
func (h *SwitchOrgHandler) Handle(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID, err := commonmw.GetUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
	}

	// Parse request
	var req SwitchOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request body",
		})
	}

	// Parse organization ID
	targetOrgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"code":    "INVALID_REQUEST",
			"message": "Invalid organization ID format",
		})
	}

	// Execute use case
	input := auth.SwitchOrgInput{
		UserID:         userID,
		TargetOrgID:    targetOrgID,
		DeviceFingerprint: c.Get("X-Device-Fingerprint"),
	}

	output, err := h.switchOrgUseCase.Execute(c.Context(), input)
	if err != nil {
		// Audit failed attempt
		failureReason := err.Error()
		h.auditService.LogFromFiberContext(c, auth.AuditActionSwitchOrg, false, &failureReason)

		// Return appropriate error
		if validationErr, ok := err.(*domain.ValidationError); ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"code":    "VALIDATION_ERROR",
				"message": validationErr.Message,
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to switch organization",
		})
	}

	// Audit successful switch
	h.auditService.LogFromFiberContext(c, auth.AuditActionSwitchOrg, true, nil)

	// Build response
	response := SwitchOrgResponse{
		User: &UserResponse{
			ID:       output.User.ID.String(),
			Email:    output.User.Email,
			FullName: output.User.FullName,
		},
		Organization: &OrganizationResponse{
			ID:   output.Organization.ID.String(),
			Name: output.Organization.Name,
			Slug: output.Organization.Slug,
		},
		Member: &MemberResponse{
			Role:     output.Member.Role,
			JoinedAt: output.Member.JoinedAt.Format(time.RFC3339),
		},
		TokenPair: &TokenPairResponse{
			AccessToken:  output.TokenPair.AccessToken,
			RefreshToken: output.TokenPair.RefreshToken,
			TokenType:    output.TokenPair.TokenType,
			ExpiresIn:    output.TokenPair.ExpiresIn,
		},
	}

	// Note: Permissions are not included in response for security
	// They are embedded in the JWT token

	return c.Status(fiber.StatusOK).JSON(response)
}</content>
<parameter name="filePath">/media/alex/5fce5774-0bd1-4b0b-93f8-9af9f811a58e/home/alex/Téléchargements/Git projects/OpenRisk/backend/internal/api/http/handlers/switch_org_handler.go