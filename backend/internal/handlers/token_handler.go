package handlers

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// TokenHandler handles API token management endpoints
type TokenHandler struct {
	tokenService services.TokenService
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(tokenService services.TokenService) TokenHandler {
	return &TokenHandler{
		tokenService: tokenService,
	}
}

// CreateToken creates a new API token (POST /api/tokens)
func (h TokenHandler) CreateToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var req domain.TokenCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "token name is required"})
	}

	if req.Type == "" {
		req.Type = domain.TokenTypeBearer
	}

	tokenWithValue, err := h.tokenService.CreateToken(userID, &req, userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create token"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"token": fiber.Map{
			"id":           tokenWithValue.ID,
			"name":         tokenWithValue.Name,
			"type":         tokenWithValue.Type,
			"token_prefix": tokenWithValue.TokenPrefix,
			"value":        tokenWithValue.Token,
			"expires_at":   tokenWithValue.ExpiresAt,
			"created_at":   tokenWithValue.CreatedAt,
		},
	})
}

// ListTokens lists all API tokens for the user (GET /api/tokens)
func (h TokenHandler) ListTokens(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokens, err := h.tokenService.ListTokens(userID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list tokens"})
	}

	responses := make([]fiber.Map, len(tokens))
	for i, token := range tokens {
		responses[i] = fiber.Map{
			"id":           token.ID,
			"name":         token.Name,
			"description":  token.Description,
			"type":         token.Type,
			"status":       token.Status,
			"token_prefix": token.TokenPrefix,
			"last_used_at": token.LastUsed,
			"expires_at":   token.ExpiresAt,
			"created_at":   token.CreatedAt,
			"permissions":  token.Permissions,
			"scopes":       token.Scopes,
		}
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"tokens": responses,
		"count":  len(tokens),
	})
}

// GetToken retrieves a single token (GET /api/tokens/:id)
func (h TokenHandler) GetToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}

	token, err := h.tokenService.GetToken(tokenID)
	if err != nil || token == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if token.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "cannot access another user's token"})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"token": fiber.Map{
			"id":           token.ID,
			"name":         token.Name,
			"description":  token.Description,
			"type":         token.Type,
			"status":       token.Status,
			"token_prefix": token.TokenPrefix,
			"last_used_at": token.LastUsed,
			"expires_at":   token.ExpiresAt,
			"created_at":   token.CreatedAt,
			"updated_at":   token.UpdatedAt,
			"permissions":  token.Permissions,
			"scopes":       token.Scopes,
			"ip_whitelist": token.IPWhitelist,
		},
	})
}

// UpdateToken updates a token (PUT /api/tokens/:id)
func (h TokenHandler) UpdateToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}

	token, err := h.tokenService.GetToken(tokenID)
	if err != nil || token == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if token.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "cannot update another user's token"})
	}

	var req domain.TokenUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	updatedToken, err := h.tokenService.UpdateToken(tokenID, &req)
	if err != nil || updatedToken == nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update token"})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"token": fiber.Map{
			"id":           updatedToken.ID,
			"name":         updatedToken.Name,
			"description":  updatedToken.Description,
			"type":         updatedToken.Type,
			"status":       updatedToken.Status,
			"token_prefix": updatedToken.TokenPrefix,
			"expires_at":   updatedToken.ExpiresAt,
			"created_at":   updatedToken.CreatedAt,
			"updated_at":   updatedToken.UpdatedAt,
		},
	})
}

// RevokeToken revokes a token (POST /api/tokens/:id/revoke)
func (h TokenHandler) RevokeToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}

	token, err := h.tokenService.GetToken(tokenID)
	if err != nil || token == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if token.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "cannot revoke another user's token"})
	}

	var req domain.TokenRevokeRequest
	_ = c.BodyParser(&req)
	if req.Reason == "" {
		req.Reason = "revoked by user"
	}

	revokedToken, err := h.tokenService.RevokeToken(tokenID, req.Reason)
	if err != nil || revokedToken == nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke token"})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"token": fiber.Map{
			"id":            revokedToken.ID,
			"status":        revokedToken.Status,
			"revoked_at":    revokedToken.RevokedAt,
			"revoke_reason": revokedToken.RevokeReason,
		},
	})
}

// RotateToken rotates a token (POST /api/tokens/:id/rotate)
func (h TokenHandler) RotateToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}

	token, err := h.tokenService.GetToken(tokenID)
	if err != nil || token == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if token.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "cannot rotate another user's token"})
	}

	rotateResp, err := h.tokenService.RotateToken(tokenID, userID, "rotated by user")
	if err != nil || rotateResp == nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to rotate token"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"old_token":  rotateResp.OldToken,
		"new_token":  rotateResp.NewToken,
		"rotated_at": rotateResp.RotatedAt,
	})
}

// DeleteToken deletes a token (DELETE /api/tokens/:id)
func (h TokenHandler) DeleteToken(c fiber.Ctx) error {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid token id"})
	}

	token, err := h.tokenService.GetToken(tokenID)
	if err != nil || token == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}

	if token.UserID != userID {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{"error": "cannot delete another user's token"})
	}

	err = h.tokenService.DeleteToken(tokenID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete token"})
	}

	return c.SendStatus(http.StatusNoContent)
}

// GetUserIDFromContext extracts user ID from fiber context
func GetUserIDFromContext(c fiber.Ctx) (uuid.UUID, error) {
	userIDVal := c.Locals("userID")
	if userIDVal == nil {
		return uuid.UUID{}, errors.New("user id not found in context")
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.UUID{}, errors.New("invalid user id type in context")
	}

	return userID, nil
}
