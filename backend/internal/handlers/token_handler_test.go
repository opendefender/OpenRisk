package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

func setupTokenHandlerTest(t testing.T) (TokenHandler, fiber.App, uuid.UUID) {
	tokenService := services.NewTokenService()
	handler := NewTokenHandler(tokenService)

	app := fiber.New()
	userID := uuid.New()

	// Middleware to inject userID for tests
	app.Use(func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	})

	// Register routes
	app.Post("/tokens", handler.CreateToken)
	app.Get("/tokens", handler.ListTokens)
	app.Get("/tokens/:id", handler.GetToken)
	app.Put("/tokens/:id", handler.UpdateToken)
	app.Post("/tokens/:id/revoke", handler.RevokeToken)
	app.Post("/tokens/:id/rotate", handler.RotateToken)
	app.Delete("/tokens/:id", handler.DeleteToken)

	return handler, app, userID
}

func TestTokenHandler_CreateToken_Success(t testing.T) {
	_, app, _ := setupTokenHandlerTest(t)

	body := domain.TokenCreateRequest{
		Name:        "Test Token",
		Description: "Token for testing",
		Type:        domain.TokenTypeBearer,
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/tokens", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	token := result["token"].(map[string]interface{})
	assert.NotEmpty(t, token["id"])
	assert.Equal(t, "Test Token", token["name"])
	assert.NotEmpty(t, token["value"])
	assert.NotEmpty(t, token["token_prefix"])
}

func TestTokenHandler_CreateToken_NoName(t testing.T) {
	_, app, _ := setupTokenHandlerTest(t)

	body := domain.TokenCreateRequest{
		Description: "Token without name",
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/tokens", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenHandler_ListTokens(t testing.T) {
	handler, _, userID := setupTokenHandlerTest(t)

	// Create some tokens for this user
	for i := ; i < ; i++ {
		handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "Token " + string(rune(+i)),
		}, userID)
	}

	// Create app with this userID
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	})
	app.Get("/tokens", handler.ListTokens)

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, float(), result["count"])
	assert.Len(t, result["tokens"], )
}

func TestTokenHandler_GetToken_Success(t testing.T) {
	handler, app, userID := setupTokenHandlerTest(t)

	// Create a token
	tokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name:        "Get Test Token",
		Description: "Token to get",
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/tokens/"+tokenWithValue.ID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	tokenData := result["token"].(map[string]interface{})
	assert.Equal(t, tokenWithValue.ID.String(), tokenData["id"])
	assert.Equal(t, "Get Test Token", tokenData["name"])
}

func TestTokenHandler_GetToken_NotFound(t testing.T) {
	_, app, _ := setupTokenHandlerTest(t)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/tokens/"+fakeID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTokenHandler_RevokeToken_Success(t testing.T) {
	handler, app, userID := setupTokenHandlerTest(t)

	// Create a token
	tokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Token to Revoke",
	}, userID)
	require.NoError(t, err)

	revokeReq := domain.TokenRevokeRequest{
		Reason: "Testing revoke",
	}

	jsonBody, _ := json.Marshal(revokeReq)
	req := httptest.NewRequest(http.MethodPost, "/tokens/"+tokenWithValue.ID.String()+"/revoke", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	tokenData := result["token"].(map[string]interface{})
	assert.Equal(t, "revoked", tokenData["status"])
	assert.Equal(t, "Testing revoke", tokenData["revoke_reason"])
}

func TestTokenHandler_DeleteToken_Success(t testing.T) {
	handler, app, userID := setupTokenHandlerTest(t)

	// Create a token
	tokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Token to Delete",
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodDelete, "/tokens/"+tokenWithValue.ID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify token is deleted
	deleted, _ := handler.tokenService.GetToken(tokenWithValue.ID)
	assert.Nil(t, deleted)
}

func TestTokenHandler_RotateToken_Success(t testing.T) {
	handler, app, userID := setupTokenHandlerTest(t)

	// Create a token
	oldTokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Token to Rotate",
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/tokens/"+oldTokenWithValue.ID.String()+"/rotate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotNil(t, result["old_token"])
	assert.NotNil(t, result["new_token"])

	oldToken := result["old_token"].(map[string]interface{})
	assert.Equal(t, "revoked", oldToken["status"])
}

func TestTokenHandler_UpdateToken_Success(t testing.T) {
	handler, app, userID := setupTokenHandlerTest(t)

	// Create a token
	tokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Token to Update",
	}, userID)
	require.NoError(t, err)

	newDesc := "Updated description"
	updateReq := domain.TokenUpdateRequest{
		Description: newDesc,
	}

	jsonBody, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/tokens/"+tokenWithValue.ID.String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	token := result["token"].(map[string]interface{})
	assert.Equal(t, newDesc, token["description"])
}

func TestTokenHandler_OwnershipEnforcement(t testing.T) {
	handler, _, userID := setupTokenHandlerTest(t)

	// Create token for userID
	tokenWithValue, err := handler.tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "User Token",
	}, userID)
	require.NoError(t, err)

	// Simulate different user trying to access
	userID := uuid.New()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	})
	handler := NewTokenHandler(handler.tokenService)
	app.Get("/tokens/:id", handler.GetToken)

	req := httptest.NewRequest(http.MethodGet, "/tokens/"+tokenWithValue.ID.String(), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
